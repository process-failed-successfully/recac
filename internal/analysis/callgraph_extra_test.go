package analysis

import (
	"go/ast"
	"testing"
)

func TestGetReceiverTypeName(t *testing.T) {
	tests := []struct {
		name     string
		recv     *ast.FieldList
		expected string
	}{
		{
			name:     "empty",
			recv:     &ast.FieldList{},
			expected: "",
		},
		{
			name: "ident",
			recv: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.Ident{Name: "MyType"},
					},
				},
			},
			expected: "MyType",
		},
		{
			name: "star expr",
			recv: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.StarExpr{
							X: &ast.Ident{Name: "MyType"},
						},
					},
				},
			},
			expected: "MyType",
		},
		{
			name: "index expr",
			recv: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.IndexExpr{
							X: &ast.Ident{Name: "MyGeneric"},
						},
					},
				},
			},
			expected: "MyGeneric",
		},
		{
			name: "other expr",
			recv: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.ArrayType{},
					},
				},
			},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getReceiverTypeName(tt.recv)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
