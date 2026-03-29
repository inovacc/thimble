package analysis

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ExtractGoReferences parses a Go file and extracts call references from function bodies.
func ExtractGoReferences(path string) ([]Reference, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	pkg := file.Name.Name

	var refs []Reference

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		caller := qualifiedFuncName(pkg, fn)

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch expr := n.(type) { //nolint:gocritic // type switch idiomatic for AST
			case *ast.CallExpr:
				callee := resolveCallee(expr.Fun)
				if callee != "" {
					refs = append(refs, Reference{
						From: caller,
						To:   callee,
						Kind: "call",
						File: filepath.ToSlash(fset.Position(expr.Pos()).Filename),
						Line: fset.Position(expr.Pos()).Line,
					})
				}
			}

			return true
		})
	}

	// Extract interface embeds from type declarations.
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			typeName := pkg + "." + ts.Name.Name

			switch t := ts.Type.(type) {
			case *ast.InterfaceType:
				if t.Methods == nil {
					continue
				}

				for _, m := range t.Methods.List {
					if len(m.Names) == 0 {
						// Embedded interface.
						embedded := formatExpr(m.Type)
						refs = append(refs, Reference{
							From: typeName,
							To:   embedded,
							Kind: "embeds",
							File: filepath.ToSlash(fset.Position(m.Pos()).Filename),
							Line: fset.Position(m.Pos()).Line,
						})
					}
				}
			case *ast.StructType:
				if t.Fields == nil {
					continue
				}

				for _, f := range t.Fields.List {
					if len(f.Names) == 0 {
						// Embedded struct.
						embedded := formatExpr(f.Type)
						refs = append(refs, Reference{
							From: typeName,
							To:   embedded,
							Kind: "embeds",
							File: filepath.ToSlash(fset.Position(f.Pos()).Filename),
							Line: fset.Position(f.Pos()).Line,
						})
					}
				}
			}
		}
	}

	return refs, nil
}

func qualifiedFuncName(pkg string, fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := formatExpr(fn.Recv.List[0].Type)
		recv = strings.TrimPrefix(recv, "*")

		return pkg + "." + recv + "." + fn.Name.Name
	}

	return pkg + "." + fn.Name.Name
}

func resolveCallee(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		// Local or builtin call: e.g., `make`, `len`, `localFunc`.
		return e.Name
	case *ast.SelectorExpr:
		// pkg.Func or receiver.Method.
		x := resolveCallee(e.X)
		if x != "" {
			return x + "." + e.Sel.Name
		}

		return e.Sel.Name
	case *ast.FuncLit:
		return "" // anonymous function
	case *ast.ParenExpr:
		return resolveCallee(e.X)
	case *ast.TypeAssertExpr:
		return "" // type assertion, not a call
	case *ast.IndexExpr:
		// Generic function call: f[T](...)
		return resolveCallee(e.X)
	default:
		return ""
	}
}
