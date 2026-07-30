package cli

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/stephrobert/feint/internal/core/emulator"
)

// The verb with the most use per line of code.
//
// `eval "$(feint env scaleway)"` has to be enough to point scw, Terraform or an
// SDK at the emulator. Before it, the README asked a reader to copy seven
// environment variables by hand, and a typo in any of them produced an error
// that says nothing about the typo.
//
// Two details decide whether `eval` works at all, and both are about which
// stream carries what. **Only the export lines go to stdout.** Every comment,
// every hint, every warning goes to stderr — a single stray line of prose on
// stdout and the shell tries to execute it. And **--unset prints the removals**,
// because a developer who pointed a shell at the emulator needs a way back to
// the real cloud without opening a new terminal.
//
// Nothing here names a provider. The variable names are provider knowledge and
// live in the pack, behind Pack.Env; this file iterates whatever packs the
// server mounts. The usual test: could this line be written identically for
// another provider? Every line here can.

func envCommand(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	shell := fs.String("shell", "bash", "shell syntax: bash, fish or powershell")
	endpoint := fs.String("endpoint", "", "the emulator's address (default: http://<the --addr default>)")
	unset := fs.Bool("unset", false, "print the removals instead of the exports")

	// The provider comes first and is taken before parsing, because the standard
	// flag package stops at the first non-flag argument: with `env scaleway
	// --endpoint …`, Parse would leave three positional arguments and no
	// endpoint at all.
	//
	// That is not a cosmetic bug. It made this command print nothing on stdout,
	// an `eval` of nothing succeed silently, and the client that followed fall
	// back to the operator's stored credentials — which created a server and a
	// flexible IP on a real, paying account. A command that produces no output
	// and exits 0 is the shape of that accident, and the check below is what
	// makes it impossible.
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(stderr, "feint: env needs a provider first: feint env <provider> [flags]")
		return exitError
	}
	wanted := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return exitError
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "feint: unexpected argument %q; env takes one provider and flags\n", fs.Arg(0))
		return exitError
	}

	srv, _, err := newServer(nil)
	if err != nil {
		fmt.Fprintf(stderr, "feint: %v\n", err)
		return exitError
	}

	names := make([]string, 0, len(srv.Packs()))
	for _, p := range srv.Packs() {
		names = append(names, p.Name())
	}
	sort.Strings(names)

	var pack packEnv
	for _, p := range srv.Packs() {
		if p.Name() == wanted {
			pack = p
			break
		}
	}
	if pack == nil {
		fmt.Fprintf(stderr, "feint: no provider %q; this emulator serves %s\n", wanted, strings.Join(names, ", "))
		return exitError
	}

	target := *endpoint
	if target == "" {
		target = defaultEndpoint()
	}
	env := pack.Env(target)

	render, ok := renderers[*shell]
	if !ok {
		fmt.Fprintf(stderr, "feint: unknown shell %q; bash, fish or powershell\n", *shell)
		return exitError
	}

	keys := make([]string, 0, len(env.Vars))
	for key := range env.Vars {
		keys = append(keys, key)
	}
	// Sorted, so two runs emit the same bytes and a diff of the output means
	// something changed rather than that a map iterated differently.
	sort.Strings(keys)

	// Empty output is a failure, never a quiet success.
	//
	// A pack that declares no variable would have this command print nothing and
	// exit 0, and the caller's `eval` would then leave the shell pointed at
	// whatever it was pointed at before — which is a real cloud, for anybody who
	// has ever logged in. Refusing is the only safe answer.
	if len(keys) == 0 {
		fmt.Fprintf(stderr, "feint: the %s pack declares no environment; refusing to print nothing, "+
			"because an eval of nothing leaves your shell pointed at the real cloud\n", wanted)
		return exitError
	}

	for _, key := range keys {
		if *unset {
			fmt.Fprintln(stdout, render.unset(key))
			continue
		}
		fmt.Fprintln(stdout, render.export(key, env.Vars[key]))
	}

	// stderr, always. This is what keeps `eval` safe while still telling the
	// user the thing they need to know.
	if !*unset && env.Note != "" {
		fmt.Fprintf(stderr, "note: %s\n", env.Note)
	}
	return exitOK
}

// packEnv is the slice of a pack this command uses. Declared here rather than
// taking emulator.Pack whole, so it is obvious that env reads one method and
// nothing else.
type packEnv interface {
	Name() string
	Env(endpoint string) emulator.Environment
}

// shellRenderer writes an assignment and its removal in one shell's syntax.
//
// Three shells because the three exist and their syntaxes share nothing:
// `export K=v`, `set -gx K v`, `$env:K = "v"`. A user on fish who is handed bash
// syntax gets a parse error rather than a hint.
type shellRenderer struct {
	export func(key, value string) string
	unset  func(key string) string
}

var renderers = map[string]shellRenderer{
	"bash": {
		// Single-quoted, with embedded quotes escaped the only way bash allows:
		// close the quote, insert an escaped one, reopen. None of the values
		// here contain a quote today, and a renderer that breaks the day one
		// does is a renderer that breaks silently.
		export: func(key, value string) string {
			return "export " + key + "='" + strings.ReplaceAll(value, "'", `'\''`) + "'"
		},
		unset: func(key string) string { return "unset " + key },
	},
	"fish": {
		export: func(key, value string) string {
			return "set -gx " + key + " '" + strings.ReplaceAll(value, "'", `\'`) + "'"
		},
		unset: func(key string) string { return "set -e " + key },
	},
	"powershell": {
		export: func(key, value string) string {
			return "$env:" + key + ` = "` + strings.ReplaceAll(value, `"`, "`\"") + `"`
		},
		unset: func(key string) string { return "Remove-Item Env:\\" + key + " -ErrorAction SilentlyContinue" },
	},
}

// defaultEndpoint is the address `serve` listens on by default, rendered as a
// URL a client can use. A bare ":4599" is a valid listen address and not a valid
// endpoint, so the host is filled in.
func defaultEndpoint() string {
	addr := DefaultAddr
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	return "http://" + addr
}
