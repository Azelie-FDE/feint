package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stephrobert/feint/internal/cli"
)

// This package had no tests at all, and that is how a working tree that did not
// compile still reported every test green: nothing linked internal/cli, so
// `go test ./...` never built it. Only `go vet` caught it, and a contributor who
// runs the tests is not doing anything unreasonable.
//
// What is pinned here is the contract the CI and the documentation depend on:
// the exit codes, and the fact that every advertised subcommand is reachable.

func run(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = cli.Run(append([]string{"feint"}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

// The exit codes are documented and the CI reads them. A change here breaks
// pipelines silently, which is why it is a test rather than a comment.
func TestExitCodesAreWhatTheDocumentationPromises(t *testing.T) {
	if code, _, _ := run("version"); code != 0 {
		t.Fatalf("version exited %d, want 0", code)
	}
	if code, _, _ := run("no-such-command"); code != 1 {
		t.Fatalf("an unknown command exited %d, want 1", code)
	}
	if code, _, _ := run("coverage"); code != 1 {
		t.Fatalf("coverage without a source exited %d, want 1", code)
	}
}

// Every subcommand the usage text advertises must exist. The usage string is
// read by users; a command listed there and missing from the dispatch is a lie
// nobody notices until somebody types it.
func TestEverySubcommandInTheUsageTextIsReachable(t *testing.T) {
	_, help, _ := run("--help")
	if !strings.Contains(help, "Usage:") {
		t.Fatalf("--help printed no usage: %q", help)
	}

	for _, name := range []string{"serve", "coverage", "probe", "docs", "catalog", "clean", "version"} {
		if !strings.Contains(help, "feint "+name) {
			t.Errorf("the usage text does not mention %q", name)
		}
		// Reached, not run: `serve` would block. An unknown command is the one
		// answer that must not come back.
		_, _, errOut := run(name, "--this-flag-does-not-exist")
		if strings.Contains(errOut, "unknown command") {
			t.Errorf("subcommand %q is advertised but not dispatched", name)
		}
	}
}

// A user who types --version and is told "unknown command" concludes the binary
// is broken, and they are not wrong to.
func TestVersionIsReachableAsAFlagToo(t *testing.T) {
	code, printed, _ := run("version")
	if code != 0 || strings.TrimSpace(printed) == "" {
		t.Fatalf("version printed %q with code %d", printed, code)
	}
	for _, form := range []string{"--version", "-v"} {
		code, alias, _ := run(form)
		if code != 0 {
			t.Fatalf("%s exited %d, want 0", form, code)
		}
		if alias != printed {
			t.Fatalf("%s printed %q, version printed %q: they must agree", form, alias, printed)
		}
	}
}

// docs --check is a gate: it must exit 2 on a stale file, not 1, because 1 is
// "something broke" and CI treats the two differently.
func TestDocsCheckExitsTwoOnAStaleFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "README.md")
	stale := "intro\n\n<!-- coverage:start -->\nwritten by hand, and wrong\n<!-- coverage:end -->\n\noutro\n"
	if err := os.WriteFile(target, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	coverage := repoPath(t, "coverage")
	code, _, errOut := run("docs", "--file", target, "--coverage", coverage, "--check")
	if code != 2 {
		t.Fatalf("docs --check on a stale file exited %d, want 2: %s", code, errOut)
	}

	// And the write path must make the check pass, or the gate would be a wall.
	if code, _, errOut := run("docs", "--file", target, "--coverage", coverage); code != 0 {
		t.Fatalf("docs exited %d: %s", code, errOut)
	}
	if code, _, errOut := run("docs", "--file", target, "--coverage", coverage, "--check"); code != 0 {
		t.Fatalf("docs --check still failed right after a write: %s", errOut)
	}

	written, err := os.ReadFile(target) //nolint:gosec // a path this test just made
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case !strings.HasPrefix(string(written), "intro\n"):
		t.Fatal("the generator rewrote what was outside the markers")
	case !strings.HasSuffix(string(written), "outro\n"):
		t.Fatal("the generator dropped what followed the end marker")
	case strings.Contains(string(written), "written by hand, and wrong"):
		t.Fatal("the stale section survived the regeneration")
	}
}

// A file with no markers must fail loudly rather than write nothing and report
// success, which is how a generated section quietly stops being generated.
func TestDocsRefusesAFileWithNoMarkers(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(target, []byte("no markers here\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run("docs", "--file", target, "--coverage", repoPath(t, "coverage"))
	if code != 1 {
		t.Fatalf("exited %d, want 1", code)
	}
	if !strings.Contains(errOut, "markers") {
		t.Fatalf("the error does not say what is missing: %q", errOut)
	}
}

func repoPath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", name))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// The default listen address is a security property, not a preference.
//
// SECURITY.md tells readers that serve binds loopback by default, and for a
// while it did not: DefaultAddr was ":4599", every interface. This emulator
// accepts every credential without checking one and, with --vm, starts
// containers with the operator's privileges — so the default was offering a
// container executor to whatever network the laptop was on, to a reader who had
// just been told the opposite by the security policy.
func TestTheDefaultAddressIsLoopback(t *testing.T) {
	if !strings.HasPrefix(cli.DefaultAddr, "127.0.0.1:") {
		t.Fatalf("DefaultAddr is %q; an emulator that authenticates nobody must not listen off-host by default",
			cli.DefaultAddr)
	}
}
