package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestPrintEvolutionReport(t *testing.T) {
	metrics := []EvolutionMetric{
		{
			Date:       "2023-01-01",
			Commit:     "abc123456789",
			LOC:        100,
			Complexity: 10,
			TODOs:      5,
		},
		{
			Date:       "2023-01-02",
			Commit:     "def987654321",
			LOC:        150,
			Complexity: 15,
			TODOs:      3,
		},
	}

	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	printEvolutionReport(cmd, metrics)

	output := buf.String()
	assert.Contains(t, output, "EVOLUTION REPORT")
	assert.Contains(t, output, "DATE")
	assert.Contains(t, output, "COMMIT")
	assert.Contains(t, output, "LOC")
	assert.Contains(t, output, "COMPLEXITY")
	assert.Contains(t, output, "TODOs")

	// Check data rows
	assert.Contains(t, output, "2023-01-01")
	assert.Contains(t, output, "abc1234") // Short hash
	assert.Contains(t, output, "100")
	assert.Contains(t, output, "10")
	assert.Contains(t, output, "5")

	assert.Contains(t, output, "2023-01-02")
	assert.Contains(t, output, "def9876") // Short hash
	assert.Contains(t, output, "150")
	assert.Contains(t, output, "15")
	assert.Contains(t, output, "3")

	// Check trends
	assert.Contains(t, output, "TRENDS:")
	assert.Contains(t, output, "[LOC]")
	assert.Contains(t, output, "100 -> 150 (+50)")
}

func TestPrintTrend(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	metrics := []EvolutionMetric{
		{LOC: 100},
		{LOC: 200},
		{LOC: 50},
	}

	printTrend(cmd, "LOC", metrics, func(m EvolutionMetric) int { return m.LOC })

	output := buf.String()
	assert.Contains(t, output, "[LOC] 100 -> 50 (-50)")
}

func TestPrintTrend_Flat(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	metrics := []EvolutionMetric{
		{LOC: 0},
		{LOC: 0},
	}

	printTrend(cmd, "LOC", metrics, func(m EvolutionMetric) int { return m.LOC })

	output := buf.String()
	assert.Contains(t, output, "[LOC] Flat (0)")
}
