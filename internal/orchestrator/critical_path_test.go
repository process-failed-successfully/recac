package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateCriticalPath(t *testing.T) {
	now := time.Now()

	// Helper to create a job
	makeJob := func(id string, start, end time.Time, deps []string) JobInfo {
		return JobInfo{
			ID:        id,
			StartTime: start,
			EndTime:   end,
			WorkItem: WorkItem{
				ID:        id,
				DependsOn: deps,
			},
		}
	}

	tests := []struct {
		name          string
		jobs          []JobInfo
		expectedPath  []string // Sequence of IDs
		expectedTotal time.Duration
	}{
		{
			name:          "Empty input",
			jobs:          []JobInfo{},
			expectedPath:  nil,
			expectedTotal: 0,
		},
		{
			name: "Linear path",
			jobs: []JobInfo{
				makeJob("A", now, now.Add(10*time.Second), nil),
				makeJob("B", now.Add(10*time.Second), now.Add(20*time.Second), []string{"A"}),
				makeJob("C", now.Add(20*time.Second), now.Add(25*time.Second), []string{"B"}),
			},
			expectedPath:  []string{"A", "B", "C"},
			expectedTotal: 25 * time.Second, // 10s + 10s + 5s
		},
		{
			name: "Parallel paths - B is longer",
			jobs: []JobInfo{
				makeJob("Start", now, now.Add(5*time.Second), nil),
				// Branch 1: A (5s)
				makeJob("A", now.Add(5*time.Second), now.Add(10*time.Second), []string{"Start"}),
				// Branch 2: B (15s) -> this should be the critical path
				makeJob("B", now.Add(5*time.Second), now.Add(20*time.Second), []string{"Start"}),
				makeJob("End", now.Add(20*time.Second), now.Add(22*time.Second), []string{"A", "B"}),
			},
			expectedPath:  []string{"Start", "B", "End"},
			expectedTotal: 22 * time.Second, // 5s + 15s + 2s
		},
		{
			name: "Parallel paths - A is longer",
			jobs: []JobInfo{
				makeJob("Start", now, now.Add(5*time.Second), nil),
				// Branch 1: A (20s) -> critical path
				makeJob("A", now.Add(5*time.Second), now.Add(25*time.Second), []string{"Start"}),
				// Branch 2: B (5s)
				makeJob("B", now.Add(5*time.Second), now.Add(10*time.Second), []string{"Start"}),
				makeJob("End", now.Add(25*time.Second), now.Add(27*time.Second), []string{"A", "B"}),
			},
			expectedPath:  []string{"Start", "A", "End"},
			expectedTotal: 27 * time.Second, // 5s + 20s + 2s
		},
		{
			name: "Unconnected components",
			jobs: []JobInfo{
				// Component 1: 10s total
				makeJob("A", now, now.Add(5*time.Second), nil),
				makeJob("B", now.Add(5*time.Second), now.Add(10*time.Second), []string{"A"}),
				// Component 2: 15s total -> should be chosen as critical path
				makeJob("X", now, now.Add(10*time.Second), nil),
				makeJob("Y", now.Add(10*time.Second), now.Add(15*time.Second), []string{"X"}),
			},
			expectedPath:  []string{"X", "Y"},
			expectedTotal: 15 * time.Second,
		},
		{
			name: "Active job without EndTime",
			jobs: []JobInfo{
				makeJob("A", now.Add(-10*time.Second), time.Time{}, nil), // Running for 10s
			},
			expectedPath:  []string{"A"},
			expectedTotal: 10 * time.Second, // approx, we'll assert within a margin
		},
		{
			name: "Unstarted jobs ignored",
			jobs: []JobInfo{
				makeJob("A", now, now.Add(5*time.Second), nil),
				makeJob("Unstarted", time.Time{}, time.Time{}, []string{"A"}),
			},
			expectedPath:  []string{"A"},
			expectedTotal: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, total := CalculateCriticalPath(tt.jobs)

			var pathIDs []string
			for _, j := range path {
				pathIDs = append(pathIDs, j.ID)
			}

			assert.Equal(t, tt.expectedPath, pathIDs)

			if tt.name == "Active job without EndTime" {
				// Duration will be slightly more than 10s because of execution time
				assert.GreaterOrEqual(t, float64(total), float64(10*time.Second))
				assert.Less(t, float64(total), float64(11*time.Second))
			} else {
				assert.Equal(t, tt.expectedTotal, total)
			}
		})
	}
}
