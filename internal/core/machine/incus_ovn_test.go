package machine

import (
	"net/netip"
	"testing"
)

// The two ranges feed one uplink: an overlap hands the same address to a
// bridged guest and to an OVN router, and the collision surfaces as a machine
// that answers for a gateway.
func TestUplinkRangesSplitTheBlockWithoutOverlap(t *testing.T) {
	tests := []struct {
		cidr     string
		wantDHCP string
		wantOVN  string
		wantErr  bool
	}{
		{cidr: "10.209.83.0/24", wantDHCP: "10.209.83.2-10.209.83.127", wantOVN: "10.209.83.128-10.209.83.254"},
		{cidr: "192.168.240.0/28", wantDHCP: "192.168.240.2-192.168.240.7", wantOVN: "192.168.240.8-192.168.240.14"},
		{cidr: "10.0.0.0/30", wantErr: true}, // no room for two ranges
		{cidr: "fd42::/64", wantErr: true},   // IPv4 only
		{cidr: "10.4.0.0/16", wantDHCP: "10.4.0.2-10.4.127.255", wantOVN: "10.4.128.0-10.4.255.254"},
	}
	for _, tt := range tests {
		prefix, err := netip.ParsePrefix(tt.cidr)
		if err != nil {
			t.Fatalf("bad test input %q: %v", tt.cidr, err)
		}
		dhcp, ovn, err := uplinkRanges(prefix)
		if tt.wantErr {
			if err == nil {
				t.Errorf("uplinkRanges(%s) = %q, %q, want an error", tt.cidr, dhcp, ovn)
			}
			continue
		}
		if err != nil {
			t.Errorf("uplinkRanges(%s): %v", tt.cidr, err)
			continue
		}
		if dhcp != tt.wantDHCP || ovn != tt.wantOVN {
			t.Errorf("uplinkRanges(%s) = %q, %q, want %q, %q", tt.cidr, dhcp, ovn, tt.wantDHCP, tt.wantOVN)
		}
	}
}

// The driver's name is what the health endpoint publishes and what the
// conformance suite branches on, so it is part of the contract.
func TestIncusDriverNames(t *testing.T) {
	tests := []struct {
		vm, ovn bool
		want    string
	}{
		{want: "incus"},
		{vm: true, want: "incus-vm"},
		{ovn: true, want: "incus-ovn"},
		{vm: true, ovn: true, want: "incus-ovn-vm"},
	}
	for _, tt := range tests {
		d := &Incus{VM: tt.vm, OVN: tt.ovn}
		if got := d.Name(); got != tt.want {
			t.Errorf("(&Incus{VM: %v, OVN: %v}).Name() = %q, want %q", tt.vm, tt.ovn, got, tt.want)
		}
	}
}

// Only the OVN mode may claim native isolation: a pack hearing "true" stops
// writing reject rules, and on bridges nothing else would separate anything.
func TestNativeIsolationFollowsTheMode(t *testing.T) {
	if NewIncus().NativeIsolation() {
		t.Error("the bridge driver claims native isolation")
	}
	if !NewIncusOVN().NativeIsolation() {
		t.Error("the OVN driver does not claim native isolation")
	}
}
