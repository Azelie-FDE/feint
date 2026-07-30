package emulator_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stephrobert/feint/internal/core/emulator"
)

// A browser cannot forge the Host header, which is what makes this the defence
// against DNS rebinding: a page that resolves its own name to 127.0.0.1 still
// asks for that name, and the emulator refuses it.
func TestRebindingGuardRefusesAForeignHost(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		listen string
		host   string
		want   int
	}{
		// Guarded: the emulator is on the loopback, so only the loopback may be
		// asked for.
		{"127.0.0.1:4599", "127.0.0.1:4599", http.StatusOK},
		{"127.0.0.1:4599", "localhost:4599", http.StatusOK},
		{"127.0.0.1:4599", "app.localhost:4599", http.StatusOK},
		{"127.0.0.1:4599", "[::1]:4599", http.StatusOK},
		{"127.0.0.1:4599", "", http.StatusOK},
		{"127.0.0.1:4599", "evil.example", http.StatusForbidden},
		{"127.0.0.1:4599", "feint.attacker.test:4599", http.StatusForbidden},
		{"localhost:4599", "evil.example", http.StatusForbidden},
		// Not guarded: an operator who binds every interface has asked to be
		// reachable by name, and refusing names would break what they requested.
		{"0.0.0.0:4599", "evil.example", http.StatusOK},
		{":4599", "evil.example", http.StatusOK},
		{"192.168.1.10:4599", "feint.lan", http.StatusOK},
	}

	for _, tc := range cases {
		h := emulator.GuardRebinding(ok, tc.listen)
		req := httptest.NewRequest(http.MethodGet, "http://example/_feint/health", nil)
		req.Host = tc.host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("listening on %q, Host %q: answered %d, want %d",
				tc.listen, tc.host, rec.Code, tc.want)
		}
	}
}

// The refusal has to say what was refused and why, or it reads as a broken
// emulator rather than a guarded one.
func TestRebindingRefusalExplainsItself(t *testing.T) {
	h := emulator.GuardRebinding(http.NotFoundHandler(), "127.0.0.1:4599")
	req := httptest.NewRequest(http.MethodGet, "http://example/_feint/health", nil)
	req.Host = "evil.example"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{"evil.example", "--addr"} {
		if !contains(body, want) {
			t.Errorf("the refusal does not mention %q:\n%s", want, body)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
