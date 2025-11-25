package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// TestInfo holds information about a discovered test
type TestInfo struct {
	Name    string // Function name (e.g., TestFoo)
	Package string // Package path
	File    string // Source file path
	Line    int    // Line number where the test function starts
}

// DiscoverTests finds all Go test functions in the given directory
func DiscoverTests(dir string) ([]TestInfo, error) {
	return DiscoverTestsRecursive(dir, true)
}

// DiscoverTestsRecursive finds all Go test functions with recursive option
func DiscoverTestsRecursive(dir string, recursive bool) ([]TestInfo, error) {
	var tests []TestInfo

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-test files
		if info.IsDir() {
			// Skip vendor and hidden directories
			if info.Name() == "vendor" || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			// Skip subdirectories if not recursive (but allow the root dir)
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process *_test.go files
		if !strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		// Parse the file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			// Skip files that can't be parsed
			return nil
		}

		// Get package directory relative to the search directory
		pkgDir, err := filepath.Rel(dir, filepath.Dir(path))
		if err != nil {
			pkgDir = filepath.Dir(path)
		}
		if pkgDir == "." {
			pkgDir = ""
		}

		// Find test functions
		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			// Check if it's a test function
			if isTestFunc(fn) {
				pos := fset.Position(fn.Pos())
				tests = append(tests, TestInfo{
					Name:    fn.Name.Name,
					Package: pkgDir,
					File:    path,
					Line:    pos.Line,
				})
			}
		}

		return nil
	})

	return tests, err
}

// isTestFunc checks if a function declaration is a test function
func isTestFunc(fn *ast.FuncDecl) bool {
	// Must be exported and start with "Test"
	name := fn.Name.Name
	if !strings.HasPrefix(name, "Test") {
		return false
	}

	// Must have exactly one parameter of type *testing.T
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}

	// Check parameter type is *testing.T
	param := fn.Type.Params.List[0]
	starExpr, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}

	selExpr, ok := starExpr.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	return ident.Name == "testing" && selExpr.Sel.Name == "T"
}
