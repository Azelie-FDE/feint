package machine

import "testing"

// gatewayAddress is where a wrong assumption becomes a network nobody can reach:
// Incus wants the gateway carrying the block's mask, not the block itself.
func TestGatewayAddress(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		gateway string
		want    string
		wantErr bool
	}{
		{
			name:    "explicit gateway keeps the block mask",
			cidr:    "10.0.0.0/24",
			gateway: "10.0.0.1",
			want:    "10.0.0.1/24",
		},
		{
			name: "empty gateway defaults to the first address",
			cidr: "192.168.42.0/24",
			want: "192.168.42.1/24",
		},
		{
			name: "the mask is the block mask, not a guess",
			cidr: "172.31.0.0/20",
			want: "172.31.0.1/20",
		},
		{
			name:    "a gateway outside the block is refused",
			cidr:    "10.0.0.0/24",
			gateway: "10.0.1.1",
			wantErr: true,
		},
		{
			name:    "an unparseable block is refused",
			cidr:    "10.0.0.0",
			wantErr: true,
		},
		{
			name:    "an unparseable gateway is refused",
			cidr:    "10.0.0.0/24",
			gateway: "not-an-address",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gatewayAddress(tt.cidr, tt.gateway)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("gatewayAddress(%q, %q) = %q, want an error", tt.cidr, tt.gateway, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("gatewayAddress(%q, %q) = %v", tt.cidr, tt.gateway, err)
			}
			if got != tt.want {
				t.Errorf("gatewayAddress(%q, %q) = %q, want %q", tt.cidr, tt.gateway, got, tt.want)
			}
		})
	}
}

// The pick is by exact device name: a substring check let an existing eth10
// shadow eth1, and a machine with a profile eth0 must not receive a second one.
func TestFreeInterfacePicksTheFirstUnusedName(t *testing.T) {
	nic := func(names ...string) map[string]map[string]string {
		devices := make(map[string]map[string]string, len(names))
		for _, name := range names {
			devices[name] = map[string]string{"type": "nic"}
		}
		return devices
	}

	tests := []struct {
		name    string
		devices map[string]map[string]string
		want    string
	}{
		{name: "empty machine starts at eth1, never eth0", devices: nic(), want: "eth1"},
		{name: "profile eth0 does not block eth1", devices: nic("eth0"), want: "eth1"},
		{name: "next free after a gap", devices: nic("eth0", "eth1", "eth3"), want: "eth2"},
		{name: "eth10 does not shadow eth1", devices: nic("eth0", "eth10"), want: "eth1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := freeInterface(tt.devices); got != tt.want {
				t.Errorf("freeInterface(%v) = %q, want %q", tt.devices, got, tt.want)
			}
		})
	}
}
