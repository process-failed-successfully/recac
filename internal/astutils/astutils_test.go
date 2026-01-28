package astutils

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReceiverTypeName(t *testing.T) {
	src := `package p
func (s MyStruct) M1() {}
func (s *MyStruct) M2() {}
func (s Generic[T]) M3() {}
func (s *Generic[T]) M4() {}
func (s Multi[K, V]) M5() {}
func (s *Multi[K, V]) M6() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	require.NoError(t, err)

	type check struct {
		funcName string
		wantType string
	}

	checks := []check{
		{"M1", "MyStruct"},
		{"M2", "MyStruct"},
		{"M3", "Generic"},
		{"M4", "Generic"},
		{"M5", "Multi"},
		{"M6", "Multi"},
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		got := GetReceiverTypeName(fn.Recv)
		// Find expected
		for _, c := range checks {
			if fn.Name.Name == c.funcName {
				assert.Equal(t, c.wantType, got, "Failed for %s", c.funcName)
			}
		}
	}
}
