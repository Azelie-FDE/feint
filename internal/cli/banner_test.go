package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The banner's one hard rule: it must never reach anything a machine reads.
//
// The project already spent a real incident on stdout being empty when it should
// not have been — `feint env` printed nothing, an eval of nothing succeeded,
// and the client that followed created billable resources on a real account.
// Decorating stdout would be the same mistake pointed the other way: a line of
// block characters where a shell expects `export`.

func TestBannerNeverWritesToSomethingThatIsNotATerminal(t *testing.T) {
	var buf bytes.Buffer
	banner(&buf, "v1.0.0")
	if buf.Len() != 0 {
		t.Fatalf("the banner wrote %d bytes to a buffer:\n%s", buf.Len(), buf.String())
	}

	// A file is not a terminal either: `feint serve > log 2>&1` must not fill
	// the log with art.
	path := filepath.Join(t.TempDir(), "out")
	file, err := os.Create(path) //nolint:gosec // a path this test just made
	if err != nil {
		t.Fatal(err)
	}
	banner(file, "v1.0.0")
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path) //nolint:gosec // same path
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 0 {
		t.Fatalf("the banner wrote to a regular file:\n%s", written)
	}
}

// The wordmark is composed, not stored, which is what makes it survive a rename.
func TestWordmarkComposesFromTheGlyphTable(t *testing.T) {
	rows := wordmark("feint")
	if len(rows) != 6 {
		t.Fatalf("%d rows, want 6", len(rows))
	}
	width := len([]rune(rows[0]))
	for i, row := range rows {
		if got := len([]rune(row)); got != width {
			t.Fatalf("row %d is %d runes wide, row 0 is %d: the block is not rectangular", i, got, width)
		}
	}

	// An unknown letter must not panic. A banner is decoration and decoration
	// never takes the process down.
	if rows := wordmark("zzz"); len(rows) != 6 {
		t.Fatalf("an unknown letter produced %d rows", len(rows))
	}
}

// The full version is what a bug report needs; the banner has a line to fill.
func TestShortVersionKeepsATagAndShortensAPseudoVersion(t *testing.T) {
	cases := map[string]string{
		"v1.2.3":                             "v1.2.3",
		"dev":                                "dev",
		"v0.0.0-20260729100614-d0f54fee2a77": "dev d0f54fee2a77",
		"v0.0.0-20260729100614-d0f54fee2a77+dirty": "dev d0f54fee2a77 (modified)",
	}
	for in, want := range cases {
		if got := shortVersion(in); got != want {
			t.Errorf("shortVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// An operator who finds it noisy must be able to silence it for good.
func TestBannerCanBeSilenced(t *testing.T) {
	t.Setenv("FEINT_NO_BANNER", "1")
	if bannerWanted(os.Stderr) {
		t.Fatal("FEINT_NO_BANNER did not silence the banner")
	}
}

// NO_COLOR removes the escapes and keeps the art, which is what no-color.org
// asks for: it is a colour preference, not a quiet flag.
func TestNoColorKeepsTheArtAndDropsTheEscapes(t *testing.T) {
	rows := wordmark("feint")
	if strings.Contains(rows[0], "\x1b") {
		t.Fatal("the glyph table itself carries escapes; colour must stay in the writer")
	}
}
