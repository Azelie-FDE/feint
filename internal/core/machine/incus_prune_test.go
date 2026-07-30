package machine

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The sweep is the one operation that deletes without being asked which
// resources. Getting it wrong in one direction leaves an emulated bridge
// holding the block the next run wants; getting it wrong in the other removes
// the operator's own instances. Neither shows up in the conformance suite,
// which runs on a host it does not own.

// twoInstances holds one the emulator created and one it did not. The label is
// the only thing that tells them apart: names are not evidence, and an operator
// is free to call their container whatever they like.
const twoInstances = `[
  {"name": "feint-scw-1", "config": {"user.feint.provider": "scaleway"}},
  {"name": "feint-scw-2", "config": {"image.os": "Debian"}},
  {"name": "build-runner", "config": {}}
]`

const twoNetworks = `[
  {"name": "scw-aaaa", "config": {"user.feint.provider": "scaleway"}},
  {"name": "incusbr0", "config": {"ipv4.address": "10.76.154.1/24"}}
]`

const twoACLs = `[
  {"name": "scw-bbbb", "description": "feint security group"},
  {"name": "iso-fnt-aaaa", "description": ""},
  {"name": "corp-baseline", "description": "written by hand"},
  {"name": "iso-lab-net", "description": ""}
]`

// pruneWorld answers the three listings a sweep starts from.
func pruneWorld() map[string]string {
	return map[string]string{
		"/1.0/instances?recursion=1":    twoInstances,
		"/1.0/networks?recursion=1":     twoNetworks,
		"/1.0/network-acls?recursion=1": twoACLs,
		"/forwards?recursion=1":         `[]`,
	}
}

func TestPruneRemovesOnlyWhatCarriesTheLabel(t *testing.T) {
	f := &fakeRuntime{answers: pruneWorld()}
	d := newFakeDriver(f)

	pruned, err := d.Prune(context.Background())
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned.Machines != 1 || pruned.Networks != 1 || pruned.Firewalls != 2 {
		t.Fatalf("swept %+v, want 1 machine, 1 network, 2 rule sets", pruned)
	}

	// A name that merely looks like the emulator's is not enough, and neither
	// is a container that happens to sit next to ours.
	for _, untouched := range []string{"feint-scw-2", "build-runner", "incusbr0", "corp-baseline", "iso-lab-net"} {
		for _, cmd := range f.commands() {
			if strings.Contains(cmd, "delete") && strings.Contains(cmd, untouched) {
				t.Errorf("the sweep touched %q, which the emulator did not create: %q", untouched, cmd)
			}
		}
	}

	// An isolation rule set can exist without a description, because the
	// description is written by the PUT that follows the create. It is
	// recognised by the emulator's own network prefix inside its name — not by
	// a bare iso-, which claimed an operator's rule sets too.
	if len(f.matching("acl delete iso-fnt-aaaa")) != 1 {
		t.Error("the isolation rule set was not swept")
	}
}

func TestPruneRetriesANetworkBlockedByANeighbour(t *testing.T) {
	// An OVN network must go before the uplink it draws from, and the listing
	// carries no such order. The first pass removes what it can, the second
	// retries what a neighbour was still standing in front of.
	world := pruneWorld()
	world["/1.0/networks?recursion=1"] = `[
	  {"name": "scw-uplink", "config": {"user.feint.provider": "scaleway"}},
	  {"name": "scw-aaaa", "config": {"user.feint.provider": "scaleway"}}
	]`

	attempts := 0
	f := &fakeRuntime{answers: world}
	f.hook = func(_ int, args []string) ([]byte, error, bool) {
		if len(args) >= 3 && args[0] == "network" && args[1] == "delete" && args[2] == "scw-uplink" {
			attempts++
			if attempts == 1 {
				return nil, errors.New("Error: Network is in use by 1 network"), true
			}
		}
		return nil, nil, false
	}
	d := newFakeDriver(f)

	pruned, err := d.Prune(context.Background())
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("the blocked network was tried %d time(s), want 2", attempts)
	}
	if pruned.Networks != 2 {
		t.Fatalf("swept %d network(s), want both", pruned.Networks)
	}
}

func TestPruneReportsWhatItCouldNotRemoveAndKeepsGoing(t *testing.T) {
	// One stuck resource must not keep the rest of the environment dirty: the
	// next run needs the blocks back, not an explanation.
	f := &fakeRuntime{answers: pruneWorld(), fail: map[string]error{
		"delete --force feint-scw-1": errors.New("Error: Instance is busy"),
	}}
	d := newFakeDriver(f)

	pruned, err := d.Prune(context.Background())
	if err == nil {
		t.Fatal("a resource left behind must be reported, not swallowed")
	}
	if !strings.Contains(err.Error(), "feint-scw-1") {
		t.Errorf("the error should name what stayed, got %v", err)
	}
	if pruned.Machines != 0 {
		t.Errorf("a machine that did not go must not be counted, got %d", pruned.Machines)
	}
	if pruned.Networks != 1 || pruned.Firewalls != 2 {
		t.Errorf("the sweep stopped early: %+v", pruned)
	}
}
