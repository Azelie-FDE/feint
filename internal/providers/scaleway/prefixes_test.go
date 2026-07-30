package scaleway_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stephrobert/feint/internal/core/emulator"
	"github.com/stephrobert/feint/internal/providers/scaleway"
)

// The prefix list is what decides whether a request that matched no route gets
// this pack's error or net/http's plain text. A product added without its prefix
// would answer with the latter for every operation it does not yet serve, which
// is most of them, and nothing else would notice.
func TestEveryRouteFallsUnderADeclaredPrefix(t *testing.T) {
	pack := scaleway.New(emulator.DefaultEnv())
	unrouted, ok := any(pack).(emulator.Unrouted)
	if !ok {
		t.Fatal("the pack no longer declares its URL space")
	}
	prefixes := unrouted.Prefixes()

	for _, route := range pack.Routes() {
		covered := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(route.Path, prefix) {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("route %s %s falls under no declared prefix: an unserved operation "+
				"of that product would answer in text/plain", route.Method, route.Path)
		}
	}
}

// A prefix matching no route is the mirror defect: it claims a product this pack
// does not serve at all, so requests meant for another pack, or for nothing,
// come back wearing Scaleway's error shape.
func TestEveryDeclaredPrefixIsActuallyServed(t *testing.T) {
	pack := scaleway.New(emulator.DefaultEnv())
	unrouted, _ := any(pack).(emulator.Unrouted)

	for _, prefix := range unrouted.Prefixes() {
		used := false
		for _, route := range pack.Routes() {
			if strings.HasPrefix(route.Path, prefix) {
				used = true
				break
			}
		}
		if !used {
			t.Errorf("prefix %q claims a product no route serves", prefix)
		}
	}
}

func TestAnUnservedOperationIsReadableByTheSDK(t *testing.T) {
	ts := newTestServer(t)

	// Placement groups exist upstream and are on the triage list, so this is the
	// answer a real caller meets today for a third of the instance product.
	status, body := do(t, ts, "POST", "/instance/v1/zones/fr-par-1/placement_groups", "{}")

	if status != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501: the operation exists upstream, it is not served here", status)
	}
	// Reaching this point at all is half the assertion: do() decodes the body as
	// JSON and fails the test otherwise, which is exactly what the SDK does.

	// Not "not_found": that type maps onto ResourceNotFoundError in the SDK, so a
	// caller branching on errors.As would be told a resource is missing when an
	// operation is merely unserved.
	if body["type"] != "not_emulated" {
		t.Errorf("type = %v, want not_emulated", body["type"])
	}
	if msg, _ := body["message"].(string); !strings.Contains(msg, "placement_groups") {
		t.Errorf("message = %q, want it to name the path", msg)
	}
}
