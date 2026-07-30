package drift

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Outscale's SDK is not shaped like Scaleway's, so it needs its own reader.
//
// Scaleway generates one package per product and version from an internal IDL,
// and every operation is a method on an API receiver that builds its own
// request. Outscale generates from OpenAPI with oapi-codegen: one package per
// service, one enormous client.gen.go, and every operation a method on *Client.
// Nothing about the first reader transfers, which is the point of keeping them
// apart rather than growing one function two shapes wide.
//
// The spec itself ships next door as pkg/<service>/api.yaml, and it would be the
// more direct source. It is not read because parsing YAML would be the project's
// first external dependency, and it would buy nothing: oapi-codegen emits
// exactly one *Client method per operationId, so the generated Go is a faithful
// projection of the spec.

// oapiClient is the receiver oapi-codegen puts every operation on.
const oapiClient = "Client"

// ScanOutscaleSDK walks <root>/pkg/*/client.gen.go and returns every operation
// of every service the SDK ships.
//
// Services are separate surfaces on separate hosts — the main API answers under
// /api/v1, Kubernetes under a different name entirely — so the package name
// becomes the product, and a whole service can be declined in one decision the
// way an alpha version can.
func ScanOutscaleSDK(root string) ([]Operation, error) {
	pkgDir := filepath.Join(root, "pkg")
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("read outscale sdk at %s: %w", pkgDir, err)
	}

	var ops []Operation
	for _, service := range entries {
		if !service.IsDir() {
			continue
		}
		path := filepath.Join(pkgDir, service.Name(), "client.gen.go")
		if _, err := os.Stat(path); err != nil {
			// Most packages are plumbing: middleware, profiles, logging. Only
			// the generated ones describe API surface.
			continue
		}
		found, err := scanOAPIFile(path, service.Name())
		if err != nil {
			return nil, err
		}
		ops = append(ops, found...)
	}

	// A checkout that yields nothing is a layout change, not an empty API. Left
	// silent it would report full coverage of a surface nobody scanned, which is
	// the one failure this whole mechanism exists to prevent.
	if len(ops) == 0 {
		return nil, fmt.Errorf("no operation found under %s: the SDK layout changed", pkgDir)
	}

	sort.Slice(ops, func(i, j int) bool { return ops[i].Name < ops[j].Name })
	return ops, nil
}

func scanOAPIFile(path, service string) ([]Operation, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	seen := make(map[string]bool)
	var ops []Operation
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || !fn.Name.IsExported() {
			continue
		}
		// Only the typed client. ClientRaw carries the same operations with a
		// Raw suffix and returns the bare response, and the Resp types carry
		// decoding helpers: counting either would double or triple a surface
		// that has not grown.
		if receiverName(fn.Recv.List[0].Type) != oapiClient {
			continue
		}
		// Each operation is emitted twice, once taking a typed body and once
		// taking an io.Reader. One endpoint, two signatures.
		name := strings.TrimSuffix(fn.Name.Name, "WithBody")
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		// Version stays empty on purpose. The generated client carries no API
		// version: the path one lives in the endpoint template, the constant
		// next to it is the SDK's own release, and neither is a property of the
		// operation. Inventing a "v1" here would be a fact nobody could check,
		// which is what this package exists not to do. The service name is the
		// whole locator, and the pack owns the path.
		ops = append(ops, Operation{
			Name:    service + "/" + oapiClient + "." + name,
			Product: service,
		})
	}
	return ops, nil
}
