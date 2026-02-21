package tui

import (
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestFilterAndSortJobs(t *testing.T) {
	realNow := time.Now()
	jobs := []orchestrator.JobInfo{
		{
			ID:        "job-1",
			Summary:   "Fix bug in auth",
			Status:    "Completed",
			StartTime: realNow.Add(-10 * time.Minute),
			EndTime:   realNow.Add(-5 * time.Minute), // Duration: 5m fixed
		},
		{
			ID:        "job-2",
			Summary:   "Implement feature X",
			Status:    "Active",
			StartTime: realNow.Add(-2 * time.Minute), // Duration: ~2m running
			EndTime:   time.Time{},
		},
		{
			ID:        "job-3",
			Summary:   "Refactor database",
			Status:    "Failed",
			StartTime: realNow.Add(-20 * time.Minute),
			EndTime:   realNow.Add(-19 * time.Minute), // Duration: 1m fixed
		},
		{
			ID:        "job-4",
			Summary:   "Update docs",
			Status:    "Completed",
			StartTime: realNow.Add(-1 * time.Minute),
			EndTime:   realNow.Add(-30 * time.Second), // Duration: 30s fixed
		},
	}

	tests := []struct {
		name         string
		filterText   string
		filterStatus string
		sortBy       string
		expectedIDs  []string
	}{
		{
			name:         "No filters, sort newest (StartTime DESC)",
			filterText:   "",
			filterStatus: "All",
			sortBy:       "Newest",
			expectedIDs:  []string{"job-4", "job-2", "job-1", "job-3"},
		},
		{
			name:         "No filters, sort oldest (StartTime ASC)",
			filterText:   "",
			filterStatus: "All",
			sortBy:       "Oldest",
			expectedIDs:  []string{"job-3", "job-1", "job-2", "job-4"},
		},
		{
			name:         "Filter by text (ID)",
			filterText:   "job-1",
			filterStatus: "All",
			sortBy:       "Newest",
			expectedIDs:  []string{"job-1"},
		},
		{
			name:         "Filter by text (Summary partial)",
			filterText:   "feature",
			filterStatus: "All",
			sortBy:       "Newest",
			expectedIDs:  []string{"job-2"},
		},
		{
			name:         "Filter by text (Case insensitive)",
			filterText:   "FIX",
			filterStatus: "All",
			sortBy:       "Newest",
			expectedIDs:  []string{"job-1"},
		},
		{
			name:         "Filter by status (Completed)",
			filterText:   "",
			filterStatus: "Completed",
			sortBy:       "Newest",
			expectedIDs:  []string{"job-4", "job-1"},
		},
		{
			name:         "Filter by status (Active)",
			filterText:   "",
			filterStatus: "Active",
			sortBy:       "Newest",
			expectedIDs:  []string{"job-2"},
		},
		{
			name:         "Filter by status (Failed)",
			filterText:   "",
			filterStatus: "Failed",
			sortBy:       "Newest",
			expectedIDs:  []string{"job-3"},
		},
		{
			name:         "Sort by Duration (DESC)",
			filterText:   "",
			filterStatus: "All",
			sortBy:       "Duration",
			expectedIDs:  []string{"job-1", "job-2", "job-3", "job-4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterAndSortJobs(jobs, tt.filterText, tt.filterStatus, tt.sortBy)
			var ids []string
			for _, job := range result {
				ids = append(ids, job.ID)
			}
			assert.Equal(t, tt.expectedIDs, ids)
		})
	}
}
