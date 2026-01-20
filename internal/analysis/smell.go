package analysis

import (
	"go/ast"
	"go/token"
)

// FunctionMetrics holds various code smell metrics for a single function.
type FunctionMetrics struct {
	Name           string
	Line           int
	LOC            int
	ParameterCount int
	ReturnCount    int
	Complexity     int
	NestingDepth   int
}

// AnalyzeFunction calculates metrics for a given function declaration.
func AnalyzeFunction(fn *ast.FuncDecl, fset *token.FileSet) FunctionMetrics {
	// Calculate LOC
	start := fset.Position(fn.Pos()).Line
	end := fset.Position(fn.End()).Line
	loc := end - start + 1

	// Calculate Parameter Count
	paramCount := 0
	if fn.Type.Params != nil {
		paramCount = len(fn.Type.Params.List)
		// Handle "a, b int" as 2 params
		// Actually Type.Params.List contains "a, b int" as one field with 2 names
		// So we count names if present, or just fields if no names?
		// "func(int, int)" -> 2 fields, no names.
		// "func(a, b int)" -> 1 field, 2 names.
		totalParams := 0
		for _, field := range fn.Type.Params.List {
			if len(field.Names) > 0 {
				totalParams += len(field.Names)
			} else {
				totalParams++
			}
		}
		paramCount = totalParams
	}

	// Calculate Return Count
	returnCount := 0
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			if len(field.Names) > 0 {
				returnCount += len(field.Names)
			} else {
				returnCount++
			}
		}
	}

	// Calculate Complexity
	complexity := CalculateComplexity(fn)

	// Calculate Nesting Depth
	nesting := calculateNestingDepth(fn)

	return FunctionMetrics{
		Name:           fn.Name.Name,
		Line:           start,
		LOC:            loc,
		ParameterCount: paramCount,
		ReturnCount:    returnCount,
		Complexity:     complexity,
		NestingDepth:   nesting,
	}
}

func calculateNestingDepth(fn *ast.FuncDecl) int {
	maxDepth := 0
	v := depthVisitor{depth: 0, maxDepth: &maxDepth}
	ast.Walk(v, fn.Body)
	return maxDepth
}

type depthVisitor struct {
	depth    int
	maxDepth *int
}

func (v depthVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	newDepth := v.depth
	switch node.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SelectStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
		newDepth++
	}

	if newDepth > *v.maxDepth {
		*v.maxDepth = newDepth
	}

	return depthVisitor{
		depth:    newDepth,
		maxDepth: v.maxDepth,
	}
}
