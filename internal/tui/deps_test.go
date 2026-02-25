package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDepsModel_Calculations(t *testing.T) {
	// Setup graph:
	// A -> B
	// B -> C, D
	// C -> (none)
	// D -> (none)
	// E -> A
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C", "D"},
		"C": {},
		"D": {},
		"E": {"A"},
	}

	model := NewDepsModel(graph)

	// Verify Metrics
	// Package A:
	// Ce (outgoing) = 1 (B)
	// Ca (incoming) = 1 (E)
	// I = 1 / (1 + 1) = 0.5
	mA, ok := model.metrics["A"]
	assert.True(t, ok)
	assert.Equal(t, 1, mA.Efferent, "A Ce mismatch")
	assert.Equal(t, 1, mA.Afferent, "A Ca mismatch")
	assert.InDelta(t, 0.5, mA.Instability, 0.001, "A Instability mismatch")

	// Package B:
	// Ce = 2 (C, D)
	// Ca = 1 (A)
	// I = 2 / (2 + 1) = 0.666...
	mB, ok := model.metrics["B"]
	assert.True(t, ok)
	assert.Equal(t, 2, mB.Efferent, "B Ce mismatch")
	assert.Equal(t, 1, mB.Afferent, "B Ca mismatch")
	assert.InDelta(t, 2.0/3.0, mB.Instability, 0.001, "B Instability mismatch")

	// Package C:
	// Ce = 0
	// Ca = 1 (B)
	// I = 0 / (0 + 1) = 0
	mC, ok := model.metrics["C"]
	assert.True(t, ok)
	assert.Equal(t, 0, mC.Efferent, "C Ce mismatch")
	assert.Equal(t, 1, mC.Afferent, "C Ca mismatch")
	assert.Equal(t, 0.0, mC.Instability, "C Instability mismatch")

	// Package E:
	// Ce = 1 (A)
	// Ca = 0
	// I = 1 / (1 + 0) = 1
	mE, ok := model.metrics["E"]
	assert.True(t, ok)
	assert.Equal(t, 1, mE.Efferent, "E Ce mismatch")
	assert.Equal(t, 0, mE.Afferent, "E Ca mismatch")
	assert.Equal(t, 1.0, mE.Instability, "E Instability mismatch")

	// Verify incoming graph logic
	assert.Contains(t, model.graph.Incoming["A"], "E")
	assert.Contains(t, model.graph.Incoming["B"], "A")
	assert.Contains(t, model.graph.Incoming["C"], "B")
}
