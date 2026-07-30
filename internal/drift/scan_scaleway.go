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

// Scaleway generates one package per product and version from its own IDL. Every
// operation is a method on a receiver whose name ends in API, and the hand-written
// entry points sit in the same directories as the generated ones.

// ScanScalewaySDK walks <root>/api/<product>/<version>/*.go and returns every
// exported method declared on an API receiver.
//
// It parses rather than greps: a comment or a string containing "func (s *API)"
// would fool a regex, and a silently inflated surface makes the coverage report
// lie in the reassuring direction.
func ScanScalewaySDK(root string) ([]Operation, error) {
	apiDir := filepath.Join(root, "api")
	entries, err := os.ReadDir(apiDir)
	if err != nil {
		return nil, fmt.Errorf("read scaleway sdk at %s: %w", apiDir, err)
	}

	var ops []Operation
	for _, product := range entries {
		if !product.IsDir() {
			continue
		}
		versions, err := os.ReadDir(filepath.Join(apiDir, product.Name()))
		if err != nil {
			return nil, fmt.Errorf("read product %s: %w", product.Name(), err)
		}
		for _, version := range versions {
			if !version.IsDir() {
				continue
			}
			dir := filepath.Join(apiDir, product.Name(), version.Name())
			found, err := scanSDKDir(dir, product.Name(), version.Name())
			if err != nil {
				return nil, err
			}
			ops = append(ops, found...)
		}
	}

	sort.Slice(ops, func(i, j int) bool { return ops[i].Name < ops[j].Name })
	return ops, nil
}

func scanSDKDir(dir, product, version string) ([]Operation, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	var ops []Operation
	for _, f := range files {
		// Every non-test .go file counts, not just *_sdk.go. Scaleway hand-writes
		// some public entry points in *_utils.go: CreateServer and UpdateServer
		// live there and delegate to the unexported generated createServer. A
		// scan restricted to the generated files reports them as missing while
		// they are the very methods callers use.
		if !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, f.Name())
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || !fn.Name.IsExported() {
				continue
			}
			recv := receiverName(fn.Recv.List[0].Type)
			if !strings.HasSuffix(recv, "API") {
				continue
			}
			if !issuesRequest(fn) {
				continue
			}
			ops = append(ops, Operation{
				Name:    fmt.Sprintf("%s/%s/%s.%s", product, version, recv, fn.Name.Name),
				Product: product,
				Version: version,
			})
		}
	}
	return ops, nil
}

// issuesRequest reports whether a method is an endpoint of its own, rather than
// a client-side convenience built on endpoints already counted.
//
// Two shapes are surface. A generated method builds its own request, always as
// a scw.ScalewayRequest literal handed to the client. A hand-written entry point
// delegates to an unexported method of the same receiver: CreateServer is the
// public name of the generated createServer, and dropping it would report as
// missing the very method every caller uses.
//
// What is left composes exported methods, and composition adds no endpoint an
// emulator could serve. ServerActionAndWait is ServerAction plus polling,
// TryDeletingPrivateNetwork is DeletePrivateNetwork plus retries, WaitForServer
// is GetServer in a loop. Zones() and Regions() go further and call nothing at
// all: they return a constant slice.
//
// This matters beyond tidiness. Counting convenience as surface inflates the
// denominator, so the coverage report understates what is served and the triage
// list fills with entries nobody can act on — which is how a drift mechanism
// stops being read.
func issuesRequest(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}
	recv := receiverVar(fn)

	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch node := n.(type) {
		case *ast.CompositeLit:
			if isRequestLiteral(node.Type) {
				found = true
			}
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok || recv == "" {
				return true
			}
			switch target := sel.X.(type) {
			case *ast.Ident:
				// s.createServer: the generated half of a hand-written entry
				// point, so this method is that endpoint's public name.
				if target.Name == recv && !sel.Sel.IsExported() {
					found = true
				}
			case *ast.SelectorExpr:
				// s.client.Do, for a method that assembled its request
				// somewhere this walk cannot see.
				//
				// Only Do. The client also answers GetDefaultZone,
				// GetDefaultRegion, GetDefaultPageSize and four more, all local
				// reads of configuration that reach nothing — and every
				// generated method calls one of them, so accepting any call on
				// the client counted convenience as surface. It did:
				// GetAllServerUserData is ListServerUserData plus GetServerUserData
				// in a loop, and it was reported as an endpoint because it asks
				// the client for a default zone first. Found by the contract
				// cross-check, which knows the operation does not exist.
				if inner, ok := target.X.(*ast.Ident); ok &&
					inner.Name == recv && target.Sel.Name == "client" &&
					strings.HasPrefix(sel.Sel.Name, "Do") {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// isRequestLiteral matches the scw.ScalewayRequest composite literal every
// generated method builds.
func isRequestLiteral(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ScalewayRequest" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "scw"
}

// receiverVar returns the receiver's variable name, empty for the anonymous
// form func (*API) Foo() which cannot delegate to anything.
func receiverVar(fn *ast.FuncDecl) string {
	if len(fn.Recv.List[0].Names) == 0 {
		return ""
	}
	return fn.Recv.List[0].Names[0].Name
}
