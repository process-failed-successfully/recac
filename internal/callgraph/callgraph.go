package callgraph

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CallMap maps Caller -> []Callee
type CallMap map[string][]string

func AnalyzeCalls(root string) (CallMap, error) {
	calls := make(CallMap)
	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" || name == ".recac" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		pkgName := f.Name.Name

		// Map to store current function scope
		var currentFunc string

		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				// Enter function
				funcName := x.Name.Name
				recv := ""
				if x.Recv != nil && len(x.Recv.List) > 0 {
					// Method
					typeExpr := x.Recv.List[0].Type
					recv = getTypeName(typeExpr) + "."
				}
				currentFunc = pkgName + "." + recv + funcName
				if _, exists := calls[currentFunc]; !exists {
					calls[currentFunc] = []string{}
				}
			case *ast.CallExpr:
				if currentFunc == "" {
					return true
				}

				callee := resolveCallee(x.Fun)
				if callee != "" {
					// Attempt to resolve local package calls
					if !strings.Contains(callee, ".") {
						callee = pkgName + "." + callee
					}
					calls[currentFunc] = append(calls[currentFunc], callee)
				}
			}
			return true
		})

		return nil
	})

	return calls, err
}

func resolveCallee(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		// Local function call
		return f.Name
	case *ast.SelectorExpr:
		// pkg.Func or obj.Method
		return getTypeName(f.X) + "." + f.Sel.Name
	}
	return ""
}

func getTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + getTypeName(t.X)
	case *ast.SelectorExpr:
		return getTypeName(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + getTypeName(t.Elt)
	case *ast.MapType:
		return "map[" + getTypeName(t.Key) + "]" + getTypeName(t.Value)
	default:
		return "unknown"
	}
}

func GenerateMermaidCallGraph(calls CallMap, focus *regexp.Regexp, depth int) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	relevant := filterGraph(calls, focus, depth)

	var keys []string
	for k := range relevant {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, caller := range keys {
		callees := calls[caller]
		seen := make(map[string]bool)
		sort.Strings(callees)

		for _, callee := range callees {
			if seen[callee] {
				continue
			}
			seen[callee] = true
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", sanitizeID(caller), sanitizeID(callee)))
		}
	}

	return sb.String()
}

func GenerateDotCallGraph(calls CallMap, focus *regexp.Regexp, depth int) string {
	var sb strings.Builder
	sb.WriteString("digraph CallGraph {\n")
	sb.WriteString("    node [shape=box];\n")

	relevant := filterGraph(calls, focus, depth)

	var keys []string
	for k := range relevant {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, caller := range keys {
		callees := calls[caller]
		seen := make(map[string]bool)
		sort.Strings(callees)

		for _, callee := range callees {
			if seen[callee] {
				continue
			}
			seen[callee] = true
			sb.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\";\n", caller, callee))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

func filterGraph(calls CallMap, focus *regexp.Regexp, depth int) map[string]bool {
	if focus == nil {
		relevant := make(map[string]bool)
		for k := range calls {
			relevant[k] = true
		}
		return relevant
	}

	relevant := make(map[string]bool)
	queue := []string{}

	for k := range calls {
		if focus.MatchString(k) {
			if !relevant[k] {
				relevant[k] = true
				queue = append(queue, k)
			}
		}
	}

	callersOf := make(map[string][]string)
	for caller, callees := range calls {
		for _, callee := range callees {
			callersOf[callee] = append(callersOf[callee], caller)
		}
	}

	add := func(n string) {
		if !relevant[n] {
			relevant[n] = true
			queue = append(queue, n)
		}
	}

	currentDepth := 0
	for len(queue) > 0 {
		if depth > 0 && currentDepth >= depth {
			break
		}

		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			node := queue[0]
			queue = queue[1:]

			for _, callee := range calls[node] {
				add(callee)
			}

			for _, caller := range callersOf[node] {
				add(caller)
			}
		}
		currentDepth++
	}

	return relevant
}

func sanitizeID(id string) string {
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, "*", "ptr_")
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "-", "_")
	return id
}
