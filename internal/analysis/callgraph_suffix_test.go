package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveExternalCall_Ambiguity(t *testing.T) {
	// Setup a graph with two nodes that have suffix ambiguity
	cg := &CallGraph{
		Nodes: make(map[string]*CallGraphNode),
	}

	// Node 1: "utils"
	cg.Nodes["utils.Func"] = &CallGraphNode{
		ID:      "utils.Func",
		Package: "utils",
		Name:    "Func",
	}

	// Node 2: "autils" (ends with "utils")
	cg.Nodes["autils.Func"] = &CallGraphNode{
		ID:      "autils.Func",
		Package: "autils",
		Name:    "Func",
	}

	// Case 1: Import "my/autils". Should match "autils.Func", NOT "utils.Func".
	// Current buggy implementation might return "utils.Func" depending on iteration order,
	// because "my/autils" ends with "utils".

	// We want to force a failure if it picks "utils".
	// Since iteration is random, we can't guarantee failure unless we loop or enforce deterministic mock (which we can't easily do for map iteration).
	// But we can check if it returns the WRONG one.

	// Actually, "my/autils" ends with "autils" AND "utils".
	// If it picks "utils", it's wrong because "autils" is a better match?
	// Or simply because "utils" is not a path suffix of "autils" (missing separator).

	id := resolveExternalCall(cg, "my/autils", "Func")
	assert.Equal(t, "autils.Func", id, "Should resolve to autils.Func, not utils.Func")

	// Case 2: Partial match that should fail
	// Import "my/myutils". We have node "utils".
	// "my/myutils" ends with "utils".
	// But it should NOT match because "utils" is part of "myutils".
	cg.Nodes["utils.Func2"] = &CallGraphNode{
		ID:      "utils.Func2",
		Package: "utils",
		Name:    "Func2",
	}

	id2 := resolveExternalCall(cg, "my/myutils", "Func2")
	assert.Equal(t, "", id2, "Should not resolve partial suffix 'utils' in 'myutils'")
}
