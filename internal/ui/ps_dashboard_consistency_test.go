package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPsDashboard_Consistency_Columns(t *testing.T) {
	m := NewPsDashboardModel(false)
	cols := m.table.Columns()

	hasCPU := false
	hasMEM := false

	for _, col := range cols {
		if col.Title == "CPU" {
			hasCPU = true
		}
		if col.Title == "MEM" {
			hasMEM = true
		}
	}

	assert.True(t, hasCPU, "Table should have CPU column for parity with CLI")
	assert.True(t, hasMEM, "Table should have MEM column for parity with CLI")
}

func TestPsDashboard_Consistency_CostColumns(t *testing.T) {
	m := NewPsDashboardModel(true)
	cols := m.table.Columns()

	hasCost := false
	hasTokens := false

	for _, col := range cols {
		if col.Title == "COST" {
			hasCost = true
		}
		if col.Title == "TOKENS" {
			hasTokens = true
		}
	}

	assert.True(t, hasCost, "Table should have COST column for parity with CLI")
	assert.True(t, hasTokens, "Table should have TOKENS column for parity with CLI")
}
