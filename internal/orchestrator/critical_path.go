package orchestrator

import (
	"time"
)

// CalculateCriticalPath identifies the critical path in the job execution graph.
// It returns the sequence of jobs that constitute the longest path (by duration)
// and the total duration of that path.
func CalculateCriticalPath(jobs []JobInfo) ([]JobInfo, time.Duration) {
	if len(jobs) == 0 {
		return nil, 0
	}

	// 1. Filter out jobs that haven't started or duration cannot be determined
	// and map ID to JobInfo and its duration.
	validJobs := make(map[string]JobInfo)
	durations := make(map[string]time.Duration)

	for _, j := range jobs {
		if j.StartTime.IsZero() {
			continue // Skip unstarted jobs
		}

		end := j.EndTime
		if end.IsZero() {
			// If running, use current time as end time for estimation
			end = time.Now()
		}

		dur := end.Sub(j.StartTime)
		if dur < 0 {
			dur = 0
		}

		validJobs[j.ID] = j
		durations[j.ID] = dur
	}

	if len(validJobs) == 0 {
		return nil, 0
	}

	// 2. Build Adjacency List
	// adj[u] = list of v where v depends on u (u -> v)
	// We only consider dependencies that exist in validJobs
	adj := make(map[string][]string)
	inDegree := make(map[string]int)

	// Initialize inDegree for all valid jobs
	for id := range validJobs {
		inDegree[id] = 0
	}

	for id, job := range validJobs {
		for _, depID := range job.WorkItem.DependsOn {
			if _, exists := validJobs[depID]; exists {
				adj[depID] = append(adj[depID], id)
				inDegree[id]++
			}
		}
	}

	// 3. Topological Sort (Kahn's Algorithm)
	var topoOrder []string
	var queue []string

	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		topoOrder = append(topoOrder, curr)

		for _, neighbor := range adj[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// If there's a cycle, topoOrder will not contain all validJobs.
	// We'll just work with whatever we successfully sorted to be robust.

	// 4. Dynamic Programming to find Longest Path
	// dist[id] = max duration ending at 'id'
	// parent[id] = predecessor in the longest path
	dist := make(map[string]time.Duration)
	parent := make(map[string]string)

	// Initialize distances
	for id := range validJobs {
		dist[id] = durations[id]
		parent[id] = "" // Empty string means no parent
	}

	// Process in topological order
	for _, u := range topoOrder {
		for _, v := range adj[u] {
			// Relax edge u -> v
			if dist[u]+durations[v] > dist[v] {
				dist[v] = dist[u] + durations[v]
				parent[v] = u
			}
		}
	}

	// 5. Find the endpoint of the critical path (max dist)
	var maxDist time.Duration
	var endNode string

	for id, d := range dist {
		if d > maxDist {
			maxDist = d
			endNode = id
		}
	}

	if endNode == "" {
		return nil, 0
	}

	// 6. Backtrack to build the path
	var path []JobInfo
	curr := endNode
	for curr != "" {
		path = append(path, validJobs[curr])
		curr = parent[curr]
	}

	// Reverse the path so it goes from start to end
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path, maxDist
}
