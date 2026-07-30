package network

import (
	"errors"
	"net/netip"
	"testing"
)

func TestParseCIDRAcceptsCanonicalBlocks(t *testing.T) {
	for _, in := range []string{"10.0.0.0/16", "192.168.1.0/24", "172.31.0.0/20", "10.0.0.0/32"} {
		p, err := ParseCIDR(in)
		if err != nil {
			t.Fatalf("ParseCIDR(%q) = %v, want no error", in, err)
		}
		if p.String() != in {
			t.Errorf("ParseCIDR(%q) = %s, want the value unchanged", in, p)
		}
	}
}

func TestParseCIDRRejectsHostBits(t *testing.T) {
	// The value a client most often gets wrong, and the one an emulator that
	// stores blindly turns into a subnet nobody can reason about.
	_, err := ParseCIDR("10.0.0.5/24")
	if !errors.Is(err, ErrHostBits) {
		t.Fatalf("ParseCIDR(10.0.0.5/24) = %v, want ErrHostBits", err)
	}
}

func TestParseCIDRRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "10.0.0.0", "10.0.0.0/33", "not-a-cidr", "10.0.0.0/-1"} {
		if _, err := ParseCIDR(in); err == nil {
			t.Errorf("ParseCIDR(%q) = nil error, want a failure", in)
		}
	}
}

func TestCheckMask(t *testing.T) {
	tests := []struct {
		cidr     string
		min, max int
		wantErr  bool
	}{
		{"10.0.0.0/16", 16, 28, false},
		{"10.0.0.0/28", 16, 28, false},
		{"10.0.0.0/8", 16, 28, true},  // too wide
		{"10.0.0.0/30", 16, 28, true}, // too narrow
	}
	for _, tt := range tests {
		p := netip.MustParsePrefix(tt.cidr)
		err := CheckMask(p, tt.min, tt.max)
		if (err != nil) != tt.wantErr {
			t.Errorf("CheckMask(%s, %d, %d) = %v, wantErr %v", tt.cidr, tt.min, tt.max, err, tt.wantErr)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		outer, inner string
		want         bool
	}{
		{"10.0.0.0/16", "10.0.1.0/24", true},
		{"10.0.0.0/16", "10.0.0.0/16", true},
		{"10.0.0.0/16", "10.1.0.0/24", false},
		{"10.0.1.0/24", "10.0.0.0/16", false}, // the child cannot hold the parent
		{"10.0.0.0/16", "fd00::/64", false},   // families do not mix
	}
	for _, tt := range tests {
		got := Contains(netip.MustParsePrefix(tt.outer), netip.MustParsePrefix(tt.inner))
		if got != tt.want {
			t.Errorf("Contains(%s, %s) = %v, want %v", tt.outer, tt.inner, got, tt.want)
		}
	}
}

func TestFirstOverlapNamesTheOffendingBlock(t *testing.T) {
	taken := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("10.0.1.0/24"),
	}

	got, ok := FirstOverlap(netip.MustParsePrefix("10.0.1.128/25"), taken)
	if !ok {
		t.Fatal("FirstOverlap found nothing, want the second block")
	}
	if got.String() != "10.0.1.0/24" {
		t.Errorf("FirstOverlap = %s, want 10.0.1.0/24", got)
	}

	if _, ok := FirstOverlap(netip.MustParsePrefix("10.0.2.0/24"), taken); ok {
		t.Error("FirstOverlap reported an overlap for a free block")
	}
}
