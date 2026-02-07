package analysis

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func parseFunc(t *testing.T, code string) (*ast.FuncDecl, *token.FileSet) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", "package p; "+code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			return fn, fset
		}
	}
	t.Fatal("no function found")
	return nil, nil
}

func TestAnalyzeFunction(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		want     FunctionMetrics
	}{
		{
			name: "Simple",
			code: "func Simple() {}",
			want: FunctionMetrics{Name: "Simple", LOC: 1, ParameterCount: 0, ReturnCount: 0, Complexity: 1, NestingDepth: 0},
		},
		{
			name: "ParamsAndReturns",
			code: "func Params(a, b int) (int, error) { return 0, nil }",
			want: FunctionMetrics{Name: "Params", LOC: 1, ParameterCount: 2, ReturnCount: 2, Complexity: 1, NestingDepth: 0},
		},
		{
			name: "Nesting",
			code: `func Nesting() {
				if true {
					for {
						if false {}
					}
				}
			}`,
			want: FunctionMetrics{Name: "Nesting", NestingDepth: 3, Complexity: 4}, // 1 base + if + for + if
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, fset := parseFunc(t, tt.code)
			got := AnalyzeFunction(fn, fset)

			if got.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", got.Name, tt.want.Name)
			}
			if tt.want.LOC > 0 && got.LOC != tt.want.LOC {
				t.Errorf("LOC = %v, want %v", got.LOC, tt.want.LOC)
			}
			if got.ParameterCount != tt.want.ParameterCount {
				t.Errorf("ParameterCount = %v, want %v", got.ParameterCount, tt.want.ParameterCount)
			}
			if got.ReturnCount != tt.want.ReturnCount {
				t.Errorf("ReturnCount = %v, want %v", got.ReturnCount, tt.want.ReturnCount)
			}
			if tt.want.Complexity > 0 && got.Complexity != tt.want.Complexity {
				t.Errorf("Complexity = %v, want %v", got.Complexity, tt.want.Complexity)
			}
			if tt.want.NestingDepth > 0 && got.NestingDepth != tt.want.NestingDepth {
				t.Errorf("NestingDepth = %v, want %v", got.NestingDepth, tt.want.NestingDepth)
			}
		})
	}
}

func TestCalculateComplexity(t *testing.T) {
	code := `func Complex(n int) {
		if n > 0 { // +1
			if n > 10 { // +1
			}
		} else if n < 0 { // +1
		}

		for i := 0; i < 10; i++ { // +1
		}

		switch n {
		case 1: // +1
		case 2: // +1
		}

		if a && b || c { // +2
		}
	}`
	// Base 1 + 8 = 9
	// Logic:
	// Base = 1
	// if n > 0 -> +1
	//   if n > 10 -> +1
	// else if n < 0 -> +1 (else if is just an if in the else block usually, but parser handles it)
	// for -> +1
	// case 1 -> +1
	// case 2 -> +1
	// if a && b || c
	//   if -> +1
	//   && -> +1
	//   || -> +1
	// Total: 1 + 1+1+1 + 1 + 1+1 + 1+1+1 = 10?
	// Let's count again.
	// 1 (Base)
	// +1 (if n > 0)
	// +1 (if n > 10)
	// +1 (if n < 0) - wait, `else if` is parsed as `Else: &IfStmt`.
	// The ast traversal visits the Else block which contains the IfStmt.
	// So `if` node visitor sees the first if.
	// Then it visits Else, which is an IfStmt.
	// So `else if` counts as +1.
	// +1 (for)
	// +1 (case 1)
	// +1 (case 2)
	// +1 (if a...)
	// +1 (&&)
	// +1 (||)
	// Total = 1 + 1+1+1+1+1+1+1+1+1 = 10.

	fn, _ := parseFunc(t, code)
	got := CalculateComplexity(fn)
	if got != 10 {
		t.Errorf("Complexity = %d, want 10", got)
	}
}

func TestNestingDepth(t *testing.T) {
	code := `func Deep() {
		if a {       // 1
			if b {   // 2
				if c { // 3
				}
				if d { // 3
					if e { // 4
					}
				}
			}
		}
	}`
	fn, _ := parseFunc(t, code)
	got := calculateNestingDepth(fn) // access internal
	if got != 4 {
		t.Errorf("NestingDepth = %d, want 4", got)
	}
}
