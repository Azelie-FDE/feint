package scaleway_test

import (
	"net/http"
	"testing"
)

// standby is a state of its own, and nothing covered it.
//
// The SDK declares `stopped in place` beside `stopped` (ServerStateStoppedInPlace),
// and the Terraform provider polls for the exact one its `state = "standby"`
// asked for: collapsing both into "stopped" made a plan fail with "expected
// state stopped in place but found stopped". A real provider found that, after
// the emulator had shipped the two actions as synonyms — because `scw` has no
// standby verb and the conformance suite never asked for one.
func TestStopInPlaceReachesItsOwnState(t *testing.T) {
	ts := newTestServer(t)
	const zone = "/instance/v1/zones/fr-par-1"

	status, out := do(t, ts, "POST", zone+"/servers", `{"name":"demo","commercial_type":"DEV1-S"}`)
	if status != http.StatusCreated {
		t.Fatalf("create: status %d", status)
	}
	server, _ := out["server"].(map[string]any)
	id, _ := server["id"].(string)

	state := func() string {
		t.Helper()
		status, out := do(t, ts, "GET", zone+"/servers/"+id, "")
		if status != http.StatusOK {
			t.Fatalf("get: status %d", status)
		}
		s, _ := out["server"].(map[string]any)
		got, _ := s["state"].(string)
		return got
	}
	action := func(name string) {
		t.Helper()
		if status, _ := do(t, ts, "POST", zone+"/servers/"+id+"/action", `{"action":"`+name+`"}`); status != http.StatusAccepted {
			t.Fatalf("%s: status %d", name, status)
		}
	}

	action("poweron")
	if got := state(); got != "running" {
		t.Fatalf("after poweron the state is %q, want running", got)
	}

	action("stop_in_place")
	if got := state(); got != "stopped in place" {
		t.Fatalf("after stop_in_place the state is %q, want \"stopped in place\"", got)
	}

	// And poweroff is not the same action: it reaches the plain stopped state.
	action("poweron")
	action("poweroff")
	if got := state(); got != "stopped" {
		t.Fatalf("after poweroff the state is %q, want stopped", got)
	}
}

// allowed_actions is derived from the state rather than from the action that was
// asked for, which is what keeps a failed start from advertising poweroff on a
// server that never came up. Nothing tested that derivation: an audit replaced
// the whole function with a fixed list and every test still passed.
func TestAllowedActionsFollowTheState(t *testing.T) {
	ts := newTestServer(t)
	const zone = "/instance/v1/zones/fr-par-1"

	status, out := do(t, ts, "POST", zone+"/servers", `{"name":"demo","commercial_type":"DEV1-S"}`)
	if status != http.StatusCreated {
		t.Fatalf("create: status %d", status)
	}
	server, _ := out["server"].(map[string]any)
	id, _ := server["id"].(string)

	allowed := func() map[string]bool {
		t.Helper()
		status, out := do(t, ts, "GET", zone+"/servers/"+id, "")
		if status != http.StatusOK {
			t.Fatalf("get: status %d", status)
		}
		s, _ := out["server"].(map[string]any)
		list, _ := s["allowed_actions"].([]any)
		set := make(map[string]bool, len(list))
		for _, a := range list {
			if name, ok := a.(string); ok {
				set[name] = true
			}
		}
		return set
	}
	action := func(name string) {
		t.Helper()
		if status, _ := do(t, ts, "POST", zone+"/servers/"+id+"/action", `{"action":"`+name+`"}`); status != http.StatusAccepted {
			t.Fatalf("%s: status %d", name, status)
		}
	}

	stopped := allowed()
	if !stopped["poweron"] || stopped["poweroff"] {
		t.Fatalf("a stopped server allows %v; want poweron and not poweroff", stopped)
	}

	action("poweron")
	running := allowed()
	for _, want := range []string{"poweroff", "reboot", "stop_in_place"} {
		if !running[want] {
			t.Fatalf("a running server does not allow %s: %v", want, running)
		}
	}
	if running["poweron"] {
		t.Fatalf("a running server still advertises poweron: %v", running)
	}

	// Standby keeps the machine, so powering it fully off is available from
	// there as much as starting it again.
	action("stop_in_place")
	standby := allowed()
	if !standby["poweron"] || !standby["poweroff"] {
		t.Fatalf("a standby server allows %v; want both poweron and poweroff", standby)
	}
	if standby["reboot"] {
		t.Fatalf("a standby server advertises reboot: %v", standby)
	}
}

// Deleting a server detaches its disks, it does not delete them. That is what
// the real API does, and it is why `scw instance server delete` carries a
// --with-volumes flag: without it, the CLI deletes the server and then removes
// the volumes itself, one call at a time, polling each one. A volume that
// vanished with its server makes those calls 404.
//
// Nothing covered it: an audit removed the detachment loop from the delete path
// and the whole suite still passed.
func TestDeletingAServerLeavesItsVolumeAvailable(t *testing.T) {
	ts := newTestServer(t)
	const zone = "/instance/v1/zones/fr-par-1"

	status, out := do(t, ts, "POST", zone+"/servers", `{"name":"demo","commercial_type":"DEV1-S"}`)
	if status != http.StatusCreated {
		t.Fatalf("create: status %d", status)
	}
	server, _ := out["server"].(map[string]any)
	id, _ := server["id"].(string)
	volumes, _ := server["volumes"].(map[string]any)
	root, _ := volumes["0"].(map[string]any)
	rootID, _ := root["id"].(string)

	if status, _ := do(t, ts, "DELETE", zone+"/servers/"+id, ""); status != http.StatusNoContent {
		t.Fatalf("delete: status %d", status)
	}

	status, out = do(t, ts, "GET", zone+"/volumes/"+rootID, "")
	if status != http.StatusOK {
		t.Fatalf("the root volume went with the server: status %d", status)
	}
	volume, _ := out["volume"].(map[string]any)
	if server := volume["server"]; server != nil {
		t.Fatalf("the volume still belongs to the deleted server: %v", server)
	}
}

// public_ips is served as a list the client can read, and it was once stored as
// an empty literal written at creation and never touched. The fix has no
// regression test, so an audit replaced the field with an empty list and nothing
// failed.
func TestPublicIPsCarryTheAttachedAddress(t *testing.T) {
	ts := newTestServer(t)
	const zone = "/instance/v1/zones/fr-par-1"

	status, out := do(t, ts, "POST", zone+"/ips", `{}`)
	if status != http.StatusCreated {
		t.Fatalf("create ip: status %d (%v)", status, out)
	}
	ip, _ := out["ip"].(map[string]any)
	ipID, _ := ip["id"].(string)
	address, _ := ip["address"].(string)

	status, out = do(t, ts, "POST", zone+"/servers",
		`{"name":"demo","commercial_type":"DEV1-S","public_ip":"`+ipID+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("create server: status %d (%v)", status, out)
	}
	server, _ := out["server"].(map[string]any)
	list, _ := server["public_ips"].([]any)
	if len(list) != 1 {
		t.Fatalf("the server carries %d public ips, want 1: %v", len(list), server["public_ips"])
	}
	first, _ := list[0].(map[string]any)
	if got, _ := first["address"].(string); got != address {
		t.Fatalf("public_ips carries %q, want the address that was attached (%q)", got, address)
	}
}
