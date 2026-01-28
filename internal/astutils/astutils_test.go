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
	tests := []struct {
		name     string
		src      string // Function declaration source
		expected string
	}{
		{
			name:     "Pointer Receiver",
			src:      "func (s *Service) Method() {}",
			expected: "Service",
		},
		{
			name:     "Value Receiver",
			src:      "func (s Service) Method() {}",
			expected: "Service",
		},
		{
			name:     "Generic Receiver (Single)",
			src:      "func (s *Service[T]) Method() {}",
			expected: "Service",
		},
		{
			name:     "Generic Receiver (Multiple)",
			src:      "func (s *Service[K, V]) Method() {}",
			expected: "Service",
		},
		{
			name:     "Generic Receiver (Value)",
			src:      "func (s Service[T]) Method() {}",
			expected: "Service",
		},
		{
			name:     "Unknown Receiver",
			src:      "func (s *[]string) Method() {}", // Invalid receiver in Go but parseable
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "", "package p; "+tt.src, 0)
			require.NoError(t, err)

			decl := f.Decls[0].(*ast.FuncDecl)
			got := GetReceiverTypeName(decl.Recv)
			assert.Equal(t, tt.expected, got)
		})
	}
}
