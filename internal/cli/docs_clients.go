package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Which client, in which version, proved what.
//
// The README said which clients prove which provider and never in what version.
// For a project whose whole argument is that the upstream moves — 453 SDK
// methods added in twelve months — that is the one column a reader needs: a
// `scw` newer than the last conformance run can reject an answer that passed,
// and nothing on the page would say so.
//
// The versions are not restated here. They are read from the workflow that
// installs them and from the Terraform fixture that pins the provider, so the
// table cannot claim a version the CI does not actually run.

const (
	clientsStartMarker = "<!-- clients:start -->"
	clientsEndMarker   = "<!-- clients:end -->"

	conformanceWorkflow = ".github/workflows/conformance.yml"
	terraformFixture    = "tools/conformance/scaleway/terraform/main.tf"
)

// clientSource ties a client to where its version is pinned and to the pack it
// exercises. Adding a client to the suite means adding a line here; a line whose
// variable the workflow does not set renders as "unpinned", which is visible
// rather than silent.
var clientSources = []struct {
	name     string
	variable string
	provider string
}{
	{"`scw`", "SCW_VERSION", "Scaleway"},
	{"Terraform", "TERRAFORM_VERSION", "Scaleway"},
	{"OpenTofu", "TOFU_VERSION", "Scaleway"},
	{"`oapi-cli`", "OAPI_VERSION", "Outscale"},
	{"`exo`", "EXO_VERSION", "Exoscale"},
}

var (
	versionPattern  = regexp.MustCompile(`(?m)^\s*([A-Z0-9_]+_VERSION):\s*'([^']+)'`)
	providerPattern = regexp.MustCompile(`(?s)scaleway\s*=\s*\{.*?version\s*=\s*"([^"]+)"`)
)

// pinnedVersions reads the versions the conformance workflow installs.
func pinnedVersions(workflow string) (map[string]string, error) {
	body, err := os.ReadFile(workflow) //nolint:gosec // a path this repository owns
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, m := range versionPattern.FindAllStringSubmatch(string(body), -1) {
		out[m[1]] = strings.TrimPrefix(m[2], "v")
	}
	return out, nil
}

// terraformProvider reads the provider constraint the Terraform fixture pins.
// The Terraform version alone proves nothing: what answers the emulator is the
// provider, and it is pinned in a different file from every other client.
func terraformProvider(fixture string) string {
	body, err := os.ReadFile(fixture) //nolint:gosec // a path this repository owns
	if err != nil {
		return ""
	}
	if m := providerPattern.FindStringSubmatch(string(body)); m != nil {
		return m[1]
	}
	return ""
}

func renderClients(workflow, fixture string) (string, error) {
	versions, err := pinnedVersions(workflow)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(docsGenerated)
	b.WriteString("\n\n| Client | Version proven in CI | Emulated provider |\n|---|---|---|\n")
	for _, c := range clientSources {
		version, ok := versions[c.variable]
		if !ok {
			version = "unpinned"
		}
		if c.name == "Terraform" {
			if p := terraformProvider(fixture); p != "" {
				version = fmt.Sprintf("%s with provider `scaleway/scaleway %s`", version, p)
			}
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", c.name, version, c.provider)
	}
	b.WriteString("\nThese are the versions the conformance workflow installs and runs on every\n")
	b.WriteString("pull request and nightly. A newer client is not known to work until the pin\n")
	b.WriteString("moves and the suite passes on it, which is the whole point of pinning it.\n")
	return b.String(), nil
}

// The splice itself lives in docs(), next to the coverage and banner ones: the
// document is already in memory there, and a second reader of the same file
// would be a second chance for the two to disagree.
