package main

import (
	"testing"

	"recac/internal/architecture"
	"github.com/stretchr/testify/assert"
)

func TestBuildTicketTreeFromArch(t *testing.T) {
	arch := architecture.SystemArchitecture{
		SystemName: "TestSystem",
		Components: []architecture.Component{
			{
				ID: "Comp1",
				Description: "Component 1",
				Type: "Service",
				ImplementationSteps: []string{"Step 1"},
				Functions: []architecture.Function{
					{Name: "Func1", Args: "a", Return: "b", Requirements: []string{"req1"}},
				},
				Consumes: []architecture.Input{
					{Type: "Input1", Source: "Src", Schema: "{}"},
				},
				Produces: []architecture.Output{
					{Type: "Output1", Schema: "{}"},
				},
			},
		},
	}

	nodes := buildTicketTreeFromArch(arch, "http://repo.com", "spec")

	assert.Len(t, nodes, 1) // Root Epic
	root := nodes[0]
	assert.Equal(t, "ID:[SYSTEM] TestSystem Architecture", root.Title)
	assert.Equal(t, "Epic", root.Type)

	assert.Len(t, root.Children, 1) // Comp1 Story
	comp := root.Children[0]
	assert.Equal(t, "ID:[Comp1] [Service] Comp1", comp.Title)
	assert.Equal(t, "Story", comp.Type)

	// Check Children of Comp1 (1 Step, 1 Func, 1 Input, 1 Output = 4 Subtasks)
	assert.Len(t, comp.Children, 4)

	// Verify types
	for _, child := range comp.Children {
		assert.Equal(t, "Subtask", child.Type)
	}
}
