package benchmark

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompare(t *testing.T) {
	prev := Run{
		Results: []Result{
			{Name: "B1", NsPerOp: 100, BytesPerOp: 50},
			{Name: "B2", NsPerOp: 200},
		},
	}
	curr := Run{
		Results: []Result{
			{Name: "B1", NsPerOp: 110, BytesPerOp: 40}, // 10% slower, 20% less memory
			{Name: "B3", NsPerOp: 300},                 // New
		},
	}

	comps := Compare(prev, curr)

	assert.Len(t, comps, 1) // Only B1 matches

	c := comps[0]
	assert.Equal(t, "B1", c.Name)
	assert.InDelta(t, 10.0, c.NsPerOpDiff, 0.01)
	assert.InDelta(t, -20.0, c.BytesPerOpDiff, 0.01)
}
