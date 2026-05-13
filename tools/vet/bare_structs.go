// bare_structs is a go vet analyzer that flags &TypeName{} composite literals
// for types that have generated New*() constructors via gen-defaults.
package main

import (
	"bufio"
	"go/ast"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var Analyzer = &analysis.Analyzer{
	Name: "bare_structs",
	Doc:  "flags &TypeName{} literals for types that have generated New*() constructors",
	Run:  run,
}

// typesFilePath is set via Analyzer.Flags before Run is called.
var typesFilePath string

// generatedTypes is populated lazily from typesFilePath.
// Setting it to nil forces a reload (used in tests).
var generatedTypes map[string]bool

func init() {
	Analyzer.Flags.StringVar(&typesFilePath, "types-file", "rive/gen_defaults_types.txt",
		"path to gen_defaults_types.txt sidecar emitted by gen-defaults")
}

func loadTypes() map[string]bool {
	if generatedTypes != nil {
		return generatedTypes
	}
	generatedTypes = make(map[string]bool)
	f, err := os.Open(typesFilePath)
	if err != nil {
		return generatedTypes
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			generatedTypes[line] = true
		}
	}
	return generatedTypes
}

func run(pass *analysis.Pass) (interface{}, error) {
	types := loadTypes()
	if len(types) == 0 {
		return nil, nil
	}

	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if isGeneratedFile(file, filename) {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			unary, ok := n.(*ast.UnaryExpr)
			if !ok || unary.Op.String() != "&" {
				return true
			}
			lit, ok := unary.X.(*ast.CompositeLit)
			if !ok {
				return true
			}
			typeName := compositeTypeName(lit.Type)
			if typeName == "" || !types[typeName] {
				return true
			}
			pass.Reportf(unary.Pos(),
				"use New%s() instead of &%s{} (generated constructor available)",
				typeName, typeName)
			return true
		})
	}
	return nil, nil
}

// compositeTypeName extracts the bare type name from a composite literal's type expression.
// Handles both Ident (&Shape{}) and SelectorExpr (&rive.Shape{}).
func compositeTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

// isGeneratedFile returns true for files that should be skipped.
// Skips: gen_ prefix filenames and files with "Code generated" header.
func isGeneratedFile(file *ast.File, filename string) bool {
	if strings.HasPrefix(filepath.Base(filename), "gen_") {
		return true
	}
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "Code generated") {
				return true
			}
		}
	}
	return false
}

func main() {
	singlechecker.Main(Analyzer)
}
