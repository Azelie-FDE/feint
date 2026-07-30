// Package probe drives every mounted route from the provider's own API
// description, and checks what comes back against it.
//
// It exists because writing one conformance case per operation does not scale.
// The suites under tools/conformance/ drive the real clients and assert
// behaviour — the address answers, the firewall filters, a second Terraform plan
// is empty — and each of their assertions is written by hand. There are around
// 150 operations still to serve across three providers, so the protocol half of
// that work has to come from somewhere else.
//
// The contract already holds it: for each operation, a path, a method, a request
// schema and a response schema. That is enough to build a valid call and to
// judge the answer.
//
// # What this proves, and what it does not
//
// A probe proves the protocol: the route answers, and its answer matches the
// schema the provider publishes. It proves nothing about behaviour. An emulator
// that returned a well-shaped empty object for everything would pass every
// probe, and that is precisely the failure of the emulators this project
// measures itself against.
//
// So the two are counted apart and never added together. The emulator marks
// synthetic requests with emulator.ProbeHeader, keeps a separate counter, and
// computes the backlog of unproven routes from real client calls alone. Probing
// a route never takes it off that list. The probe is a check that fails, not a
// score that rises.
//
// # Where it refuses to guess
//
// An identifier is never invented. Values come from what earlier calls in the
// plan returned, and an operation needing one that nothing produces makes the
// planning fail with a message naming it. Inventing an id would be an invented
// format dressed up as automation, which is the one thing this project never
// does: a shape comes from the provider's own SDK or not at all.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/stephrobert/feint/internal/contract"
	"github.com/stephrobert/feint/internal/core/emulator"
)

// Runner drives one provider's routes against a running emulator.
type Runner struct {
	// Doc is the provider's contract.
	Doc *contract.Doc
	// Base is the emulator's address, e.g. "http://127.0.0.1:4599".
	Base string
	// Client is the HTTP client to use. Nil means a default with a short
	// timeout: every call here is local and answers immediately or is stuck.
	Client *http.Client
}

// Result is what one probed operation produced.
type Result struct {
	// Operation is the route's declared operation name.
	Operation string
	// Status is the HTTP status the emulator answered.
	Status int
	// Refused says the emulator declined the synthetic call with a 4xx. Not a
	// failure: the emulator is entitled to be stricter than the schema, and
	// often is on purpose — it refuses to generate a keypair rather than hand
	// out a private key that only looks usable. What matters is that it refused
	// in its own error shape rather than crashing or answering nonsense.
	Refused bool
	// Skipped says why the operation was not called, empty when it was.
	Skipped string
	// Violations are the ways the response disagreed with the contract.
	Violations contract.Violations
	// Err is a transport or decoding failure.
	Err error
}

// Report is what a run produced.
type Report struct {
	Provider string
	Results  []Result
}

// Probed counts the operations that were called and answered a valid response.
func (r Report) Probed() int {
	n := 0
	for _, res := range r.Results {
		if res.Skipped == "" && !res.Refused && res.Err == nil &&
			len(res.Violations) == 0 && res.Status < 300 {
			n++
		}
	}
	return n
}

// Failures are the results a run must fail on: a violated contract, a transport
// error, or a status the operation should not have answered.
//
// A skip is not a failure. Refusing to call an operation whose identifiers
// nothing produces is the honest outcome, and it is reported as such.
func (r Report) Failures() []Result {
	var out []Result
	for _, res := range r.Results {
		if res.Skipped != "" || res.Refused {
			continue
		}
		if res.Err != nil || len(res.Violations) > 0 || res.Status >= 300 {
			out = append(out, res)
		}
	}
	return out
}

// Proven counts the operations that were actually called and answered.
//
// It exists because Failures() alone cannot fail on the one regression that
// matters most to a probe: a change that makes every synthetic body rejected
// turns every result into a refusal, Failures() returns nothing, and the run is
// green with nothing measured. "0 probed, 0 failed" and "34 probed, 0 failed"
// printed the same verdict.
func (r Report) Proven() int {
	count := 0
	for _, res := range r.Results {
		if res.Skipped == "" && !res.Refused && res.Err == nil && res.Status < 300 {
			count++
		}
	}
	return count
}

