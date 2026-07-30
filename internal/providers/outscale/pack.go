// Package outscale emulates the Outscale API.
//
// Outscale speaks a different dialect from the other two packs, and that is the
// point of hosting it here: the core must stay protocol-neutral. Every call is a
// POST on /api/v1/<Action> with a JSON body, responses carry a ResponseContext
// with a RequestId, and errors come back as an Errors array. Authentication is
// AWS Signature v4; the emulator accepts any signature without checking it.
//
// Shapes are read from Outscale's own OpenAPI document, which ships inside their
// Go SDK, and the emulator checks itself against it: contracts/outscale.json is
// extracted from that document and internal/contract fails a response carrying a
// field the API does not define. That is not a rule invented here — 643 of their
// 650 schemas declare additionalProperties: false.
//
// Everything the pack serves is declared in Routes with its upstream operation
// name, and everything it deliberately does not serve is declared in Declined.
package outscale

import (
	"net/http"
	"strings"
	"sync"

	"github.com/stephrobert/feint/internal/core/emulator"
)

// Name is the provider key.
const Name = "outscale"

// The resource kinds this pack stores.
const (
	kindVM      = "vm"
	kindKeypair = "keypair"
)

// Pack implements emulator.Pack for Outscale.
type Pack struct {
	env *emulator.Env
	// addresses serialises the choice of a block, which is read-modify-write
	// over the store: what is free is computed from what exists, and two
	// concurrent creates without it both find the same range free and both take
	// it. Terraform creates ten resources at a time by default.
	addresses sync.Mutex
}

// New returns an Outscale pack backed by env.
func New(env *emulator.Env) *Pack { return &Pack{env: env} }

// Name implements emulator.Pack.
func (p *Pack) Name() string { return Name }

// operation returns the canonical name of an Outscale call, in the form the
// drift scan produces from their SDK. The action itself is what the contract
// keys on, which is why both live one string apart.
func operation(action string) string { return "osc/Client." + action }

// route declares one action. Outscale has no path structure to speak of: every
// call is a POST on /api/v1/<Action>, so a route is fully described by its
// action name and its handler.
func route(action string, handler http.HandlerFunc) emulator.Route {
	return emulator.Route{
		Method:    "POST",
		Path:      pathPrefix + action,
		Operation: operation(action),
		Handler:   handler,
	}
}

// Routes implements emulator.Pack.
func (p *Pack) Routes() []emulator.Route {
	return []emulator.Route{
		// Vms: the machine and its lifecycle.
		route("ReadVms", p.readVms),
		route("CreateVms", p.createVms),
		route("UpdateVm", p.updateVm),
		route("DeleteVms", p.deleteVms),
		route("StartVms", p.startVms),
		route("StopVms", p.stopVms),
		route("RebootVms", p.rebootVms),

		// The inventory every client reads before it creates anything. The
		// Scaleway pack learned this the hard way: decline the catalogue and the
		// official CLI cannot create a server at all.
		route("ReadVmTypes", p.readVmTypes),
		route("ReadImages", p.readImages),
		route("ReadRegions", p.readRegions),
		route("ReadSubregions", p.readSubregions),

		// Nets and Subnets: the addressing plane. A block is parsed, its mask
		// bounded, its containment and its overlap checked, and the address count
		// computed from the mask — none of which the emulators this project
		// measures itself against do.
		route("CreateNet", p.createNet),
		route("ReadNets", p.readNets),
		route("DeleteNet", p.deleteNet),
		route("CreateSubnet", p.createSubnet),
		route("ReadSubnets", p.readSubnets),
		route("DeleteSubnet", p.deleteSubnet),

		// Keypairs, on the critical path to a machine anyone can log into.
		route("CreateKeypair", p.createKeypair),
		route("ReadKeypairs", p.readKeypairs),
		route("DeleteKeypair", p.deleteKeypair),
	}
}

// pathPrefix is the whole of Outscale's URL space: one flat namespace, every
// call a POST on /api/v1/<Action>.
const pathPrefix = "/api/v1/"

