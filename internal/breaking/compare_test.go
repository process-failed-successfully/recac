package breaking

import (
	"testing"
)

func TestCompare(t *testing.T) {
	oldAPI := NewAPI()
	oldAPI.Identifiers["p.FuncA"] = "func()"
	oldAPI.Identifiers["p.FuncB"] = "func(int)"
	oldAPI.Identifiers["p.TypeA"] = "struct { F int }"

	newAPI := NewAPI()
	newAPI.Identifiers["p.FuncA"] = "func()"       // Same
	newAPI.Identifiers["p.FuncB"] = "func(string)" // Changed
	// TypeA removed
	newAPI.Identifiers["p.FuncC"] = "func()" // Added

	changes := Compare(oldAPI, newAPI)

	if len(changes) != 3 {
		t.Fatalf("Expected 3 changes, got %d", len(changes))
	}

	// Order is identifier sorted: FuncB, FuncC, TypeA

	// 1. FuncB Changed
	if changes[0].Identifier != "p.FuncB" || changes[0].Type != ChangeChanged {
		t.Errorf("Expected FuncB Changed, got %v", changes[0])
	}

	// 2. FuncC Added
	if changes[1].Identifier != "p.FuncC" || changes[1].Type != ChangeAdded {
		t.Errorf("Expected FuncC Added, got %v", changes[1])
	}

	// 3. TypeA Removed
	if changes[2].Identifier != "p.TypeA" || changes[2].Type != ChangeRemoved {
		t.Errorf("Expected TypeA Removed, got %v", changes[2])
	}
}
