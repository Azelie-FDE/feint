package drift_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stephrobert/feint/internal/drift"
)

func TestBaselineRoundTrip(t *testing.T) {
	rep := drift.Compare("scaleway", upstream(), []string{"instance/v1/API.ListServers"}, nil)
	base := drift.NewBaseline(rep, []string{"instance", "rdb"})

	var buf bytes.Buffer
	if err := base.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := drift.LoadBaseline(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Provider != "scaleway" || len(got.Unknown) != rep.Unknown {
		t.Fatalf("baseline lost content: %+v", got)
	}
}

// The gate that matters: an operation appearing upstream between two runs.
func TestBaselineDetectsANewUpstreamOperation(t *testing.T) {
	before := drift.Compare("scaleway", upstream(), []string{"instance/v1/API.ListServers"}, nil)
	base := drift.NewBaseline(before, nil)

	grown := append(upstream(), drift.Operation{
		Name: "instance/v1/API.BrandNewThing", Product: "instance", Version: "v1",
	})
	after := drift.Compare("scaleway", grown, []string{"instance/v1/API.ListServers"}, nil)

	diff := base.Compare(after)
	if diff.Empty() {
		t.Fatal("expected the new upstream operation to be reported")
	}
	if len(diff.Added) != 1 || diff.Added[0] != "instance/v1/API.BrandNewThing" {
		t.Fatalf("unexpected diff: %+v", diff)
	}

	var buf bytes.Buffer
	if err := diff.WriteText(&buf); err != nil {
		t.Fatalf("write text: %v", err)
	}
	if !strings.Contains(buf.String(), "BrandNewThing") {
		t.Fatalf("the CI log must name the new operation:\n%s", buf.String())
	}
}

// Implementing an operation shrinks the baseline; that is a refresh, not a
// failure, and the message must say so.
func TestBaselineReportsResolvedEntries(t *testing.T) {
	before := drift.Compare("scaleway", upstream(), nil, nil)
	base := drift.NewBaseline(before, nil)

	after := drift.Compare("scaleway", upstream(), []string{"instance/v1/API.ListServers"}, nil)
	diff := base.Compare(after)

	if len(diff.Added) != 0 {
		t.Fatalf("nothing was added upstream: %+v", diff)
	}
	if len(diff.Removed) != 1 || diff.Removed[0] != "instance/v1/API.ListServers" {
		t.Fatalf("expected the newly served operation to leave the baseline: %+v", diff)
	}
}

func TestBaselineNoDrift(t *testing.T) {
	rep := drift.Compare("scaleway", upstream(), []string{"instance/v1/API.ListServers"}, nil)
	base := drift.NewBaseline(rep, nil)

	if diff := base.Compare(rep); !diff.Empty() {
		t.Fatalf("a report compared to its own baseline must be clean: %+v", diff)
	}
}
