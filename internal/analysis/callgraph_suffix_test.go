package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveExternalCall_PartialSuffix(t *testing.T) {
	// Setup nodes
	// We have a node in package "utils"
	nodes := []*CallGraphNode{
		{
			ID:      "utils.Func",
			Package: "utils",
			Name:    "Func",
		},
	}

	// We have an import "my/autils"
	// It should NOT resolve to "utils.Func" just because "autils" ends with "utils"

	target := resolveExternalCall(nodes, "my/autils", "Func")
	assert.Equal(t, "", target, "Should not resolve partial suffix match")

	// But "my/utils" SHOULD resolve
	target2 := resolveExternalCall(nodes, "my/utils", "Func")
	assert.Equal(t, "utils.Func", target2, "Should resolve exact path component match")
}
