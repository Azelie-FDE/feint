package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeReleaseWorkflow = `
    strategy:
      matrix:
        include:
          - { goos: linux, goarch: amd64 }
          - { goos: darwin, goarch: arm64 }
      steps:
      - run: sha256sum feint-* > checksums.txt
      - run: cosign sign-blob --yes --bundle checksums.txt.cosign.bundle checksums.txt
`

// An install command naming an asset the release does not build is a 404 on the
// first command a reader runs, and nothing else in this repository would fail.
func TestDocumentedAssetsMustBeBuiltByTheRelease(t *testing.T) {
	dir := t.TempDir()
	workflow := filepath.Join(dir, "release.yml")
	if err := os.WriteFile(workflow, []byte(fakeReleaseWorkflow), 0o600); err != nil {
		t.Fatal(err)
	}

	good := filepath.Join(dir, "good.md")
	if err := os.WriteFile(good, []byte(
		"base=https://github.com/x/y/releases/download/v1.2.3\n"+
			"curl -O \"$base/feint-linux-amd64\"\n"+
			"curl -O \"$base/checksums.txt\"\n"+
			"curl -O \"$base/checksums.txt.cosign.bundle\"\n"+
			"gh release download v1.2.3 --repo x/y --pattern 'feint-darwin-arm64'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := documentedAssetMismatches(workflow, good); len(got) != 0 {
		t.Fatalf("assets the workflow does build were reported: %v", got)
	}

	// A platform nobody builds. This is the shape of the mistake: a reader on
	// Windows follows the page and gets a 404 from GitHub.
	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(bad, []byte(
		"curl -O https://github.com/x/y/releases/download/v1.2.3/feint-windows-amd64\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := documentedAssetMismatches(workflow, bad)
	if len(got) != 1 || !strings.Contains(got[0], "feint-windows-amd64") {
		t.Fatalf("want the windows asset reported, got %v", got)
	}

	// And a non-binary artefact the workflow stopped producing.
	renamed := filepath.Join(dir, "renamed.md")
	if err := os.WriteFile(renamed, []byte(
		"curl -O https://github.com/x/y/releases/download/v1.2.3/SHA256SUMS\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := documentedAssetMismatches(workflow, renamed); len(got) != 1 {
		t.Fatalf("want the renamed checksum file reported, got %v", got)
	}

	// The reference itself, whatever it points at. `latest` is where this
	// started, and a page that fetches whatever is newest cannot be checked
	// against a hash: the reader ends up holding a binary neither of you can
	// name, adopted the day it was published.
	mutable := filepath.Join(dir, "mutable.md")
	if err := os.WriteFile(mutable, []byte(
		"curl -O https://github.com/x/y/releases/latest/download/feint-linux-amd64\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got = documentedAssetMismatches(workflow, mutable)
	if len(got) != 1 || !strings.Contains(got[0], "mutable reference") {
		t.Fatalf("want the mutable reference reported, got %v", got)
	}

	// Absent files are silence, not failure: the binary runs outside a checkout.
	if got := documentedAssetMismatches(filepath.Join(dir, "absent"), good); len(got) != 0 {
		t.Fatalf("a missing workflow was reported: %v", got)
	}
	if got := documentedAssetMismatches(workflow, filepath.Join(dir, "absent")); len(got) != 0 {
		t.Fatalf("a missing document was reported: %v", got)
	}
}

// Every published binary must be selectable by the documented command. Naming
// one and leaving an arm64 reader to substitute is how somebody downloads an
// amd64 binary and meets an exec error that names neither the file nor why.
func TestEveryBuiltPlatformIsSelectable(t *testing.T) {
	block := renderPlatformSelectorFixture(t)
	for _, asset := range []string{
		"feint-linux-amd64", "feint-linux-arm64",
		"feint-darwin-amd64", "feint-darwin-arm64",
	} {
		if !strings.Contains(block, "asset="+asset) {
			t.Errorf("no case selects %s", asset)
		}
	}
	// And the refusal: an unpublished platform must not fall through to a
	// default, which would install the wrong architecture.
	if !strings.Contains(block, "no published binary for") {
		t.Error("the selector has no refusal branch")
	}
}

// A platform the release builds and unameOf does not map is published and not
// installable. It must be named rather than dropped.
func TestAnUnmappedPlatformIsNamedRatherThanDropped(t *testing.T) {
	dir := t.TempDir()
	workflow := filepath.Join(dir, "release.yml")
	if err := os.WriteFile(workflow, []byte(fakeReleaseWorkflow+
		"          - { goos: freebsd, goarch: riscv64 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := renderInstall(workflow, filepath.Join("..", "..", "go.mod"), filepath.Join("..", "..", "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "feint-freebsd-riscv64") {
		t.Fatalf("the unmapped platform is missing from the page entirely:\n%s", got)
	}
	if strings.Contains(got, "asset=feint-freebsd-riscv64") {
		t.Fatal("an unmapped platform was given a uname case, which cannot be right")
	}
}

func renderPlatformSelectorFixture(t *testing.T) string {
	t.Helper()
	root := ".." + string(filepath.Separator) + ".."
	// The repository's own release matrix and module, which is what the pages
	// actually carry.
	got, err := renderInstall(filepath.Join(root, releaseWorkflow), filepath.Join(root, "go.mod"), filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// Built is not exercised, and the page must not let a reader hear one for the
// other. Four binaries ship; the tests run on one platform.
func TestExercisedPlatformsComeFromTheRunners(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.yml")
	if err := os.WriteFile(path, []byte("jobs:\n  a:\n    runs-on: ubuntu-24.04\n  b:\n    runs-on: macos-14\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := exercisedPlatforms(path)
	if len(got) != 2 || got[0] != "darwin-arm64" || got[1] != "linux-amd64" {
		t.Fatalf("want the two runners' platforms, got %v", got)
	}

	// A runner nobody mapped is named as unknown rather than silently dropped:
	// a platform quietly missing from this list would understate what is
	// tested, and one quietly guessed would overstate it.
	unknown := filepath.Join(dir, "unknown.yml")
	if err := os.WriteFile(unknown, []byte("    runs-on: freebsd-15\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := exercisedPlatforms(unknown); len(got) != 1 || !strings.Contains(got[0], "unknown runner freebsd-15") {
		t.Fatalf("want the unmapped runner surfaced, got %v", got)
	}
}

// Unconditionally exercised is one platform, and the three others are a promise
// until the repository is public. A table that calls all four "exercised" is the
// half-truth this project exists to avoid, so the two sets stay separate.
func TestThisRepositorySeparatesExercisedFromPromised(t *testing.T) {
	root := ".." + string(filepath.Separator) + ".."
	workflows := []string{filepath.Join(root, goWorkflow), filepath.Join(root, conformanceWorkflow)}

	conditional := conditionalPlatforms(workflows...)
	want := map[string]bool{"linux-arm64": true, "darwin-arm64": true, "darwin-amd64": true}
	if len(conditional) != len(want) {
		t.Fatalf("the public-repository-only platforms are %v, want the three non-amd64-linux ones", conditional)
	}
	for _, platform := range conditional {
		if !want[platform] {
			t.Fatalf("%s is gated on visibility and should not be", platform)
		}
	}

	// linux-amd64 must not be in that set: it is the one that runs on every
	// pull request today, and demoting it to a promise would be the opposite
	// mistake.
	for _, platform := range conditional {
		if platform == "linux-amd64" {
			t.Fatal("linux-amd64 is reported as public-only; it runs on every pull request")
		}
	}
	if got := exercisedPlatforms(workflows...); len(got) != 4 {
		t.Fatalf("the workflows name %v; the release builds four platforms", got)
	}
}

// The repository's own pages against its own release workflow. This is the
// assertion that would fail the day somebody renames an artefact in the build
// matrix without opening the README.
func TestThisRepositoryDocumentsAssetsItActuallyPublishes(t *testing.T) {
	root := ".." + string(filepath.Separator) + ".."
	got := documentedAssetMismatches(
		filepath.Join(root, releaseWorkflow),
		filepath.Join(root, "README.md"),
		filepath.Join(root, "docs", "install.md"),
	)
	for _, line := range got {
		t.Error(line)
	}
	if len(got) > 0 {
		t.Fatal("the install documentation names assets this repository does not publish")
	}
}
