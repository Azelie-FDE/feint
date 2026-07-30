package drift

import (
	"fmt"
	"sort"

	"github.com/stephrobert/feint/internal/contract"
)

// A provider that publishes an OpenAPI document needs no SDK reader: the
// document already lists every operation it has, and the extraction under
// contracts/ has already turned it into something Go can read.
//
// One artefact, two mechanisms. The contract answers "is this field real"; the
// same file answers "did the surface move", which is what the baseline compares
// against. Exoscale is the case that made this obvious: it had neither, and its
// five routes declared operation names nothing could check — the exact state
// Outscale was in when a scan found all seven of its names wrong.

// ScanContract returns the upstream surface a contract describes.
//
// Operations are named the way a route must declare them: the provider key, the
// document's path prefix, then the operationId the document itself gives. That
// last part is not translated. Exoscale's identifiers are kebab-case
// (list-instances) and their Go SDK renames them; picking the renamed form would
// mean this project performing a translation nobody could check, which is how
// hand-written names go wrong.
func ScanContract(doc *contract.Doc) ([]Operation, error) {
	if len(doc.Operations) == 0 {
		return nil, fmt.Errorf("contract for %s lists no operation", doc.Provider)
	}

	product := doc.Provider + doc.PathPrefix
	ops := make([]Operation, 0, len(doc.Operations))
	for name := range doc.Operations {
		ops = append(ops, Operation{
			Name:    product + "." + name,
			Product: doc.Provider,
			Version: doc.APIVersion,
		})
	}

	sort.Slice(ops, func(i, j int) bool { return ops[i].Name < ops[j].Name })
	return ops, nil
}