// Barren reports a run that proved nothing at all despite having work to do.
//
// A provider whose every route is skipped for want of identifiers is not barren:
// nothing was attempted, and the plan says so. A provider whose every attempt
// came back refused is — something answers, and it answers no to everything,
// which is a broken emulator or a broken probe and never a healthy run.
func (r Report) Barren() bool {
	attempted := 0
	for _, res := range r.Results {
		if res.Skipped == "" {
			attempted++
		}
	}
	return attempted > 0 && r.Proven() == 0
}

// Run executes the plan and returns what each step produced.
func (r *Runner) Run(ctx context.Context, routes []contract.MountedRoute) (Report, error) {
	plan, err := Plan(r.Doc, routes)
	if err != nil {
		return Report{}, err
	}

	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	pool := newPool()
	report := Report{Provider: r.Doc.Provider}
	for _, step := range plan {
		report.Results = append(report.Results, r.call(ctx, client, step, pool))
	}
	return report, nil
}

func (r *Runner) call(ctx context.Context, client *http.Client, step Step, pool *pool) Result {
	result := Result{Operation: step.Operation}
	if step.Skip != "" {
		result.Skipped = step.Skip
		return result
	}

	body, err := minimalBody(r.Doc, step.Request, pool)
	if err != nil {
		// Not a failure: the contract does not let us build this call honestly,
		// and inventing the missing value is the one thing this must not do.
		result.Skipped = err.Error()
		return result
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		result.Err = fmt.Errorf("encode body: %w", err)
		return result
	}

	path, err := pool.fill(step.Path)
	if err != nil {
		result.Skipped = err.Error()
		return result
	}
	url := strings.TrimRight(r.Base, "/") + r.Doc.PathPrefix + path
	req, err := http.NewRequestWithContext(ctx, step.Method, url, bytes.NewReader(encoded))
	if err != nil {
		result.Err = err
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	// What keeps this out of the number that counts.
	req.Header.Set(emulator.ProbeHeader, "1")

	res, err := client.Do(req)
	if err != nil {
		result.Err = err
		return result
	}
	defer func() { _ = res.Body.Close() }()

	result.Status = res.StatusCode
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		result.Err = fmt.Errorf("read body: %w", err)
		return result
	}
	if res.StatusCode >= 400 && res.StatusCode < 500 {
		result.Refused = true
		return result
	}
	if res.StatusCode >= 300 || len(raw) == 0 {
		return result
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		result.Err = fmt.Errorf("response is not JSON: %w", err)
		return result
	}
	result.Violations = r.Doc.ValidateResponse(step.Contract, decoded)

	// Whatever came back feeds the calls that follow: this is how a create finds
	// the identifier a delete needs, without a scenario written by hand.
	pool.harvest(decoded)
	return result
}

// String renders a report the way the conformance scripts print theirs.
func (r Report) String() string {
	lines := make([]string, 0, len(r.Results)+1)
	probed, skipped, refused := 0, 0, 0
	for _, res := range r.Results {
		switch {
		case res.Skipped != "":
			skipped++
		case res.Refused:
			refused++
		case res.Err != nil:
			lines = append(lines, "  FAIL "+res.Operation+": "+res.Err.Error())
		case len(res.Violations) > 0:
			lines = append(lines, "  FAIL "+res.Operation+": "+res.Violations.Error())
		case res.Status >= 300:
			lines = append(lines, fmt.Sprintf("  FAIL %s: answered %d", res.Operation, res.Status))
		default:
			probed++
		}
	}
	sort.Strings(lines)
	head := fmt.Sprintf("probe %s: %d probed, %d refused, %d skipped, %d failed",
		r.Provider, probed, refused, skipped, len(lines))
	return strings.Join(append([]string{head}, lines...), "\n")
}
