package visualize

import (
	"recac/internal/runner"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"foo bar", "foo_bar"},
		{"foo-bar", "foo_bar"},
		{"foo.bar", "foo_bar"},
		{"foo bar-baz.qux", "foo_bar_baz_qux"},
	}

	for _, tt := range tests {
		result := SanitizeID(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestGenerateMermaid(t *testing.T) {
	g := runner.NewTaskGraph()
	g.Nodes["A"] = &runner.TaskNode{ID: "A", Name: "Start Node", Status: runner.TaskDone}
	g.Nodes["B"] = &runner.TaskNode{ID: "B", Name: "Process", Status: runner.TaskInProgress}
	g.Nodes["C"] = &runner.TaskNode{ID: "C", Name: "End", Status: runner.TaskReady}

	g.Nodes["A"].Dependencies = []string{}
	g.Nodes["B"].Dependencies = []string{"A"}
	g.Nodes["C"].Dependencies = []string{"B"}

	mermaid := GenerateMermaid(g)

	assert.Contains(t, mermaid, "graph TD")
	assert.Contains(t, mermaid, `A["Start Node"]:::done`)
	assert.Contains(t, mermaid, `B["Process"]:::inprogress`)
	assert.Contains(t, mermaid, `C["End"]:::ready`)
	assert.Contains(t, mermaid, "A --> B")
	assert.Contains(t, mermaid, "B --> C")

	// Check deterministic order (A, B, C sorted by ID)
	lines := strings.Split(mermaid, "\n")
	var nodeLines []string
	for _, l := range lines {
		if strings.Contains(l, "[") {
			nodeLines = append(nodeLines, l)
		}
	}
	// A comes before B, B before C
	aIdx := -1
	bIdx := -1
	cIdx := -1
	for i, l := range nodeLines {
		if strings.Contains(l, `A["`) {
			aIdx = i
		} else if strings.Contains(l, `B["`) {
			bIdx = i
		} else if strings.Contains(l, `C["`) {
			cIdx = i
		}
	}
	assert.True(t, aIdx < bIdx, "A should follow B")
	assert.True(t, bIdx < cIdx, "B should follow C")
}
