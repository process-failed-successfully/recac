package astutils

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
			name:     "Empty receiver",
			recv:     &ast.FieldList{},
			expected: "",
		},
		{
			name: "Value receiver (Ident)",
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
			name: "Pointer receiver (*Ident)",
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
			name: "Generic receiver (IndexExpr)",
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
			name: "Pointer generic receiver (*IndexExpr)",
			recv: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.StarExpr{
							X: &ast.IndexExpr{
								X: &ast.Ident{Name: "MyGeneric"},
							},
						},
					},
				},
			},
			expected: "MyGeneric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetReceiverTypeName(tt.recv)
			if got != tt.expected {
				t.Errorf("GetReceiverTypeName() = %v, want %v", got, tt.expected)
			}
		})
	}
}
