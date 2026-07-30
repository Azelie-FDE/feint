package scaleway_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// raw performs a request and returns the status and the body untouched: user
// data is transferred as a bare body, so a JSON-decoding helper cannot see what
// this test is about.
func raw(t *testing.T, ts *httptest.Server, method, path, body string) (int, string) {
	t.Helper()

	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(out)
}

// A user data value round-trips as the exact bytes the client sent. Wrapping it
// in a JSON envelope is the mistake that hands a client a quoted string it never
// wrote, and a boot script that no longer parses.
func TestUserDataRoundTripsRaw(t *testing.T) {
	ts := newTestServer(t)
	id, _ := serverWith(t, ts, `{"name":"ud","commercial_type":"DEV1-S"}`)

	const script = "#cloud-config\nruncmd:\n  - [ echo, \"hello\" ]\n"
	status, _ := raw(t, ts, "PATCH", zoneURL+"/servers/"+id+"/user_data/cloud-init", script)
	if status != http.StatusNoContent {
		t.Fatalf("set user data: expected 204, got %d", status)
	}

	status, got := raw(t, ts, "GET", zoneURL+"/servers/"+id+"/user_data/cloud-init", "")
	if status != http.StatusOK {
		t.Fatalf("get user data: expected 200, got %d", status)
	}
	if got != script {
		t.Errorf("user data came back altered:\n got %q\nwant %q", got, script)
	}
}

func TestUserDataListingAndDeletion(t *testing.T) {
	ts := newTestServer(t)
	id, _ := serverWith(t, ts, `{"name":"ud","commercial_type":"DEV1-S"}`)
	base := zoneURL + "/servers/" + id + "/user_data"

	// The provider reads the whole set right after a create: an empty set is a
	// normal answer, not a 404.
	status, listed := do(t, ts, "GET", base, "")
	if status != http.StatusOK {
		t.Fatalf("list user data on a fresh server: expected 200, got %d", status)
	}
	if keys, _ := listed["user_data"].([]any); len(keys) != 0 {
		t.Errorf("expected no keys, got %v", keys)
	}

	for _, key := range []string{"cloud-init", "another"} {
		if status, _ := raw(t, ts, "PATCH", base+"/"+key, "value of "+key); status != http.StatusNoContent {
			t.Fatalf("set %s: expected 204, got %d", key, status)
		}
	}
	_, listed = do(t, ts, "GET", base, "")
	keys, _ := listed["user_data"].([]any)
	// Sorted: an order that changed between reads would show as a diff.
	if len(keys) != 2 || keys[0] != "another" || keys[1] != "cloud-init" {
		t.Errorf("listing is not the sorted key set: %v", keys)
	}

	if status, _ := raw(t, ts, "DELETE", base+"/another", ""); status != http.StatusNoContent {
		t.Errorf("delete user data: expected 204, got %d", status)
	}
	if status, _ := raw(t, ts, "GET", base+"/another", ""); status != http.StatusNotFound {
		t.Errorf("get a deleted key: expected 404, got %d", status)
	}
	// A key that was never set is not found either, rather than an empty 200.
	if status, _ := raw(t, ts, "GET", base+"/never-set", ""); status != http.StatusNotFound {
		t.Errorf("get an unknown key: expected 404, got %d", status)
	}
}

// User data belongs to its server: reading it through another one must not
// surface it.
func TestUserDataIsScopedToItsServer(t *testing.T) {
	ts := newTestServer(t)
	first, _ := serverWith(t, ts, `{"name":"first","commercial_type":"DEV1-S"}`)
	second, _ := serverWith(t, ts, `{"name":"second","commercial_type":"DEV1-S"}`)

	if status, _ := raw(t, ts, "PATCH", zoneURL+"/servers/"+first+"/user_data/cloud-init", "secret"); status != http.StatusNoContent {
		t.Fatalf("set user data: got %d", status)
	}
	if status, _ := raw(t, ts, "GET", zoneURL+"/servers/"+second+"/user_data/cloud-init", ""); status != http.StatusNotFound {
		t.Errorf("another server's user data is readable: got %d", status)
	}
}
