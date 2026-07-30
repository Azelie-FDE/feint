package network

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"sync"
)

// Errors an allocator returns. They are sentinels because a provider pack has to
// map each one onto its own error body, and matching on a message would break
// the first time the wording changes.
var (
	// ErrExhausted reports that the block has no free address left.
	ErrExhausted = errors.New("network: address block exhausted")
	// ErrOutOfRange reports an address that does not belong to the block, or
	// one that falls in the reserved range.
	ErrOutOfRange = errors.New("network: address outside the usable range")
	// ErrInUse reports an address already handed out.
	ErrInUse = errors.New("network: address already allocated")
	// ErrNotIPv4 reports an IPv6 block. IPv6 allocation is not implemented:
	// a /64 holds more addresses than the counters here can express, and no
	// emulated resource needs one yet.
	ErrNotIPv4 = errors.New("network: only IPv4 blocks are supported")
)

// Allocator hands out addresses from one IPv4 block, and is safe for concurrent
// use.
//
// The bottom of the block is reserved, because every cloud keeps a few addresses
// for itself: AWS takes four (network, router, DNS, one spare), Incus takes the
// network address and the gateway. The broadcast address is always excluded on
// top of that, and the network address is never handed out whatever the caller
// asks, since allocating it would produce an address no machine can carry.
//
// What is left is what Available reports, computed from the mask rather than
// assumed. With four reserved, a /24 offers 251 addresses and a /20 offers 4091,
// which is the AWS arithmetic; an emulator that hardcodes the count answers 251
// for both.
type Allocator struct {
	mu     sync.Mutex
	prefix netip.Prefix
	first  uint32 // first usable address, network address + reserved
	last   uint32 // last usable address, broadcast - 1
	cursor uint32 // where the next scan starts
	used   map[uint32]bool
}

// NewAllocator returns an allocator over p, keeping the first reserved
// addresses of the block for the runtime, the network address included. A
// reserved count below one is raised to one for that reason.
//
// A block too small to hold anything is not an error: it allocates nothing and
// reports a capacity of zero, which is the honest answer for a /31 or a /32.
func NewAllocator(p netip.Prefix, reserved int) (*Allocator, error) {
	if !p.IsValid() {
		return nil, fmt.Errorf("network: invalid prefix")
	}
	if !p.Addr().Is4() {
		return nil, fmt.Errorf("%w: %s", ErrNotIPv4, p)
	}
	if reserved < 0 {
		return nil, fmt.Errorf("network: reserved count %d is negative", reserved)
	}

	p = p.Masked()
	base := uint64(toUint32(p.Addr()))
	size := uint64(1) << uint(32-p.Bits())
	low := uint64(max(reserved, 1)) // the network address is never allocatable

	a := &Allocator{prefix: p, used: make(map[uint32]bool)}
	// A block with no room left is represented by first > last, which every
	// method below reads as "capacity zero" without a special case.
	if size < low+2 {
		a.first, a.last = 1, 0
		return a, nil
	}
	// Both conversions are bounded by the check above: low + 2 <= size, and
	// base + size <= 1<<32 because base is the masked network address.
	a.first = uint32(base + low)     //nolint:gosec // bounded above
	a.last = uint32(base + size - 2) //nolint:gosec // bounded above, minus the broadcast address
	a.cursor = a.first
	return a, nil
}

// Prefix returns the block, masked.
func (a *Allocator) Prefix() netip.Prefix { return a.prefix }

// Capacity reports how many addresses the block can ever hand out.
func (a *Allocator) Capacity() int {
	if a.first > a.last {
		return 0
	}
	return int(a.last-a.first) + 1
}

// Available reports how many addresses are still free. This is the value a
// subnet has to publish, and computing it is the whole point: emulators that
// hardcode it answer 251 for a /20 that holds 4091.
func (a *Allocator) Available() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Capacity() - len(a.used)
}

// Allocate hands out the next free address. It scans forward from the last
// allocation and wraps once, so a released address is reused rather than lost.
func (a *Allocator) Allocate() (netip.Addr, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.first > a.last {
		return netip.Addr{}, fmt.Errorf("%w: %s holds no usable address", ErrExhausted, a.prefix)
	}
	span := a.last - a.first + 1
	for i := uint32(0); i < span; i++ {
		candidate := a.first + (a.cursor-a.first+i)%span
		if a.used[candidate] {
			continue
		}
		a.used[candidate] = true
		a.cursor = a.first + (candidate-a.first+1)%span
		return toAddr(candidate), nil
	}
	return netip.Addr{}, fmt.Errorf("%w: %s", ErrExhausted, a.prefix)
}

// Reserve claims one specific address, for a client that asked for it and for
// rebuilding the allocator from persisted state after a restart.
func (a *Allocator) Reserve(addr netip.Addr) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	v, err := a.usable(addr)
	if err != nil {
		return err
	}
	if a.used[v] {
		return fmt.Errorf("%w: %s", ErrInUse, addr)
	}
	a.used[v] = true
	return nil
}

// Release gives an address back. Releasing one that was never allocated is not
// an error: delete paths run twice more often than anyone expects, and the
// second run must not fail.
func (a *Allocator) Release(addr netip.Addr) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if v, err := a.usable(addr); err == nil {
		delete(a.used, v)
	}
}

// Allocated reports whether the address is currently handed out.
func (a *Allocator) Allocated(addr netip.Addr) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	v, err := a.usable(addr)
	return err == nil && a.used[v]
}

// usable converts an address and checks it sits in the usable range. The caller
// holds the lock.
func (a *Allocator) usable(addr netip.Addr) (uint32, error) {
	if !addr.Is4() {
		return 0, fmt.Errorf("%w: %s", ErrNotIPv4, addr)
	}
	v := toUint32(addr)
	if a.first > a.last || v < a.first || v > a.last {
		return 0, fmt.Errorf("%w: %s is not in %s", ErrOutOfRange, addr, a.prefix)
	}
	return v, nil
}

func toUint32(addr netip.Addr) uint32 {
	b := addr.As4()
	return binary.BigEndian.Uint32(b[:])
}

func toAddr(v uint32) netip.Addr {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return netip.AddrFrom4(b)
}
