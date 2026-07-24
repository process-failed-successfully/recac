package analysis

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Route represents an API route definition found in the code.
type Route struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Source  string `json:"source,omitempty"` // The source code line defining the route
}

// ScanRoutes scans the given root directory for API route definitions.
// It supports standard library (net/http), Gin, and Echo frameworks.
func ScanRoutes(root string) ([]Route, error) {
	var routes []Route
	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse file
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			// Skip malformed files
			return nil
		}

		// AST Inspection
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Looking for call expressions like:
			// - http.HandleFunc("/path", handler)
			// - r.GET("/path", handler)
			// - group.POST("/path", handler)

			// Heuristic: First argument must be a string literal starting with "/"
			if len(call.Args) < 2 {
				return true
			}

			pathArg, ok := call.Args[0].(*ast.BasicLit)
			if !ok || pathArg.Kind != token.STRING {
				return true
			}

			routePath := strings.Trim(pathArg.Value, "\"")
			if !strings.HasPrefix(routePath, "/") {
				return true
			}

			// Analyze the function call (Method)
			var method string

			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				// Check for HTTP verbs
				method = normalizeMethod(fun.Sel.Name)
			case *ast.Ident:
				// Local function call?Unlikely for routing libraries usually attached to structs or packages.
				// But maybe alias?
				method = normalizeMethod(fun.Name)
			}

			if method == "" {
				return true
			}

			// Extract Handler Name
			handlerArg := call.Args[1] // Usually the second argument
			// Gin/Echo: func(c *Context)
			// Stdlib: func(w, r)

			var handlerName string
			switch h := handlerArg.(type) {
			case *ast.Ident:
				handlerName = h.Name
			case *ast.SelectorExpr:
				if ident, ok := h.X.(*ast.Ident); ok {
					handlerName = fmt.Sprintf("%s.%s", ident.Name, h.Sel.Name)
				} else {
					handlerName = fmt.Sprintf("(complex).%s", h.Sel.Name)
				}
			case *ast.FuncLit:
				handlerName = "(anonymous)"
			default:
				handlerName = "(complex)"
			}

			// Record Route
			pos := fset.Position(call.Pos())
			routes = append(routes, Route{
				Method:  method,
				Path:    routePath,
				Handler: handlerName,
				File:    path,
				Line:    pos.Line,
				Source:  extractSourceSnippet(path, pos.Line),
			})

			return true
		})

		return nil
	})

	return routes, err
}

// ⚡ Bolt: Replaced allocation-heavy strings.ToUpper with zero-allocation len switch and strings.EqualFold
func normalizeMethod(name string) string {
	switch len(name) {
	case 3:
		if strings.EqualFold(name, "GET") {
			return "GET"
		}
		if strings.EqualFold(name, "PUT") {
			return "PUT"
		}
	case 4:
		if strings.EqualFold(name, "POST") {
			return "POST"
		}
		if strings.EqualFold(name, "HEAD") {
			return "HEAD"
		}
	case 5:
		if strings.EqualFold(name, "PATCH") {
			return "PATCH"
		}
	case 6:
		if strings.EqualFold(name, "DELETE") {
			return "DELETE"
		}
		if strings.EqualFold(name, "HANDLE") {
			return "ANY" // or "ALL"
		}
	case 7:
		if strings.EqualFold(name, "OPTIONS") {
			return "OPTIONS"
		}
	case 10:
		if strings.EqualFold(name, "HANDLEFUNC") {
			return "ANY" // or "ALL"
		}
	}
	return ""
}

func extractSourceSnippet(path string, line int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	currentLine := 0
	for scanner.Scan() {
		currentLine++
		if currentLine == line {
			return strings.TrimSpace(scanner.Text())
		}
	}
	return ""
}
