package tui

import (
	"recac/internal/orchestrator"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRenderCriticalPath(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	end := start.Add(10 * time.Minute)

	tests := []struct {
		name         string
		jobs         []orchestrator.JobInfo
		totalDur     time.Duration
		expectedStrs []string
	}{
		{
			name:     "empty path",
			jobs:     nil,
			totalDur: 0,
			expectedStrs: []string{
				"No jobs available for critical path analysis",
			},
		},
		{
			name: "valid path",
			jobs: []orchestrator.JobInfo{
				{
					ID:        "job-1",
					Summary:   "Compile Code",
					StartTime: start,
					EndTime:   end,
				},
				{
					ID:        "job-2",
					Summary:   "Run Tests",
					StartTime: end,
					EndTime:   end.Add(20 * time.Minute),
				},
			},
			totalDur: 30 * time.Minute,
			expectedStrs: []string{
				"Critical Path Analysis",
				"job-1 [10m0s]",
				"job-2 [20m0s]",
				"Total Critical Duration:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := renderCriticalPath(tt.jobs, tt.totalDur)
			for _, exp := range tt.expectedStrs {
				assert.Contains(t, output, exp)
			}
		})
	}
}
