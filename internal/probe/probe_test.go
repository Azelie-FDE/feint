package probe_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stephrobert/feint/internal/contract"
	"github.com/stephrobert/feint/internal/core/emulator"
	"github.com/stephrobert/feint/internal/probe"
	"github.com/stephrobert/feint/internal/providers/exoscale"
	"github.com/stephrobert/feint/internal/providers/outscale"
	"github.com/stephrobert/feint/internal/providers/scaleway"
)

// The probe runs against the emulator in this process, so it is the half of
// conformance that needs no client installed and runs in CI on `go test`. The
// other half — the suites under tools/conformance/ — drives the real clients and
// proves behaviour, and neither replaces the other.

func packOf(t *testing.T, provider string, env *emulator.Env) emulator.Pack {
	t.Helper()
	switch provider {
	case scaleway.Name:
		return scaleway.New(env)
	case outscale.Name:
		return outscale.New(env)
	case exoscale.Name:
		return exoscale.New(env)
	}
	t.Fatalf("no pack named %q", provider)
	return nil
}

func runProbe(t *testing.T, provider string) probe.Report {
	t.Helper()

	doc, err := contract.Load(filepath.Join("..", "..", "contracts", provider+".json"))
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}

	env := emulator.DefaultEnv()
	env.Contracts = map[string]*contract.Doc{provider: doc}
	pack := packOf(t, provider, env)

	srv, err := emulator.NewServer(env, pack)
	if err != nil {
		t.Fatalf("build emulator: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	mounted := make([]contract.MountedRoute, 0, len(pack.Routes()))
	for _, r := range pack.Routes() {
		mounted = append(mounted, contract.MountedRoute{
			Method: r.Method, Path: r.Path, Operation: r.Operation,
		})
	}

	runner := &probe.Runner{Doc: doc, Base: ts.URL, Client: ts.Client()}
	report, err := runner.Run(context.Background(), mounted)
	if err != nil {
		t.Fatalf("%s: %v", provider, err)
	}
	return report
}

// TestEveryRouteAnswersItsContract is the check the probe exists to be. A route
// that answers a shape its provider does not define fails here, without anybody
// having written a case for it — which is what makes the remaining hundred and
// fifty operations affordable.
func TestEveryRouteAnswersItsContract(t *testing.T) {
	for _, provider := range []string{outscale.Name, exoscale.Name} {
		report := runProbe(t, provider)
		for _, failure := range report.Failures() {
			switch {
			case failure.Err != nil:
				t.Errorf("%s %s: %v", provider, failure.Operation, failure.Err)
			case len(failure.Violations) > 0:
				t.Errorf("%s %s does not match the contract: %v",
					provider, failure.Operation, failure.Violations)
			default:
				t.Errorf("%s %s answered %d to a request built from its own schema",
					provider, failure.Operation, failure.Status)
			}
		}
		t.Logf("%s", report)
	}
}

// Scaleway is probed too, and expected to prove less. Its document declares no
// request schema at all — the bodies are inline, which the extraction cannot
// name — so only the operations that take no body are reachable. Stated as a
// test so the day the extraction learns inline bodies, the gain is visible
// rather than assumed.
func TestScalewayProbesWhatItsDocumentAllows(t *testing.T) {
	report := runProbe(t, scaleway.Name)
	for _, failure := range report.Failures() {
		t.Errorf("scaleway %s: status %d, %v, %v",
			failure.Operation, failure.Status, failure.Violations, failure.Err)
	}
	if report.Probed() == 0 {
		t.Error("nothing was probed at all, which means the plan is empty rather than limited")
	}
	t.Logf("%s", report)
}

// A probe must never make the score go up. The backlog of routes nobody has
// proven is computed from real client calls alone, and this is the assertion
// that keeps it that way — the one thing that stops this package from becoming
// the defect it was built to avoid.
func TestProbingDoesNotCountAsProven(t *testing.T) {
	doc, err := contract.Load(filepath.Join("..", "..", "contracts", outscale.Name+".json"))
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}

	env := emulator.DefaultEnv()
	env.Contracts = map[string]*contract.Doc{outscale.Name: doc}
	pack := outscale.New(env)
	srv, err := emulator.NewServer(env, pack)
	if err != nil {
		t.Fatalf("build emulator: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	mounted := make([]contract.MountedRoute, 0, len(pack.Routes()))
	for _, r := range pack.Routes() {
		mounted = append(mounted, contract.MountedRoute{
			Method: r.Method, Path: r.Path, Operation: r.Operation,
		})
	}
	runner := &probe.Runner{Doc: doc, Base: ts.URL, Client: ts.Client()}
	report, err := runner.Run(context.Background(), mounted)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Probed() == 0 {
		t.Fatal("the probe reached nothing, so this proves nothing either")
	}

	status, body := get(t, ts, "/_feint/conformance")
	if status != 200 {
		t.Fatalf("conformance report answered %d", status)
	}
	if !strings.Contains(body, `"exercised":0`) {
		t.Errorf("probing moved the exercised count, which must only ever be a "+
			"real client: %s", body)
	}
	if strings.Contains(body, `"probed":0`) {
		t.Error("the probe reached routes and none was counted as probed")
	}
}

func get(t *testing.T, ts *httptest.Server, path string) (int, string) {
	t.Helper()
	res, err := ts.Client().Get(ts.URL + path) //nolint:noctx // test client
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	buf := make([]byte, 8192)
	n, _ := res.Body.Read(buf)
	return res.StatusCode, string(buf[:n])
}
