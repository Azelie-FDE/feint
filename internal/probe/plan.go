package probe

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stephrobert/feint/internal/contract"
)

// Step is one call the probe will make.
type Step struct {
	// Operation is the route's declared name, for reporting.
	Operation string
	// Contract is the key the contract knows it by.
	Contract string
	Method   string
	Path     string
	// Request is the schema of the body to build, empty for none.
	Request string
	// Skip says why this step will not run, empty when it will.
	Skip string
}

// Plan orders the routes so that what produces an identifier runs before what
// needs one, and what destroys runs last.
//
// The order is derived, not written. Reads come first because they need nothing
// and they are where the emulated inventory is published — a client that creates
// a machine reads the catalogue first, and so does this. Creates follow, and
// deletes come last so the run leaves the store as it found it.
//
// Within a group the order is the contract's own, which is alphabetical, so two
// runs plan the same thing. Nothing here tries to sort creates among themselves
// by dependency: an identifier is resolved by whatever earlier call returned it,
// and when none did, the step says so instead of running.
func Plan(doc *contract.Doc, routes []contract.MountedRoute) ([]Step, error) {
	if doc == nil {
		return nil, fmt.Errorf("no contract to plan from")
	}

	var reads, creates, deletes []Step
	for _, route := range routes {
		op, name, known := doc.OperationFor(route.Operation)
		if !known {
			// The route table check already fails on this. Reported here too
			// rather than skipped in silence, because a probe that quietly does
			// nothing is worse than one that does not run.
			return nil, fmt.Errorf("route %s serves %q, which the contract does not define",
				route.Path, route.Operation)
		}

		step := Step{
			Operation: route.Operation,
			Contract:  name,
			Method:    route.Method,
			Path:      op.Path,
			Request:   op.Request,
		}
		if op.Response == "" {
			// Nothing to validate against, so calling it would prove only that
			// it answered. Said out loud: these are what the extraction reports
			// as operations with no response schema.
			step.Skip = "the contract declares no response schema"
		}

		switch kind(name, route.Method) {
		case kindDelete:
			deletes = append(deletes, step)
		case kindCreate:
			creates = append(creates, step)
		default:
			reads = append(reads, step)
		}
	}

	byName := func(s []Step) { sort.Slice(s, func(i, j int) bool { return s[i].Contract < s[j].Contract }) }
	byName(reads)
	byName(creates)
	byName(deletes)

	// Reads run twice, and the second pass is what makes most of the plan
	// reachable. The first finds an empty account, so nothing is harvested and
	// every read-by-identifier skips; the creates then produce the identifiers,
	// and the same reads become answerable. Measured on Scaleway: 19 operations
	// probed with one pass, 34 with two; on Outscale, 12 against 18.
	plan := make([]Step, 0, len(reads)*2+len(creates)+len(deletes))
	plan = append(plan, reads...)
	plan = append(plan, creates...)
	plan = append(plan, reads...)
	plan = append(plan, deletes...)
	return plan, nil
}

type opKind int

const (
	kindRead opKind = iota
	kindCreate
	kindDelete
)

// kind classifies an operation from its name and method.
//
// The verbs are each provider's own — Scaleway creates and gets, Outscale
// creates and reads, Exoscale uses kebab-case — so all three vocabularies are
// here, the same way internal/drift's triage carries both. The HTTP method
// decides when the name says nothing.
func kind(name, method string) opKind {
	verb := name
	if i := strings.LastIndex(verb, "."); i >= 0 {
		verb = verb[i+1:]
	}
	lower := strings.ToLower(verb)

	switch {
	case strings.HasPrefix(lower, "delete"), strings.HasPrefix(lower, "terminate"),
		strings.HasPrefix(lower, "release"), strings.HasPrefix(lower, "unlink"),
		strings.HasPrefix(lower, "detach"):
		return kindDelete
	case strings.HasPrefix(lower, "create"), strings.HasPrefix(lower, "register"),
		strings.HasPrefix(lower, "link"), strings.HasPrefix(lower, "attach"):
		return kindCreate
	case strings.HasPrefix(lower, "read"), strings.HasPrefix(lower, "list"),
		strings.HasPrefix(lower, "get"):
		return kindRead
	}

	switch method {
	case "DELETE":
		return kindDelete
	case "POST", "PUT", "PATCH":
		return kindCreate
	default:
		return kindRead
	}
}
