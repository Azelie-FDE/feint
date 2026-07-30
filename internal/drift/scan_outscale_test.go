package drift_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stephrobert/feint/internal/drift"
)

func TestScanOutscaleSDK(t *testing.T) {
	ops, err := drift.ScanOutscaleSDK(filepath.Join("testdata", "fake-outscale-sdk"))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	names := make([]string, 0, len(ops))
	for _, op := range ops {
		names = append(names, op.Name)
	}

	want := []string{
		"oks/Client.ListProjects",
		"osc/Client.CreateVms",
		"osc/Client.ReadVms",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("unexpected surface\n got: %v\nwant: %v", names, want)
	}

	// Every trap of this SDK shape: the raw-body twin of an operation, the
	// lower-level client that mirrors all of them, its plumbing, the response
	// types named after operations, an unexported helper, and a hand-written
	// package with no generated client at all.
	for _, ghost := range []string{"WithBody", "Raw", "LogRequest", "StatusCode", "newRequest", "Do"} {
		for _, name := range names {
			if strings.Contains(name, ghost) {
				t.Fatalf("scanner picked up %q, which is not an API operation", ghost)
			}
		}
	}
}

func TestScanOutscaleCarriesTheServiceAsProduct(t *testing.T) {
	// Services are separate surfaces on separate hosts, so the product is what
	// makes "decline the Kubernetes service" one decision instead of
	// twenty-seven unrelated ones.
	ops, err := drift.ScanOutscaleSDK(filepath.Join("testdata", "fake-outscale-sdk"))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, op := range ops {
		service, _, found := strings.Cut(op.Name, "/")
		if !found || op.Product != service {
			t.Fatalf("operation %q carries product %q, want %q", op.Name, op.Product, service)
		}
	}
}

func TestScanOutscaleRefusesASilentlyEmptySurface(t *testing.T) {
	// A checkout whose packages are all plumbing yields nothing, and nothing
	// must be an error: reported as an empty API it would show as full coverage
	// of a surface nobody scanned, which is the one outcome this package exists
	// to prevent. The fixture has a pkg/ directory on purpose — pointing at a
	// missing tree fails earlier, on the read, and would never reach the guard.
	if _, err := drift.ScanOutscaleSDK(filepath.Join("testdata", "fake-outscale-sdk-plumbing")); err == nil {
		t.Fatal("expected an error when no generated client is found")
	}
	if _, err := drift.ScanOutscaleSDK(filepath.Join("testdata", "does-not-exist")); err == nil {
		t.Fatal("expected a clear error when the SDK checkout is missing")
	}
}
