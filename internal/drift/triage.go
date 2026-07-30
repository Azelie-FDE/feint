package drift

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Triage answers the question the coverage report leaves open: of everything
// nobody has decided on, what is worth deciding first?
//
// A flat list of a hundred operation names does not answer it. Grouped by API
// and by subject, it does, and the shape of the backlog becomes obvious: a whole
// alpha version of a product nobody will serve, an API served on a different
// address, a resource whose five operations arrived together upstream.
//
// This exists as a command rather than as a throwaway script because the
// question comes back at every SDK bump, and because the answer has to be
// reproducible by whoever triages the weekly drift pull request.

// TriageGroup is one subject with the operations left undecided on it.
type TriageGroup struct {
	// API is the qualified interface the operations belong to, such as
	// "instance/v1/API". Two APIs of one product are two different surfaces:
	// MetadataAPI is served to the machines, not to the client.
	API string `json:"api"`
	// Subject is the resource the operations act on, derived from the verb:
	// ListSnapshots, CreateSnapshot and DeleteSnapshot share "Snapshot".
	Subject string `json:"subject"`
	// Operations are the undecided names, sorted.
	Operations []string `json:"operations"`
}

// verbs are the prefixes an operation name starts with. Stripping one leaves
// the subject, which is what a triage decision is actually about: nobody decides
// on GetSnapshot alone, they decide on snapshots.
//
// Two clouds, two vocabularies for the same acts: Scaleway gets and attaches,
// Outscale reads and links. Both lists live here because the subject is what
// the grouping is for, and a verb nobody stripped leaves a resource split
// across as many groups as there are ways to say the same thing.
//
// Order matters where one verb prefixes another: ListAll before List, ScaleUp
// before Scale. The first match wins.
var verbs = []string{
	"ListAll", "List", "GetAll", "Get", "Read", "Create", "Update", "Delete", "Set",
	"Attach", "Detach", "Unlink", "Link", "Add", "Remove", "Apply", "Check",
	"Export", "Import", "Plan", "Migrate", "Release", "Try", "Enable", "Disable",
	"Start", "Stop", "Reboot", "Pause", "Server",
	"Accept", "Reject", "Put", "Deregister", "Register", "ScaleUp", "ScaleDown",
}

// Triage groups every undecided operation of a report.
//
// Groups are ordered by size, largest first, because a subject with five
// undecided operations is one decision and a singleton is one too: sorting by
// weight puts the cheap wins on top. Ties break on name so two runs agree.
func (r Report) Triage() []TriageGroup {
	type key struct{ api, subject string }
	groups := map[key][]string{}

	for _, entry := range r.Entries {
		if entry.Status != StatusUnknown {
			continue
		}
		api, name := splitOperation(entry.Operation)
		k := key{api: api, subject: SubjectOf(name)}
		groups[k] = append(groups[k], name)
	}

	out := make([]TriageGroup, 0, len(groups))
	for k, ops := range groups {
		sort.Strings(ops)
		out = append(out, TriageGroup{API: k.api, Subject: k.subject, Operations: ops})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Operations) != len(out[j].Operations) {
			return len(out[i].Operations) > len(out[j].Operations)
		}
		if out[i].API != out[j].API {
			return out[i].API < out[j].API
		}
		return out[i].Subject < out[j].Subject
	})
	return out
}

// WriteTriage renders the groups, then the per-API totals.
//
// The totals matter as much as the groups: they are what reveals that most of a
// backlog belongs to one alpha API, which is a single decision rather than
// sixty.
func (r Report) WriteTriage(w io.Writer) error {
	groups := r.Triage()
	if len(groups) == 0 {
		_, err := fmt.Fprintf(w, "provider %s: nothing left to triage\n", r.Provider)
		return err
	}

	if _, err := fmt.Fprintf(w, "provider %s: %d operations to triage\n\n", r.Provider, r.Unknown); err != nil {
		return err
	}
	for _, group := range groups {
		ops := strings.Join(group.Operations, ", ")
		if len(ops) > 96 {
			ops = ops[:93] + "..."
		}
		if _, err := fmt.Fprintf(w, "  %-24s %-26s %3d  %s\n",
			group.API, group.Subject, len(group.Operations), ops); err != nil {
			return err
		}
	}

	perAPI := map[string]int{}
	for _, group := range groups {
		perAPI[group.API] += len(group.Operations)
	}
	names := make([]string, 0, len(perAPI))
	for api := range perAPI {
		names = append(names, api)
	}
	sort.Slice(names, func(i, j int) bool {
		if perAPI[names[i]] != perAPI[names[j]] {
			return perAPI[names[i]] > perAPI[names[j]]
		}
		return names[i] < names[j]
	})

	if _, err := fmt.Fprintln(w, "\nby API:"); err != nil {
		return err
	}
	for _, api := range names {
		if _, err := fmt.Fprintf(w, "  %4d  %s\n", perAPI[api], api); err != nil {
			return err
		}
	}
	return nil
}

// splitOperation cuts "instance/v1/API.ListServers" into its interface and its
// method. An operation without a dot keeps its whole name as the method, which
// is how a malformed entry stays visible instead of vanishing into a group.
func splitOperation(operation string) (api, method string) {
	if i := strings.LastIndex(operation, "."); i >= 0 {
		return operation[:i], operation[i+1:]
	}
	return "", operation
}

// SubjectOf strips the leading verb and the plural, so a resource is one
// subject whichever verb acts on it.
//
// It is exported because the README generator groups served operations by
// the same subject the triage groups undecided ones by. Two extractors would
// answer differently the day one of them learned a verb, and the backlog and the
// documentation would then disagree about what a component is.
func SubjectOf(method string) string {
	for _, verb := range verbs {
		if len(method) > len(verb) && strings.HasPrefix(method, verb) {
			subject := strings.TrimSuffix(method[len(verb):], "s")
			if subject != "" {
				return subject
			}
		}
	}
	return method
}
