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
	// Helper to add a mutation
	addMutation := func(newOp token.Token, typeStr MutationType) {
		pos := m.Fset.Position(node.Pos())
		originalOp := node.Op

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

	switch node.Op {
	case token.ADD: // +
		addMutation(token.SUB, MutationArithmetic)
	case token.SUB: // -
		addMutation(token.ADD, MutationArithmetic)
	case token.MUL: // *
		addMutation(token.QUO, MutationArithmetic)
	case token.QUO: // /
		addMutation(token.MUL, MutationArithmetic)
	case token.EQL: // ==
		addMutation(token.NEQ, MutationCondition)
	case token.NEQ: // !=
		addMutation(token.EQL, MutationCondition)
	case token.LSS: // <
		addMutation(token.GEQ, MutationCondition) // < to >= (Invert)
		addMutation(token.LEQ, MutationCondition) // < to <= (Boundary)
	case token.GTR: // >
		addMutation(token.LEQ, MutationCondition) // > to <= (Invert)
		addMutation(token.GEQ, MutationCondition) // > to >= (Boundary)
	case token.LEQ: // <=
		addMutation(token.GTR, MutationCondition) // <= to > (Invert)
		addMutation(token.LSS, MutationCondition) // <= to < (Boundary)
	case token.GEQ: // >=
		addMutation(token.LSS, MutationCondition) // >= to < (Invert)
		addMutation(token.GTR, MutationCondition) // >= to > (Boundary)
	case token.LAND: // &&
		addMutation(token.LOR, MutationBoolean)
	case token.LOR: // ||
		addMutation(token.LAND, MutationBoolean)
	}
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
