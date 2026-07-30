package machine

import (
	"errors"
	"testing"
)

// A network name becomes a host interface, and Linux caps those at 15
// characters. Provider IDs are UUIDs, so this is not a corner case: it is every
// call.
func TestNetworkNameFitsTheInterfaceLimit(t *testing.T) {
	ids := []string{
		"11111111-2222-3333-4444-555555555555",
		"11111111-2222-3333-4444-666666666666", // shares its first 23 characters
		"fr-par-1/short",
		"",
	}

	seen := make(map[string]string, len(ids))
	for _, id := range ids {
		got := NetworkName("scw", id)
		if len(got) > MaxNetworkNameLen {
			t.Errorf("NetworkName(scw, %q) = %q, %d characters, limit is %d",
				id, got, len(got), MaxNetworkNameLen)
		}
		if !safeName.MatchString(got) {
			t.Errorf("NetworkName(scw, %q) = %q, which safeName rejects", id, got)
		}
		if other, dup := seen[got]; dup {
			t.Errorf("NetworkName collided: %q and %q both give %q", id, other, got)
		}
		seen[got] = id
	}

	// Stable across calls, or a restart would look for a network that no longer
	// matches the one it created.
	if a, b := NetworkName("scw", ids[0]), NetworkName("scw", ids[0]); a != b {
		t.Errorf("NetworkName is not stable: %q then %q", a, b)
	}
}

// Names reach a command line, so the guard has to hold whatever a pack passes.
func TestSafeNameRejectsShellMetacharacters(t *testing.T) {
	for _, in := range []string{"", "-flag", "a b", "a;rm -rf /", "a/b", "a$(id)", "a`id`"} {
		if safeName.MatchString(in) {
			t.Errorf("safeName accepted %q", in)
		}
	}
	for _, in := range []string{"feint-scw-fr-par-1-abc", "net_1", "a.b-c"} {
		if !safeName.MatchString(in) {
			t.Errorf("safeName rejected %q", in)
		}
	}
}

// Reading "it does not exist" out of prose is a last resort, and a broad match
// once cost this project a leak: "Storage pool not found" was read as "the
// instance is gone", so Remove reported success and left the instance running.
// The match has to be narrow enough that a message about something else never
// passes for the object being absent.
func TestIsNotFoundStaysNarrow(t *testing.T) {
	gone := []string{
		"Error: Instance not found",
		"Error: Network not found",
		"Error: Network ACL not found",
		"Error: Network forward not found",
		"Error: Storage volume not found",
		"Error: No such object",
	}
	for _, msg := range gone {
		if !isNotFound(errors.New(msg)) {
			t.Errorf("isNotFound(%q) = false, want true", msg)
		}
	}

	present := []string{
		"Error: Storage pool not found",
		"Error: Project not found",
		"Error: Profile not found",
		"Error: Image not found",
		"Error: Instance is busy",
		"Error: not found",
	}
	for _, msg := range present {
		if isNotFound(errors.New(msg)) {
			t.Errorf("isNotFound(%q) = true: a message about something else "+
				"passed for the object being gone", msg)
		}
	}
}
