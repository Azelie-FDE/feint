package network

import (
	"errors"
	"net/netip"
	"sync"
	"testing"
)

func mustAllocator(t *testing.T, cidr string, reserved int) *Allocator {
	t.Helper()
	a, err := NewAllocator(netip.MustParsePrefix(cidr), reserved)
	if err != nil {
		t.Fatalf("NewAllocator(%s, %d) = %v", cidr, reserved, err)
	}
	return a
}

// The count emulators get wrong: a fixed number whatever the mask. With the four
// addresses AWS reserves at the bottom and the broadcast excluded, a /24 holds
// 251 and a /20 holds 4091. Answering 251 for both is the bug this guards.
func TestCapacityFollowsTheMask(t *testing.T) {
	tests := []struct {
		cidr     string
		reserved int
		want     int
	}{
		{"10.0.0.0/24", 4, 251}, // the AWS arithmetic
		{"10.0.0.0/20", 4, 4091},
		{"10.0.0.0/28", 4, 11},
		{"10.0.0.0/16", 4, 65531},
		{"10.0.0.0/24", 5, 250},
		{"10.0.0.0/24", 2, 253}, // network address plus the Incus gateway
		{"10.0.0.0/30", 5, 0},   // too small to hold anything
		{"10.0.0.0/31", 0, 0},   // the network address is never allocatable
		{"10.0.0.0/32", 0, 0},
	}
	for _, tt := range tests {
		a := mustAllocator(t, tt.cidr, tt.reserved)
		if got := a.Capacity(); got != tt.want {
			t.Errorf("Capacity(%s, reserved=%d) = %d, want %d", tt.cidr, tt.reserved, got, tt.want)
		}
		if got := a.Available(); got != tt.want {
			t.Errorf("Available(%s) = %d, want %d on a fresh allocator", tt.cidr, got, tt.want)
		}
	}
}

func TestAllocateSkipsTheReservedRange(t *testing.T) {
	a := mustAllocator(t, "10.0.0.0/24", 5)

	first, err := a.Allocate()
	if err != nil {
		t.Fatalf("Allocate() = %v", err)
	}
	if first.String() != "10.0.0.5" {
		t.Errorf("first allocation = %s, want 10.0.0.5", first)
	}

	second, err := a.Allocate()
	if err != nil {
		t.Fatalf("Allocate() = %v", err)
	}
	if second.String() != "10.0.0.6" {
		t.Errorf("second allocation = %s, want 10.0.0.6", second)
	}
	if a.Available() != 248 {
		t.Errorf("Available() = %d after two allocations, want 248", a.Available())
	}
}

// A /20 is the mask that catches an allocator counting in the last octet only.
func TestAllocateCrossesOctetBoundaries(t *testing.T) {
	a := mustAllocator(t, "10.0.0.0/20", 5)

	var last netip.Addr
	for i := 0; i < 300; i++ {
		addr, err := a.Allocate()
		if err != nil {
			t.Fatalf("Allocate() failed after %d addresses: %v", i, err)
		}
		last = addr
	}
	if last.String() != "10.0.1.48" {
		t.Errorf("300th allocation = %s, want 10.0.1.48", last)
	}
	if !a.Prefix().Contains(last) {
		t.Errorf("%s escaped the block %s", last, a.Prefix())
	}
}

func TestAllocateExhaustsAndReuses(t *testing.T) {
	a := mustAllocator(t, "10.0.0.0/28", 5) // ten usable addresses

	var got []netip.Addr
	for i := 0; i < 10; i++ {
		addr, err := a.Allocate()
		if err != nil {
			t.Fatalf("Allocate() = %v at %d", err, i)
		}
		got = append(got, addr)
	}
	if _, err := a.Allocate(); !errors.Is(err, ErrExhausted) {
		t.Fatalf("Allocate() on a full block = %v, want ErrExhausted", err)
	}

	// A released address must come back, otherwise a create/destroy loop
	// silently drains the subnet.
	a.Release(got[3])
	if a.Available() != 1 {
		t.Fatalf("Available() = %d after one release, want 1", a.Available())
	}
	reused, err := a.Allocate()
	if err != nil {
		t.Fatalf("Allocate() after a release = %v", err)
	}
	if reused != got[3] {
		t.Errorf("reallocated %s, want the released %s", reused, got[3])
	}
}

func TestReserveClaimsOneAddress(t *testing.T) {
	a := mustAllocator(t, "10.0.0.0/24", 5)
	addr := netip.MustParseAddr("10.0.0.42")

	if err := a.Reserve(addr); err != nil {
		t.Fatalf("Reserve(%s) = %v", addr, err)
	}
	if !a.Allocated(addr) {
		t.Errorf("Allocated(%s) = false right after Reserve", addr)
	}
	if err := a.Reserve(addr); !errors.Is(err, ErrInUse) {
		t.Errorf("Reserve(%s) twice = %v, want ErrInUse", addr, err)
	}

	// Allocation must never hand out what was reserved.
	for i := 0; i < 249; i++ {
		out, err := a.Allocate()
		if err != nil {
			t.Fatalf("Allocate() = %v at %d", err, i)
		}
		if out == addr {
			t.Fatalf("Allocate() handed out the reserved %s", addr)
		}
	}
}

func TestReserveRejectsAddressesOutsideTheUsableRange(t *testing.T) {
	a := mustAllocator(t, "10.0.0.0/24", 5)

	for _, in := range []string{
		"10.0.0.1",   // inside the reserved range
		"10.0.0.255", // broadcast
		"10.0.1.5",   // another block
	} {
		if err := a.Reserve(netip.MustParseAddr(in)); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("Reserve(%s) = %v, want ErrOutOfRange", in, err)
		}
	}
}

func TestReleaseUnknownAddressIsSilent(t *testing.T) {
	a := mustAllocator(t, "10.0.0.0/24", 5)
	before := a.Available()

	a.Release(netip.MustParseAddr("10.0.0.42")) // never allocated
	a.Release(netip.MustParseAddr("10.9.9.9"))  // not even in the block

	if a.Available() != before {
		t.Errorf("Available() = %d after releasing unknown addresses, want %d", a.Available(), before)
	}
}

func TestNewAllocatorRejectsIPv6(t *testing.T) {
	_, err := NewAllocator(netip.MustParsePrefix("fd00::/64"), 0)
	if !errors.Is(err, ErrNotIPv4) {
		t.Fatalf("NewAllocator(fd00::/64) = %v, want ErrNotIPv4", err)
	}
}

func TestNewAllocatorRejectsNegativeReserved(t *testing.T) {
	if _, err := NewAllocator(netip.MustParsePrefix("10.0.0.0/24"), -1); err == nil {
		t.Fatal("NewAllocator with a negative reserved count = nil error, want a failure")
	}
}

// The allocator is called from HTTP handlers, so two concurrent creates must
// never receive the same address.
func TestAllocateIsRaceFree(t *testing.T) {
	a := mustAllocator(t, "10.0.0.0/24", 5)

	const workers = 50
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		out = make(map[netip.Addr]bool, workers)
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			addr, err := a.Allocate()
			if err != nil {
				t.Errorf("Allocate() = %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if out[addr] {
				t.Errorf("address %s handed out twice", addr)
			}
			out[addr] = true
		}()
	}
	wg.Wait()

	if len(out) != workers {
		t.Errorf("got %d distinct addresses, want %d", len(out), workers)
	}
}
