package orchestrator

import (
	"log/slog"
	"sort"
	"strings"
)

type DiagnosticReport struct {
	DeadlockedJobs   []DeadlockedJob   `json:"deadlocked_jobs"`
	UnresolvableJobs []UnresolvableJob `json:"unresolvable_jobs"`
}

type DeadlockedJob struct {
	JobID string   `json:"job_id"`
	Cycle []string `json:"cycle"`
}

type UnresolvableJob struct {
	JobID          string   `json:"job_id"`
	MissingDeps    []string `json:"missing_dependencies"`
	FailedDeps     []string `json:"failed_dependencies"`
	CanceledDeps   []string `json:"canceled_dependencies"`
}

// Diagnose analyzes the current state of pending jobs to find unresolvable dependencies and deadlocks (circular dependencies).
func (o *Orchestrator) Diagnose(logger *slog.Logger) (DiagnosticReport, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var report DiagnosticReport

	// Build a fast lookup for job status
	jobStatusMap := make(map[string]string)
	for id, job := range o.activeJobs {
		jobStatusMap[id] = job.Status
	}
	for id, job := range o.pendingJobs {
		jobStatusMap[id] = job.Status
	}
	for _, job := range o.completedJobs {
		jobStatusMap[job.ID] = job.Status
	}
	// Add persistence if any
	if o.Persistence != nil {
		if jobs, err := o.Persistence.GetJobs(10000); err == nil {
			for _, job := range jobs {
				// Only set if not already in memory (memory is newer)
				if _, exists := jobStatusMap[job.ID]; !exists {
					jobStatusMap[job.ID] = job.Status
				}
			}
		}
	}

	// 1. Find unresolvable jobs
	for id, job := range o.pendingJobs {
		var missing []string
		var failed []string
		var canceled []string

		for _, dep := range job.WorkItem.DependsOn {
			status, exists := jobStatusMap[dep]
			if !exists {
				missing = append(missing, dep)
			} else if status == "Failed" {
				failed = append(failed, dep)
			} else if status == "Canceled" {
				canceled = append(canceled, dep)
			}
		}

		if len(missing) > 0 || len(failed) > 0 || len(canceled) > 0 {
			report.UnresolvableJobs = append(report.UnresolvableJobs, UnresolvableJob{
				JobID:        id,
				MissingDeps:  missing,
				FailedDeps:   failed,
				CanceledDeps: canceled,
			})
		}
	}

	// 2. Find deadlocks (circular dependencies) among pending and active jobs
	// We'll use a simple DFS cycle detection
	visited := make(map[string]int) // 0: unvisited, 1: visiting, 2: visited
	var path []string

	// adjacency list for cycle detection
	adj := make(map[string][]string)
	for id, job := range o.pendingJobs {
		adj[id] = job.WorkItem.DependsOn
	}
	for id, job := range o.activeJobs {
		adj[id] = job.WorkItem.DependsOn
	}

	var dfs func(node string)
	dfs = func(node string) {
		visited[node] = 1
		path = append(path, node)

		for _, neighbor := range adj[node] {
			if visited[neighbor] == 1 {
				// Cycle detected
				// Extract the cycle from path
				cycleStart := -1
				for i, n := range path {
					if n == neighbor {
						cycleStart = i
						break
					}
				}
				if cycleStart != -1 {
					cycle := make([]string, len(path)-cycleStart)
					copy(cycle, path[cycleStart:])
					// Add the closing node to make the cycle clear
					cycle = append(cycle, neighbor)

					report.DeadlockedJobs = append(report.DeadlockedJobs, DeadlockedJob{
						JobID: node,
						Cycle: cycle,
					})
				}
			} else if visited[neighbor] == 0 {
				dfs(neighbor)
			}
		}

		path = path[:len(path)-1]
		visited[node] = 2
	}

	// Make execution deterministic by sorting keys
	var keys []string
	for id := range adj {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	for _, id := range keys {
		if visited[id] == 0 {
			dfs(id)
		}
	}

	// Deduplicate deadlocks based on the set of nodes in the cycle
	uniqueCycles := make(map[string]DeadlockedJob)
	for _, d := range report.DeadlockedJobs {
		nodes := make([]string, len(d.Cycle)-1)
		copy(nodes, d.Cycle[:len(d.Cycle)-1])
		sort.Strings(nodes)

		sig := strings.Join(nodes, "|")
		uniqueCycles[sig] = d
	}

	report.DeadlockedJobs = nil
	for _, d := range uniqueCycles {
		report.DeadlockedJobs = append(report.DeadlockedJobs, d)
	}

	// Sort results for deterministic testing
	sort.Slice(report.UnresolvableJobs, func(i, j int) bool {
		return report.UnresolvableJobs[i].JobID < report.UnresolvableJobs[j].JobID
	})
	sort.Slice(report.DeadlockedJobs, func(i, j int) bool {
		return report.DeadlockedJobs[i].JobID < report.DeadlockedJobs[j].JobID
	})

	if logger != nil {
		logger.Info("Diagnose completed", "unresolvable", len(report.UnresolvableJobs), "deadlocked", len(report.DeadlockedJobs))
	}

	return report, nil
}
