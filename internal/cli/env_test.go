package cli_test

import (
	"strings"
	"testing"
)

// What is pinned here is the accident, not the feature.
//
// `feint env scaleway --endpoint http://…` printed nothing, because Go's flag
// package stops at the first non-flag argument and the provider was read after
// parsing. The command exited 0. A test script then ran `eval "$(…)"` on that
// empty output, which succeeded, and the scw CLI that followed fell back to the
// operator's stored credentials — creating a DEV1-S server and a flexible IP on
// a real, paying account.
//
// Every test below exists so that no single link of that chain can form again.

// The provider must be readable when flags follow it, which is the ordinary way
// anybody types this command.
func TestEnvReadsTheProviderBeforeItsFlags(t *testing.T) {
	code, printed, errOut := run("env", "scaleway", "--endpoint", "http://127.0.0.1:4599")
	if code != 0 {
		t.Fatalf("exited %d: %s", code, errOut)
	}
	if !strings.Contains(printed, "export SCW_API_URL='http://127.0.0.1:4599'") {
		t.Fatalf("the endpoint flag was not applied:\n%s", printed)
	}
}

// Success with empty output is the shape of the accident: an eval of nothing
// leaves the shell pointed wherever it already was, which is a real cloud for
// anybody who has ever logged in.
func TestEnvNeverSucceedsWithoutPrintingAnything(t *testing.T) {
	for _, args := range [][]string{
		{"env"},
		{"env", "--endpoint", "http://127.0.0.1:4599"},
		{"env", "nowhere"},
	} {
		code, printed, _ := run(args...)
		if code == 0 && strings.TrimSpace(printed) == "" {
			t.Fatalf("%v exited 0 and printed nothing on stdout", args)
		}
	}
}

// Only exports reach stdout. One line of prose there and the caller's eval tries
// to execute it.
func TestEnvPutsNothingButExportsOnStdout(t *testing.T) {
	for _, provider := range []string{"scaleway", "outscale", "exoscale"} {
		code, printed, errOut := run("env", provider)
		if code != 0 {
			t.Fatalf("env %s exited %d: %s", provider, code, errOut)
		}
		for _, line := range strings.Split(strings.TrimSpace(printed), "\n") {
			if !strings.HasPrefix(line, "export ") {
				t.Errorf("env %s put a non-export line on stdout: %q", provider, line)
			}
		}
	}
}

// The Exoscale note is the reason the Note field exists: that provider's CLI
// reads no endpoint variable at all, and a user handed variables that do not
// work for their client, without being told, is worse served than one told
// nothing.
func TestEnvSendsItsNoteToStderr(t *testing.T) {
	code, printed, errOut := run("env", "exoscale")
	if code != 0 {
		t.Fatalf("exited %d: %s", code, errOut)
	}
	if !strings.Contains(errOut, "config file") {
		t.Fatalf("the exoscale caveat is not on stderr: %q", errOut)
	}
	if strings.Contains(printed, "config file") {
		t.Fatalf("the caveat leaked onto stdout, where eval would execute it:\n%s", printed)
	}
}

// A shell gets its own syntax or a clear refusal, never another shell's.
func TestEnvRendersEachShellInItsOwnSyntax(t *testing.T) {
	cases := map[string]string{
		"bash":       "export SCW_API_URL='",
		"fish":       "set -gx SCW_API_URL '",
		"powershell": `$env:SCW_API_URL = "`,
	}
	for shell, want := range cases {
		code, printed, errOut := run("env", "scaleway", "--shell", shell)
		if code != 0 {
			t.Fatalf("env --shell %s exited %d: %s", shell, code, errOut)
		}
		if !strings.Contains(printed, want) {
			t.Errorf("--shell %s did not render %q:\n%s", shell, want, printed)
		}
	}
	if code, _, errOut := run("env", "scaleway", "--shell", "csh"); code == 0 {
		t.Errorf("an unknown shell was accepted: %s", errOut)
	}
}

// --unset is the way back to the real cloud in the same shell. Without it, a
// developer who pointed a terminal at the emulator has to open a new one.
func TestEnvUnsetRemovesWhatItSet(t *testing.T) {
	_, exported, _ := run("env", "scaleway")
	code, removed, errOut := run("env", "scaleway", "--unset")
	if code != 0 {
		t.Fatalf("exited %d: %s", code, errOut)
	}
	for _, line := range strings.Split(strings.TrimSpace(exported), "\n") {
		key := strings.TrimPrefix(line, "export ")
		key = key[:strings.Index(key, "=")]
		if !strings.Contains(removed, "unset "+key) {
			t.Errorf("%s is exported but never unset", key)
		}
	}
}

// An unknown provider must name the ones that exist. "no such provider" alone
// sends the reader to the source.
func TestEnvNamesTheProvidersItServes(t *testing.T) {
	code, _, errOut := run("env", "nowhere")
	if code != 1 {
		t.Fatalf("exited %d, want 1", code)
	}
	for _, provider := range []string{"scaleway", "outscale", "exoscale"} {
		if !strings.Contains(errOut, provider) {
			t.Errorf("the error does not mention %s: %q", provider, errOut)
		}
	}
}
