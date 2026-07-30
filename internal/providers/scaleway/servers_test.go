package scaleway_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stephrobert/feint/internal/core/emulator"
	"github.com/stephrobert/feint/internal/core/store"
	"github.com/stephrobert/feint/internal/providers/scaleway"
)

// newTestServer returns an emulator with deterministic time and IDs, so a
// response body can be compared field by field.
func newTestServer(t testing.TB) *httptest.Server {
	t.Helper()

	var seq int
	env := &emulator.Env{
		Store: store.New(),
		Now:   func() time.Time { return time.Unix(1700000000, 0).UTC() },
		NewID: func() string {
			seq++
			return fmt.Sprintf("00000000-0000-4000-8000-%012d", seq)
		},
	}
	srv, err := emulator.NewServer(env, scaleway.New(env))
	if err != nil {
		t.Fatalf("build emulator: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, ts *httptest.Server, method, path, body string) (int, map[string]any) {
	t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Auth-Token", "irrelevant-the-emulator-ignores-it")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	out := map[string]any{}
	if resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("%s %s: decode body: %v", method, path, err)
		}
	}
	return resp.StatusCode, out
}

func TestServerLifecycle(t *testing.T) {
	ts := newTestServer(t)
	const zone = "/instance/v1/zones/fr-par-1"

	status, created := do(t, ts, "POST", zone+"/servers", `{"name":"web-1","commercial_type":"DEV1-S"}`)
	if status != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d (%v)", status, created)
	}
	server, _ := created["server"].(map[string]any)
	id, _ := server["id"].(string)
	if id == "" {
		t.Fatalf("create: no id in %v", created)
	}
	if server["state"] != "stopped" {
		t.Fatalf("create: expected a stopped server, got %v", server["state"])
	}
	if server["zone"] != "fr-par-1" {
		t.Fatalf("create: expected the zone to be echoed, got %v", server["zone"])
	}
	if server["project"] == "" || server["project"] == nil {
		t.Fatal("create: expected a default project to be filled in")
	}

	// Terraform reads back right after the create and compares field by field.
	status, got := do(t, ts, "GET", zone+"/servers/"+id, "")
	if status != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", status)
	}
	readBack, _ := got["server"].(map[string]any)
	for _, field := range []string{"id", "name", "commercial_type", "state", "zone", "creation_date"} {
		if fmt.Sprint(readBack[field]) != fmt.Sprint(server[field]) {
			t.Fatalf("get: field %q changed between create and read: %v vs %v", field, server[field], readBack[field])
		}
	}

	// Deleting a running server must fail, which is how a client learns it has
	// to power off first.
	if status, _ := do(t, ts, "POST", zone+"/servers/"+id+"/action", `{"action":"poweron"}`); status != http.StatusAccepted {
		t.Fatalf("poweron: expected 202, got %d", status)
	}
	status, running := do(t, ts, "GET", zone+"/servers/"+id, "")
	if s := running["server"].(map[string]any)["state"]; s != "running" {
		t.Fatalf("poweron: expected running, got %v (status %d)", s, status)
	}
	if status, _ := do(t, ts, "DELETE", zone+"/servers/"+id, ""); status != http.StatusConflict {
		t.Fatalf("delete running: expected 409, got %d", status)
	}

	if status, _ := do(t, ts, "POST", zone+"/servers/"+id+"/action", `{"action":"poweroff"}`); status != http.StatusAccepted {
		t.Fatalf("poweroff: expected 202, got %d", status)
	}
	if status, _ := do(t, ts, "DELETE", zone+"/servers/"+id, ""); status != http.StatusNoContent {
		t.Fatalf("delete stopped: expected 204, got %d", status)
	}
	if status, _ := do(t, ts, "GET", zone+"/servers/"+id, ""); status != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", status)
	}
}

func TestListPagination(t *testing.T) {
	ts := newTestServer(t)
	const zone = "/instance/v1/zones/fr-par-1"

	for i := range 5 {
		body := fmt.Sprintf(`{"name":"web-%d","commercial_type":"DEV1-S"}`, i)
		if status, _ := do(t, ts, "POST", zone+"/servers", body); status != http.StatusCreated {
			t.Fatalf("seed %d: unexpected status %d", i, status)
		}
	}

	cases := []struct {
		query     string
		wantCount int
	}{
		{"", 5},
		{"?page_size=2", 2},
		{"?page_size=2&page=3", 1},
		{"?page_size=2&page=9", 0},
		{"?name=web-3", 1},
	}
	for _, tc := range cases {
		t.Run("list"+tc.query, func(t *testing.T) {
			status, body := do(t, ts, "GET", zone+"/servers"+tc.query, "")
			if status != http.StatusOK {
				t.Fatalf("expected 200, got %d", status)
			}
			servers, _ := body["servers"].([]any)
			if len(servers) != tc.wantCount {
				t.Fatalf("expected %d servers, got %d", tc.wantCount, len(servers))
			}
			if _, ok := body["total_count"]; !ok {
				t.Fatal("total_count is missing: the SDK pagination loop depends on it")
			}
		})
	}
}

func TestListIsZoneScoped(t *testing.T) {
	ts := newTestServer(t)
	if status, _ := do(t, ts, "POST", "/instance/v1/zones/fr-par-1/servers", `{"name":"a","commercial_type":"DEV1-S"}`); status != http.StatusCreated {
		t.Fatal("seed failed")
	}
	_, body := do(t, ts, "GET", "/instance/v1/zones/nl-ams-1/servers", "")
	servers, _ := body["servers"].([]any)
	if len(servers) != 0 {
		t.Fatalf("a server created in fr-par-1 leaked into nl-ams-1: %v", servers)
	}
}

func TestErrorShapes(t *testing.T) {
	ts := newTestServer(t)

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantType   string
	}{
		{"unknown zone", "GET", "/instance/v1/zones/xx-yyy-9/servers", "", http.StatusBadRequest, "invalid_arguments"},
		{"missing name", "POST", "/instance/v1/zones/fr-par-1/servers", `{"commercial_type":"DEV1-S"}`, http.StatusBadRequest, "invalid_arguments"},
		{"missing commercial type", "POST", "/instance/v1/zones/fr-par-1/servers", `{"name":"web"}`, http.StatusBadRequest, "invalid_arguments"},
		{"malformed body", "POST", "/instance/v1/zones/fr-par-1/servers", `{`, http.StatusBadRequest, "invalid_arguments"},
		{"unknown server", "GET", "/instance/v1/zones/fr-par-1/servers/nope", "", http.StatusNotFound, "not_found"},
		{"unknown action", "POST", "/instance/v1/zones/fr-par-1/servers/nope/action", `{"action":"selfdestruct"}`, http.StatusNotFound, "not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := do(t, ts, tc.method, tc.path, tc.body)
			if status != tc.wantStatus {
				t.Fatalf("expected %d, got %d (%v)", tc.wantStatus, status, body)
			}
			// The SDK dispatches on this field; a missing or wrong "type" turns a
			// typed error into an opaque one.
			if body["type"] != tc.wantType {
				t.Fatalf("expected error type %q, got %v", tc.wantType, body["type"])
			}
		})
	}
}