// Prefixes implements emulator.Unrouted.
func (p *Pack) Prefixes() []string { return []string{pathPrefix} }

// NotFound implements emulator.Unrouted.
//
// The body is the SDK's ErrorResponse shape, so a client decodes an API error
// instead of choking on net/http's text/plain. Code is deliberately empty: the
// numeric codes are Outscale's own catalogue, this is the emulator speaking, and
// minting a number would be exactly the kind of invented format this project
// never ships: a shape comes from their SDK or not at all. An empty
// code is what "not one of theirs" looks like, and no upstream check matches it.
//
// 404 rather than 501, decided by what the real client does. 501 says what is
// true — the operation exists, this server does not implement it — but oapi-cli
// treats it as retryable and spends 12 seconds on three backed-off attempts
// before giving up, measured. On 404 it returns at once. With most of the
// surface still unserved, that answer is the common case, not the exception.
//
// The Go SDK would not have shown this: it retries through go-retryablehttp,
// whose default policy excludes 501 by name. Two official clients, two
// behaviours, and only the one that was run said anything.
func (p *Pack) NotFound(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimPrefix(r.URL.Path, pathPrefix)
	p.writeError(w, http.StatusNotFound, "", "OperationNotEmulated",
		"feint does not serve "+action+"; see /_feint/routes for what it does")
}

type responseContext struct {
	RequestID string `json:"RequestId"`
}

func (p *Pack) context() responseContext {
	return responseContext{RequestID: p.env.NewID()}
}

// Outscale identifiers are a prefix and a hexadecimal suffix, not UUIDs. The
// prefixes and their lengths are read from their OpenAPI document, which spells
// them out in the field descriptions: 91 examples of vpc-, 63 of i-, 56 of sg-,
// all eight hexadecimal characters, and key- alone at thirty-two.
//
// It matters because clients validate the shape. A UUID where an i-<8> belongs
// is the kind of thing that passes every unit test and fails the first real
// call.
const (
	idLen        = 8
	keypairIDLen = 32
)

// newID builds one, prefix and eight hexadecimal characters.
func newID(prefix, seed string) string { return prefix + "-" + hexOf(seed, idLen) }

func newVMID(seed string) string      { return newID("i", seed) }
func newKeypairID(seed string) string { return "key-" + hexOf(seed, keypairIDLen) }

// hexOf takes the hexadecimal characters of a generated UUID, which is where the
// entropy is; the dashes carry none.
func hexOf(seed string, n int) string {
	trimmed := strings.ReplaceAll(seed, "-", "")
	if len(trimmed) < n {
		// Cannot happen with the env's own UUIDs, which give 32 characters.
		// Padding rather than panicking keeps a bad seed from taking the server
		// down over a cosmetic property.
		return trimmed + strings.Repeat("0", n-len(trimmed))
	}
	return trimmed[:n]
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// Env implements emulator.Pack.
//
// The values come from tools/conformance/outscale/fake-credentials.env, so the
// CLI and the suite cannot drift apart. They are deliberately public and open
// nothing: the emulator accepts any signature without checking it, but the
// client still refuses to sign a request whose credentials are not well-formed.
//
// OSC_ENDPOINT_API takes the bare host. The CLI appends /api/v1/<Call> itself,
// so an endpoint carrying that prefix produces a request for
// /api/v1/api/v1/<Call> — a 404 that reads exactly like a missing route, and an
// afternoon to diagnose the first time.
func (p *Pack) Env(endpoint string) emulator.Environment {
	return emulator.Environment{
		Vars: map[string]string{
			"OSC_ACCESS_KEY":   "AAAAAAAAAAAAAAAAAAAA",
			"OSC_SECRET_KEY":   "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			"OSC_REGION":       regionName,
			"OSC_PROTOCOL":     "http",
			"OSC_ENDPOINT_API": endpoint,
		},
		Note: "oapi-cli reads a JSON profile rather than the environment: --config, with endpoints.api set to " +
			endpoint + " (the bare host; it appends /api/v1 itself).",
	}
}
