package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// The startup banner, in the same ANSI Shadow style as the sibling projects.
//
// Three rules decide everything about it, and each one is a lesson rather than a
// preference.
//
// **It goes to stderr, always.** Never stdout. `eval "$(feint env scaleway)"`
// consumes stdout, and a single decorative line there is a line the shell tries
// to execute. This project already produced a real incident from stdout being
// empty when it should not have been; polluting it would be the mirror image.
//
// **It prints only to a terminal.** Piped, redirected or in CI, stderr is not a
// character device and the banner is silently skipped: six lines of block
// characters in a CI log are noise nobody asked for, and a `feint wait` in a
// loop would emit them on every iteration.
//
// **It is drawn, not stored.** The glyphs are a table and the wordmark is
// composed at runtime, which is what makes it survive a rename: change the word,
// not six lines of art. That is not hypothetical here — the project is under a
// pending rename, and a hand-drawn banner would be six more lines to redo.
//
// No dependency. The sibling projects reach for lipgloss; this project stays at
// zero external dependencies until a real need forces one, and a colour is an
// escape sequence.

// glyphs are ANSI Shadow letters, six rows each. Only the letters this project's
// name needs, because an unused glyph is a glyph nobody checks.
//
// A letter that is missing renders as a blank column rather than panicking: a
// banner is decoration, and decoration must never take the process down.
var glyphs = map[rune][6]string{
	'C': {" ██████╗", "██╔════╝", "██║     ", "██║     ", "╚██████╗", " ╚═════╝"},
	'O': {" ██████╗ ", "██╔═══██╗", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'U': {"██╗   ██╗", "██║   ██║", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'F': {"███████╗", "██╔════╝", "█████╗  ", "██╔══╝  ", "██║     ", "╚═╝     "},
	'E': {"███████╗", "██╔════╝", "█████╗  ", "██╔══╝  ", "███████╗", "╚══════╝"},
	'I': {"██╗", "██║", "██║", "██║", "██║", "╚═╝"},
	'N': {"███╗   ██╗", "████╗  ██║", "██╔██╗ ██║", "██║╚██╗██║", "██║ ╚████║", "╚═╝  ╚═══╝"},
	'T': {"████████╗", "╚══██╔══╝", "   ██║   ", "   ██║   ", "   ██║   ", "   ╚═╝   "},
}

// wordmark composes a word from the glyph table.
func wordmark(word string) []string {
	rows := make([]string, 6)
	for _, r := range strings.ToUpper(word) {
		glyph, known := glyphs[r]
		if !known {
			// A blank column of the same height keeps the block rectangular.
			glyph = [6]string{"   ", "   ", "   ", "   ", "   ", "   "}
		}
		for i := range rows {
			rows[i] += glyph[i] + " "
		}
	}
	return rows
}

// ANSI escapes, written out rather than pulled from a library. The colour is a
// 256-palette teal — deliberately distant from the trade dress of every provider
// this emulator imitates, and legible on both a light and a dark terminal.
const (
	ansiReset = "\x1b[0m"
	ansiBrand = "\x1b[1;38;5;36m"
	ansiMuted = "\x1b[2m"
)

// banner writes the startup banner, or nothing at all.
//
// Silence is the common case and it is deliberate: a tool that decorates a pipe
// is a tool people work around.
func banner(w io.Writer, version string) {
	if !bannerWanted(w) {
		return
	}

	brand, muted, reset := ansiBrand, ansiMuted, ansiReset
	if os.Getenv("NO_COLOR") != "" {
		// no-color.org: the presence of the variable is the signal, whatever its
		// value. The art still prints; only the escapes go.
		brand, muted, reset = "", "", ""
	}

	fmt.Fprintln(w)
	for _, row := range wordmark("feint") {
		fmt.Fprintf(w, " %s%s%s\n", brand, row, reset)
	}
	// Two lines, and the order matters. The first states what the project is
	// for; the second names who is served today. Saying "Scaleway, Outscale and
	// Exoscale" alone reads as a tool about three clouds, when the point is that
	// every European cloud needs one of these and three happen to be first.
	fmt.Fprintf(w, "\n %sEvery European cloud needs an emulator%s\n", brand, reset)
	fmt.Fprintf(w, " %sScaleway, Outscale and Exoscale first  ·  %s%s\n\n",
		muted, shortVersion(version), reset)
}

// bannerWanted decides whether anything should be printed.
//
// FEINT_NO_BANNER is checked first so an operator who finds it noisy can
// silence it once and for all, including in an interactive shell.
func bannerWanted(w io.Writer) bool {
	if os.Getenv("FEINT_NO_BANNER") != "" {
		return false
	}
	file, isFile := w.(*os.File)
	if !isFile {
		// A buffer, which is what the tests pass. Never decorate something a
		// test is asserting on.
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	// A character device is a terminal. A pipe, a file and /dev/null are not,
	// and none of them wants six lines of block characters.
	return info.Mode()&os.ModeCharDevice != 0
}

// shortVersion renders a version for the banner, which has a line to fill rather
// than a value to report.
//
// A binary built from a checkout carries a Go pseudo-version —
// v0.0.0-20260729100614-d0f54fee2a77+dirty — which is correct, unreadable, and
// forty characters of noise next to a six-line wordmark. A release tag is short
// already and passes through untouched.
//
// `feint version` still prints the full string: that is the value a bug report
// needs, and this is decoration.
func shortVersion(version string) string {
	const pseudoPrefix = "v0.0.0-"
	if !strings.HasPrefix(version, pseudoPrefix) {
		return version
	}
	parts := strings.Split(strings.TrimSuffix(version, "+dirty"), "-")
	if len(parts) < 3 {
		return version
	}
	short := "dev " + parts[len(parts)-1]
	if strings.HasSuffix(version, "+dirty") {
		// Worth saying: a dirty build is not the commit it names.
		short += " (modified)"
	}
	return short
}
