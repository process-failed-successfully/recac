package astutils

import (
	"go/ast"
)

// GetReceiverTypeName extracts the type name from a receiver field list.
// It handles pointers, identifiers, and generic types (IndexExpr and IndexListExpr).
func GetReceiverTypeName(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	if index, ok := expr.(*ast.IndexExpr); ok {
		if ident, ok := index.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	// Support for Go 1.18+ IndexListExpr (for multiple type parameters)
	if indexList, ok := expr.(*ast.IndexListExpr); ok {
		if ident, ok := indexList.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return "Unknown"
}
