package analysis

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ParseGoFile parses a single Go source file and extracts symbols and references.
func ParseGoFile(path string) (*FileResult, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	result := &FileResult{
		Path:     path,
		Package:  file.Name.Name,
		Language: LangGo,
	}

	// Extract imports.
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		result.Imports = append(result.Imports, importPath)
	}

	// Walk top-level declarations.
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			result.Symbols = append(result.Symbols, extractFunc(fset, d, file))
		case *ast.GenDecl:
			result.Symbols = append(result.Symbols, extractGenDecl(fset, d, file)...)
		}
	}

	// Extract references (call graph, embeds).
	refs, err := ExtractGoReferences(path)
	if err == nil {
		result.References = refs
	}

	return result, nil
}

func extractFunc(fset *token.FileSet, fn *ast.FuncDecl, file *ast.File) Symbol {
	pos := fset.Position(fn.Pos())
	sym := Symbol{
		Name:     fn.Name.Name,
		Kind:     KindFunction,
		Package:  file.Name.Name,
		File:     filepath.ToSlash(pos.Filename),
		Line:     pos.Line,
		Exported: fn.Name.IsExported(),
		Doc:      extractDoc(fn.Doc),
	}

	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sym.Kind = KindMethod
		sym.Receiver = formatExpr(fn.Recv.List[0].Type)
	}

	sym.Signature = formatFuncSignature(fn)

	return sym
}

func extractGenDecl(fset *token.FileSet, decl *ast.GenDecl, file *ast.File) []Symbol {
	var symbols []Symbol

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			pos := fset.Position(s.Pos())
			sym := Symbol{
				Name:     s.Name.Name,
				Package:  file.Name.Name,
				File:     filepath.ToSlash(pos.Filename),
				Line:     pos.Line,
				Exported: s.Name.IsExported(),
				Doc:      extractDoc(decl.Doc),
			}

			switch s.Type.(type) {
			case *ast.StructType:
				sym.Kind = KindStruct
				sym.Signature = formatStructSignature(s)
			case *ast.InterfaceType:
				sym.Kind = KindInterface
				sym.Signature = formatInterfaceSignature(s)
			default:
				sym.Kind = KindType
				sym.Signature = fmt.Sprintf("type %s %s", s.Name.Name, formatExpr(s.Type))
			}

			symbols = append(symbols, sym)

		case *ast.ValueSpec:
			for _, name := range s.Names {
				pos := fset.Position(name.Pos())

				kind := KindVariable
				if decl.Tok == token.CONST {
					kind = KindConstant
				}

				sym := Symbol{
					Name:     name.Name,
					Kind:     kind,
					Package:  file.Name.Name,
					File:     filepath.ToSlash(pos.Filename),
					Line:     pos.Line,
					Exported: name.IsExported(),
					Doc:      extractDoc(decl.Doc),
				}

				if s.Type != nil {
					sym.Signature = fmt.Sprintf("%s %s %s", decl.Tok, name.Name, formatExpr(s.Type))
				} else {
					sym.Signature = fmt.Sprintf("%s %s", decl.Tok, name.Name)
				}

				symbols = append(symbols, sym)
			}
		}
	}

	return symbols
}

func extractDoc(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}

	return strings.TrimSpace(cg.Text())
}

func formatFuncSignature(fn *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")

	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sb.WriteString("(")
		sb.WriteString(formatExpr(fn.Recv.List[0].Type))
		sb.WriteString(") ")
	}

	sb.WriteString(fn.Name.Name)
	sb.WriteString("(")
	sb.WriteString(formatFieldList(fn.Type.Params))
	sb.WriteString(")")

	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		results := formatFieldList(fn.Type.Results)
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			sb.WriteString(" ")
			sb.WriteString(results)
		} else {
			sb.WriteString(" (")
			sb.WriteString(results)
			sb.WriteString(")")
		}
	}

	return sb.String()
}

func formatStructSignature(ts *ast.TypeSpec) string {
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return fmt.Sprintf("type %s struct{}", ts.Name.Name)
	}

	if st.Fields == nil || len(st.Fields.List) == 0 {
		return fmt.Sprintf("type %s struct{}", ts.Name.Name)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "type %s struct { ", ts.Name.Name)

	fields := make([]string, 0, len(st.Fields.List))
	for _, f := range st.Fields.List {
		typeName := formatExpr(f.Type)
		if len(f.Names) == 0 {
			// Embedded field.
			fields = append(fields, typeName)
		} else {
			for _, name := range f.Names {
				fields = append(fields, fmt.Sprintf("%s %s", name.Name, typeName))
			}
		}
	}

	sb.WriteString(strings.Join(fields, "; "))
	sb.WriteString(" }")

	return sb.String()
}

func formatInterfaceSignature(ts *ast.TypeSpec) string {
	iface, ok := ts.Type.(*ast.InterfaceType)
	if !ok {
		return fmt.Sprintf("type %s interface{}", ts.Name.Name)
	}

	if iface.Methods == nil || len(iface.Methods.List) == 0 {
		return fmt.Sprintf("type %s interface{}", ts.Name.Name)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "type %s interface { ", ts.Name.Name)

	methods := make([]string, 0, len(iface.Methods.List))
	for _, m := range iface.Methods.List {
		if len(m.Names) > 0 {
			methods = append(methods, m.Names[0].Name+"()")
		} else {
			// Embedded interface.
			methods = append(methods, formatExpr(m.Type))
		}
	}

	sb.WriteString(strings.Join(methods, "; "))
	sb.WriteString(" }")

	return sb.String()
}

func formatFieldList(fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}

	var parts []string

	for _, f := range fl.List {
		typeName := formatExpr(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typeName)
		} else {
			names := make([]string, len(f.Names))
			for i, n := range f.Names {
				names[i] = n.Name
			}

			parts = append(parts, strings.Join(names, ", ")+" "+typeName)
		}
	}

	return strings.Join(parts, ", ")
}

func formatExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return formatExpr(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + formatExpr(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + formatExpr(e.Elt)
		}

		return "[...]" + formatExpr(e.Elt)
	case *ast.MapType:
		return "map[" + formatExpr(e.Key) + "]" + formatExpr(e.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		return "chan " + formatExpr(e.Value)
	case *ast.FuncType:
		return "func()"
	case *ast.Ellipsis:
		return "..." + formatExpr(e.Elt)
	case *ast.IndexExpr:
		return formatExpr(e.X) + "[" + formatExpr(e.Index) + "]"
	default:
		return "?"
	}
}
