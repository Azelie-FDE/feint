package probe

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/stephrobert/feint/internal/contract"
)

// minimalBody builds the smallest request the contract accepts: the required
// fields and nothing else.
//
// Nothing else on purpose. Filling optional fields would mean choosing values
// the document does not constrain — a commercial type, an image name — and those
// live in the emulated catalogue, not in the schema. A probe that guessed them
// would be inventing a format, which no part of this emulator is allowed to do.
//
// A required field that is an identifier is taken from what earlier calls
// returned. When nothing produced it, this fails with the field named, and the
// caller skips the step rather than sending a value nobody can justify.
func minimalBody(doc *contract.Doc, schemaName string, pool *pool) (map[string]any, error) {
	body := map[string]any{}
	if schemaName == "" {
		// No request schema. Either the operation takes no body, or the document
		// declares it inline and the extraction could not name it — which is
		// what every Scaleway operation does, and why an empty body is the only
		// honest thing to send.
		return body, nil
	}

	schema, known := doc.Schemas[schemaName]
	if !known {
		return nil, fmt.Errorf("the contract has no schema named %s", schemaName)
	}

	for _, field := range schema.Required {
		prop, declared := schema.Properties[field]
		if !declared {
			return nil, fmt.Errorf("%s requires %s and does not declare it", schemaName, field)
		}
		value, err := valueFor(doc, field, prop, pool)
		if err != nil {
			return nil, err
		}
		body[field] = value
	}
	return body, nil
}

// valueFor produces one field's value, from the pool when it is an identifier
// and from the schema when the schema constrains it.
func valueFor(doc *contract.Doc, field string, prop contract.Property, pool *pool) (any, error) {
	// An enumerated field has its answer in the document. First value, so two
	// runs send the same thing.
	if len(prop.Enum) > 0 {
		return prop.Enum[0], nil
	}

	switch {
	case prop.Type == "array":
		if prop.Items == nil {
			return []any{}, nil
		}
		item, err := valueFor(doc, singular(field), *prop.Items, pool)
		if err != nil {
			return nil, err
		}
		return []any{item}, nil

	case prop.Ref != "":
		nested, err := minimalBody(doc, prop.Ref, pool)
		if err != nil {
			return nil, err
		}
		return nested, nil

	case prop.Type == "boolean":
		return false, nil
	case prop.Type == "integer", prop.Type == "number":
		return 1, nil
	}

	// A string. If it looks like an identifier, it must come from somewhere
	// real; anything else is a name the emulator will not validate.
	if looksLikeID(field) {
		if value, found := pool.take(field); found {
			return value, nil
		}
		return nil, fmt.Errorf("%s is an identifier and nothing in the plan produced one", field)
	}
	return "feint-probe", nil
}

// looksLikeID recognises the fields whose value has to exist. All three
// providers name them the same way, in their own case: VmId, ImageId, net-id,
// private_network_id.
func looksLikeID(field string) bool {
	lower := strings.ToLower(strings.TrimSuffix(strings.ToLower(field), "s"))
	return strings.HasSuffix(lower, "id") || strings.HasSuffix(lower, "-id") ||
		strings.HasSuffix(lower, "_id")
}

// singular turns a plural field name into the key its elements are harvested
// under: VmIds holds VmId values.
func singular(field string) string {
	if strings.HasSuffix(field, "s") {
		return field[:len(field)-1]
	}
	return field
}

// pool holds the identifiers earlier calls returned, keyed by the field name
// they arrived under, folded.
//
// This is what replaces a scenario written by hand: a create answers with an id,
// and the delete that follows takes it from here. Nothing else is remembered,
// because nothing else is safe to reuse.
type pool struct {
	mu     sync.Mutex
	values map[string][]string
}

// seeds are the path parameters no call produces because they are not resources:
// a zone and a region are coordinates, and every provider's paths carry one.
//
// The values are the emulator's own defaults, which is what makes them safe: a
// zone it does not know is refused, and the probe would be measuring its own
// bad guess rather than the emulator.
var seeds = map[string]string{
	"zone":   "fr-par-1",
	"region": "fr-par",
}

func newPool() *pool {
	values := map[string][]string{}
	for key, value := range seeds {
		values[key] = []string{value}
	}
	return &pool{values: values}
}

// fill substitutes a path's parameters. It reports the first one it cannot
// satisfy, so the step is skipped with a reason rather than sent with a literal
// brace — which is what every Scaleway route did on the first run, and produced
// a wall of 400s that said nothing.
func (p *pool) fill(path string) (string, error) {
	out := path
	for {
		open := strings.Index(out, "{")
		if open < 0 {
			return out, nil
		}
		closing := strings.Index(out[open:], "}")
		if closing < 0 {
			return "", fmt.Errorf("malformed path %q", path)
		}
		name := out[open+1 : open+closing]

		value, found := p.take(name)
		if !found {
			// Path parameters are named for what they hold rather than by their
			// type: {id} under /servers is a server id. Tried both ways before
			// giving up.
			value, found = p.takeAny(name)
		}
		if !found {
			return "", fmt.Errorf("%s is a path parameter and nothing in the plan produced one", name)
		}
		out = out[:open] + value + out[open+closing+1:]
	}
}

// takeAny answers a generic parameter name — {id}, {ipID} — with any identifier
// harvested so far. Deliberately loose: the alternative is a table mapping every
// path to the resource it addresses, which is a second source of truth to keep
// in step with the document.
func (p *pool) takeAny(name string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !looksLikeID(name) && strings.ToLower(name) != "id" {
		return "", false
	}
	keys := make([]string, 0, len(p.values))
	for key := range p.values {
		if _, isSeed := seeds[key]; !isSeed && len(p.values[key]) > 0 {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return "", false
	}
	sort.Strings(keys)
	return p.values[keys[0]][0], true
}

func (p *pool) take(field string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.takeLocked(field)
}

func (p *pool) takeLocked(field string) (string, bool) {
	got, found := p.values[strings.ToLower(field)]
	if !found || len(got) == 0 {
		// A plural field takes a singular value: VmIds is satisfied by a VmId.
		got, found = p.values[strings.ToLower(singular(field))]
		if !found || len(got) == 0 {
			return "", false
		}
	}
	return got[0], true
}

// harvest records every identifier a response carried, at any depth.
func (p *pool) harvest(value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.walk(value)
}

func (p *pool) walk(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if text, ok := nested.(string); ok && text != "" && looksLikeID(key) {
				folded := strings.ToLower(key)
				p.values[folded] = append(p.values[folded], text)
			}
			p.walk(nested)
		}
	case []any:
		for _, item := range typed {
			p.walk(item)
		}
	}
}
