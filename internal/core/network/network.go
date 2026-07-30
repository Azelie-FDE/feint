// Package network models the addressing plane of an emulated cloud: parsing and
// validating CIDR blocks, then handing out addresses from them.
//
// It exists because of what every local cloud emulator does instead. They store
// the block a client asked for, never read it again, and answer with whatever
// address the backing container happened to get on the default bridge. The
// addressing plan is then decorative: a subnet reports a fixed count of free
// addresses whatever its mask, two overlapping subnets are both accepted, and
// the address returned by the API belongs to no declared range. Anything a test
// asserts about addressing proves nothing.
//
// The package holds no provider knowledge, and it is not a store: providers
// disagree on the mask range they accept and on how many addresses they reserve
// at the bottom of a subnet, so both are parameters. Persisting an allocation
// is the caller's job, which Reserve makes possible after a restart.
package network

import (
	"errors"
	"fmt"
	"net/netip"
)

// ErrHostBits reports a block whose host bits are set, such as 10.0.0.5/24.
// Callers that would rather normalise than reject can call Prefix.Masked on the
// value they parsed themselves; rejecting is the safer default, because a
// client that sends host bits is usually sending the wrong value.
var ErrHostBits = errors.New("network: CIDR has host bits set")

// ParseCIDR parses a block in its canonical form. The returned prefix is always
// masked, so callers can compare prefixes for equality.
func ParseCIDR(s string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("network: parse CIDR %q: %w", s, err)
	}
	if p.Masked() != p {
		return netip.Prefix{}, fmt.Errorf("%w: %s (did you mean %s?)", ErrHostBits, s, p.Masked())
	}
	return p, nil
}

// CheckMask reports whether the prefix length sits within the range a provider
// accepts, minBits and maxBits included. AWS takes /16 to /28 for a VPC; every
// provider has its own window, which is why this is an argument.
func CheckMask(p netip.Prefix, minBits, maxBits int) error {
	if bits := p.Bits(); bits < minBits || bits > maxBits {
		return fmt.Errorf("network: prefix /%d outside the accepted range /%d to /%d", bits, minBits, maxBits)
	}
	return nil
}

// Contains reports whether outer fully contains inner. netip covers an address
// inside a prefix; a subnet inside its parent network is what an emulator needs,
// and it is the check that tells a client its subnet does not fit its VPC.
func Contains(outer, inner netip.Prefix) bool {
	if !outer.IsValid() || !inner.IsValid() {
		return false
	}
	if outer.Addr().Is4() != inner.Addr().Is4() {
		return false
	}
	return inner.Bits() >= outer.Bits() && outer.Contains(inner.Addr())
}

// FirstOverlap returns the first prefix in taken that overlaps p, and whether
// one was found. Creating a subnet means checking it against every sibling, and
// the offending block is what the error message has to name.
func FirstOverlap(p netip.Prefix, taken []netip.Prefix) (netip.Prefix, bool) {
	for _, other := range taken {
		if p.Overlaps(other) {
			return other, true
		}
	}
	return netip.Prefix{}, false
}
