package tui

import (
	"strings"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestRenderTree(t *testing.T) {
	tests := []struct {
		name     string
		jobs     []orchestrator.JobInfo
		expected []string
	}{
		{
			name:     "Empty list",
			jobs:     []orchestrator.JobInfo{},
			expected: []string{"No jobs found."},
		},
		{
			name: "Single node",
			jobs: []orchestrator.JobInfo{
				{
					ID:      "JOB-1",
					Summary: "First job",
					Status:  "Completed",
				},
			},
			expected: []string{
				"Job Dependency Tree",
				"JOB-1",
				"Completed",
				"First job",
				"└──",
			},
		},
		{
			name: "Linear dependency",
			jobs: []orchestrator.JobInfo{
				{
					ID:      "JOB-1",
					Summary: "First job",
					Status:  "Completed",
				},
				{
					ID:      "JOB-2",
					Summary: "Second job",
					Status:  "Pending",
					WorkItem: orchestrator.WorkItem{
						DependsOn: []string{"JOB-1"},
					},
				},
			},
			expected: []string{
				"JOB-1",
				"Completed",
				"└──",
				"JOB-2",
				"Pending",
				"└──",
			},
		},
		{
			name: "Branching dependency",
			jobs: []orchestrator.JobInfo{
				{
					ID:      "JOB-1",
					Summary: "Root job",
					Status:  "Completed",
				},
				{
					ID:      "JOB-2",
					Summary: "Child A",
					Status:  "Active",
					WorkItem: orchestrator.WorkItem{
						DependsOn: []string{"JOB-1"},
					},
				},
				{
					ID:      "JOB-3",
					Summary: "Child B",
					Status:  "Failed",
					WorkItem: orchestrator.WorkItem{
						DependsOn: []string{"JOB-1"},
					},
				},
			},
			expected: []string{
				"JOB-1",
				"Completed",
				"├──", // or └── depending on order, but it should branch
				"JOB-2",
				"Active",
				"JOB-3",
				"Failed",
			},
		},
		{
			name: "Multiple roots",
			jobs: []orchestrator.JobInfo{
				{
					ID:      "JOB-1",
					Summary: "Root 1",
					Status:  "Completed",
				},
				{
					ID:      "JOB-2",
					Summary: "Root 2",
					Status:  "Pending",
				},
			},
			expected: []string{
				"JOB-1",
				"JOB-2",
				"└──", // Both should be rendered as root elements ending with └──
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := renderTree(tt.jobs)

			// We check for expected substrings rather than exact string match
			// due to lipgloss ANSI escape codes that will be in the output.
			for _, expectedStr := range tt.expected {
				assert.Contains(t, output, expectedStr, "Output should contain: %s", expectedStr)
			}
		})
	}
}

func TestRenderTree_LimitString(t *testing.T) {
	// A job with a very long summary to test limitString inside renderNode
	jobs := []orchestrator.JobInfo{
		{
			ID:      "JOB-LONG",
			Summary: strings.Repeat("A", 100),
			Status:  "Pending",
		},
	}

	output := renderTree(jobs)

	// Since limitString restricts to 40 chars and adds '...', the summary length in output will be 43
	expectedSummary := strings.Repeat("A", 40) + "..."
	assert.Contains(t, output, expectedSummary)

	// Ensure the full 100 character string is NOT in the output
	assert.NotContains(t, output, strings.Repeat("A", 100))
}
