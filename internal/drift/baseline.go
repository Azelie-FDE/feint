package drift

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Baseline records which upstream operations were known-but-unserved at a point
// in time.
//
// A plain "fail when anything is unknown" gate is useless here: Scaleway ships
// 1700 operations and feint will never serve them all. What must fail CI is a
// NEW unknown appearing, because that is an upstream addition nobody has looked
// at yet. Committing the baseline turns every SDK bump into a reviewable diff.
type Baseline struct {
	Provider string   `json:"provider"`
	Products []string `json:"products,omitempty"`
	Unknown  []string `json:"unknown"`
}

// NewBaseline captures the unknown operations of a report.
func NewBaseline(rep Report, products []string) Baseline {
	unknown := make([]string, 0, rep.Unknown)
	for _, e := range rep.Entries {
		if e.Status == StatusUnknown {
			unknown = append(unknown, e.Operation)
		}
	}
	sort.Strings(unknown)
	sort.Strings(products)
	return Baseline{Provider: rep.Provider, Products: products, Unknown: unknown}
}

// LoadBaseline reads a baseline file.
func LoadBaseline(r io.Reader) (Baseline, error) {
	var b Baseline
	if err := json.NewDecoder(r).Decode(&b); err != nil {
		return Baseline{}, fmt.Errorf("read baseline: %w", err)
	}
	return b, nil
}

// Write serializes the baseline, sorted, so the committed file has a stable diff.
func (b Baseline) Write(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(b)
}

// BaselineDiff is what changed upstream since the baseline was taken.
type BaselineDiff struct {
	// Added are operations that appeared upstream and nobody has triaged.
	Added []string
	// Removed are operations that vanished upstream, or that someone has since
	// implemented or declined. Both cases mean the baseline should be refreshed.
	Removed []string
}

// Empty reports whether upstream and the baseline agree.
func (d BaselineDiff) Empty() bool { return len(d.Added) == 0 && len(d.Removed) == 0 }

// Compare reports the difference between a fresh report and the baseline.
func (b Baseline) Compare(rep Report) BaselineDiff {
	current := NewBaseline(rep, b.Products)

	known := toSet(b.Unknown)
	fresh := toSet(current.Unknown)

	var d BaselineDiff
	for _, op := range current.Unknown {
		if !known[op] {
			d.Added = append(d.Added, op)
		}
	}
	for _, op := range b.Unknown {
		if !fresh[op] {
			d.Removed = append(d.Removed, op)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	return d
}

// WriteText renders the diff for a CI log.
func (d BaselineDiff) WriteText(w io.Writer) error {
	if d.Empty() {
		_, err := fmt.Fprintln(w, "no drift: upstream matches the baseline")
		return err
	}
	if len(d.Added) > 0 {
		if _, err := fmt.Fprintf(w, "%d new upstream operation(s), none triaged:\n  %s\n",
			len(d.Added), strings.Join(d.Added, "\n  ")); err != nil {
			return err
		}
	}
	if len(d.Removed) > 0 {
		if _, err := fmt.Fprintf(w, "%d baseline entry(ies) no longer unknown (removed upstream, or now served):\n  %s\n",
			len(d.Removed), strings.Join(d.Removed, "\n  ")); err != nil {
			return err
		}
	}
	return nil
}
