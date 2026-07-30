package cloudinit_test

import (
	"strings"
	"testing"

	"github.com/stephrobert/feint/internal/core/cloudinit"
)

func TestRenderInstallsTheKeyForRoot(t *testing.T) {
	out, err := cloudinit.Render(cloudinit.Spec{
		Distribution:   "ubuntu:22.04",
		Hostname:       "web-1",
		User:           "root",
		AuthorizedKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 demo"},
		InstallSSHD:    true,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	for _, want := range []string{
		"#cloud-config",
		"hostname: web-1",
		"- name: root",
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 demo",
		"disable_root: false", // Scaleway logs in as root
		"openssh-server",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// The trap dsoxlab documented: cloud-init locks the account by default and
// OpenSSH 9+ then refuses a key-based login. Losing these two lines silently
// produces a machine that boots, holds the right key, and never answers.
func TestRenderNeverLocksTheAccount(t *testing.T) {
	for _, distro := range []string{"ubuntu", "debian", "almalinux", "rocky", "unknown-distro"} {
		t.Run(distro, func(t *testing.T) {
			out, err := cloudinit.Render(cloudinit.Spec{
				Distribution:   distro,
				User:           "cloud",
				AuthorizedKeys: []string{"ssh-ed25519 AAAA demo"},
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(out, "lock_passwd: false") {
				t.Errorf("%s: lock_passwd must be forced to false:\n%s", distro, out)
			}
			if !strings.Contains(out, `passwd: "*"`) {
				t.Errorf("%s: passwd must be \"*\":\n%s", distro, out)
			}
		})
	}
}

func TestRenderGivesANonRootUserSudo(t *testing.T) {
	out, err := cloudinit.Render(cloudinit.Spec{
		Distribution:   "debian",
		User:           "outscale",
		AuthorizedKeys: []string{"ssh-ed25519 AAAA demo"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "- name: outscale") {
		t.Fatalf("the provider's user is missing:\n%s", out)
	}
	if !strings.Contains(out, "NOPASSWD:ALL") {
		t.Fatalf("a provisioned account needs sudo:\n%s", out)
	}
	if !strings.Contains(out, "disable_root: true") {
		t.Fatalf("root must stay closed when the cloud provisions its own user:\n%s", out)
	}
}

func TestRenderPicksTheRightAdminGroup(t *testing.T) {
	cases := map[string]string{
		"ubuntu":    "groups: [sudo]",
		"debian":    "groups: [sudo]",
		"almalinux": "groups: [wheel]",
		"rocky":     "groups: [wheel]",
	}
	for distro, want := range cases {
		out, err := cloudinit.Render(cloudinit.Spec{
			Distribution:   distro,
			User:           "cloud",
			AuthorizedKeys: []string{"ssh-ed25519 AAAA demo"},
		})
		if err != nil {
			t.Fatalf("%s: render: %v", distro, err)
		}
		if !strings.Contains(out, want) {
			t.Errorf("%s: expected %q:\n%s", distro, want, out)
		}
	}
}

func TestRenderWithoutKeysIsEmpty(t *testing.T) {
	out, err := cloudinit.Render(cloudinit.Spec{Distribution: "ubuntu", User: "root"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out != "" {
		t.Fatalf("provisioning an account nobody can reach is pointless, got:\n%s", out)
	}
}

func TestRenderSkipsThePackageInstallWhenNotNeeded(t *testing.T) {
	out, err := cloudinit.Render(cloudinit.Spec{
		Distribution:   "ubuntu",
		User:           "root",
		AuthorizedKeys: []string{"ssh-ed25519 AAAA demo"},
		InstallSSHD:    false,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "openssh-server") {
		t.Fatalf("cloud images already carry sshd, no package should be installed:\n%s", out)
	}
}
