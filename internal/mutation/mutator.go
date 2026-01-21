package mutation

import (
	"go/ast"
	"go/token"
)

// MutationType defines the category of mutation
type MutationType string

const (
	MutationArithmetic MutationType = "Arithmetic"
	MutationCondition  MutationType = "Condition"
	MutationBoolean    MutationType = "Boolean"
)

// Mutation represents a specific change to the code
type Mutation struct {
	ID       string
	Type     MutationType
	File     string
	Line     int
	Original string
	Mutated  string
	Apply    func() // Function to apply the mutation to the AST
	Revert   func() // Function to revert the mutation
}

// Mutator handles the generation of mutations
type Mutator struct {
	Fset *token.FileSet
}

func NewMutator(fset *token.FileSet) *Mutator {
	return &Mutator{Fset: fset}
}

// GenerateMutations traverses the file and returns a list of possible mutations.
// Note: It does not apply them.
func (m *Mutator) GenerateMutations(file *ast.File, filename string) []Mutation {
	var mutations []Mutation

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BinaryExpr:
			m.mutateBinaryExpr(node, filename, &mutations)
		case *ast.Ident:
			m.mutateBooleanIdent(node, filename, &mutations)
		}
		return true
	})

	return mutations
}

func (m *Mutator) mutateBinaryExpr(node *ast.BinaryExpr, filename string, mutations *[]Mutation) {
	var originalOp token.Token
	var newOp token.Token
	var typeStr MutationType

	originalOp = node.Op

	switch node.Op {
	case token.ADD: // +
		newOp = token.SUB
		typeStr = MutationArithmetic
	case token.SUB: // -
		newOp = token.ADD
		typeStr = MutationArithmetic
	case token.MUL: // *
		newOp = token.QUO
		typeStr = MutationArithmetic
	case token.QUO: // /
		newOp = token.MUL
		typeStr = MutationArithmetic
	case token.EQL: // ==
		newOp = token.NEQ
		typeStr = MutationCondition
	case token.NEQ: // !=
		newOp = token.EQL
		typeStr = MutationCondition
	case token.LSS: // <
		newOp = token.GEQ
		typeStr = MutationCondition
	case token.GTR: // >
		newOp = token.LEQ
		typeStr = MutationCondition
	case token.LEQ: // <=
		newOp = token.GTR
		typeStr = MutationCondition
	case token.GEQ: // >=
		newOp = token.LSS
		typeStr = MutationCondition
	case token.LAND: // &&
		newOp = token.LOR
		typeStr = MutationBoolean
	case token.LOR: // ||
		newOp = token.LAND
		typeStr = MutationBoolean
	default:
		return
	}

	pos := m.Fset.Position(node.Pos())

	mutation := Mutation{
		Type:     typeStr,
		File:     filename,
		Line:     pos.Line,
		Original: originalOp.String(),
		Mutated:  newOp.String(),
		Apply: func() {
			node.Op = newOp
		},
		Revert: func() {
			node.Op = originalOp
		},
	}
	*mutations = append(*mutations, mutation)
}

func (m *Mutator) mutateBooleanIdent(node *ast.Ident, filename string, mutations *[]Mutation) {
	if node.Name == "true" {
		pos := m.Fset.Position(node.Pos())
		mutation := Mutation{
			Type:     MutationBoolean,
			File:     filename,
			Line:     pos.Line,
			Original: "true",
			Mutated:  "false",
			Apply: func() {
				node.Name = "false"
			},
			Revert: func() {
				node.Name = "true"
			},
		}
		*mutations = append(*mutations, mutation)
	} else if node.Name == "false" {
		pos := m.Fset.Position(node.Pos())
		mutation := Mutation{
			Type:     MutationBoolean,
			File:     filename,
			Line:     pos.Line,
			Original: "false",
			Mutated:  "true",
			Apply: func() {
				node.Name = "true"
			},
			Revert: func() {
				node.Name = "false"
			},
		}
		*mutations = append(*mutations, mutation)
	}
}
