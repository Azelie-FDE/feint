package drift_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stephrobert/feint/internal/drift"
)

func TestScanScalewaySDK(t *testing.T) {
	ops, err := drift.ScanScalewaySDK(filepath.Join("testdata", "fake-scaleway-sdk"))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	names := make([]string, 0, len(ops))
	for _, op := range ops {
		names = append(names, op.Name)
	}

	want := []string{
		"instance/v1/API.CreateServer",
		"instance/v1/API.GetServer",
		"instance/v1/API.ListServers",
		"instance/v1/API.ServerAction",
		"instance/v1/API.UpdateServer",
		"instance/v1/ZonedAPI.ListVolumes",
		"rdb/v1/API.ListInstances",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("unexpected surface\n got: %v\nwant: %v", names, want)
	}

	// The traps must not appear: a comment, a string literal, an unexported
	// method and a non-API receiver are all invisible to the real API surface.
	// So are the two shapes of client-side convenience — a poller, and a method
	// composing exported calls — and a constant accessor that reaches nothing.
	for _, ghost := range []string{"GhostFromAComment", "GhostFromAString", "internalHelper", "NotAnOperation", "updateServer", "WaitForServer", "ServerActionAndWait", "Zones"} {
		for _, name := range names {
			if strings.Contains(name, ghost) {
				t.Fatalf("scanner picked up %q, which is not an API operation", ghost)
			}
		}
	}
}

func TestScanScalewaySDKMissingCheckout(t *testing.T) {
	if _, err := drift.ScanScalewaySDK(filepath.Join("testdata", "does-not-exist")); err == nil {
		t.Fatal("expected a clear error when the SDK checkout is missing")
	}
}

func TestScanProductAndVersionAreCarried(t *testing.T) {
	ops, err := drift.ScanScalewaySDK(filepath.Join("testdata", "fake-scaleway-sdk"))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, op := range ops {
		if op.Product == "" || op.Version == "" {
			t.Fatalf("operation %q lost its product/version: %+v", op.Name, op)
		}
	}
}
