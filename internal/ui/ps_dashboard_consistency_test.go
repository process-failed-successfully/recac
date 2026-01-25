package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPsDashboard_Consistency_Columns(t *testing.T) {
	m := NewPsDashboardModel()
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
