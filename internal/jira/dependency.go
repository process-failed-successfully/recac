package jira

import (
	"fmt"
	"sort"
)

// ResolveDependencies sorts a list of issues based on their 'Blocks' dependencies.
// It returns a topologically sorted slice where blocked issues appear after their blockers.
// It detects and returns an error for circular dependencies.
// depsFetcher is a function that returns a list of issue keys that block the given issue.
func ResolveDependencies(issues []map[string]interface{}, depsFetcher func(map[string]interface{}) ([]string, error)) ([]map[string]interface{}, error) {
	// Map issues by Key for easy lookup
	issueMap := make(map[string]map[string]interface{})
	for _, issue := range issues {
		if key, ok := issue["key"].(string); ok {
			issueMap[key] = issue
		}
	}

	// Build Dependency Graph
	// Graph: Blocked -> [Blocker1, Blocker2]
	// We want to process blockers first. So if A blocks B, B depends on A.
	// B -> A
	// Link direction: Inward "is blocked by" means Current "is blocked by" Other.
	// So Dependency(Current) = {Other}

	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	// Initialize inDegree for all issues in the set (to ensure they are in the graph)
	for key := range issueMap {
		inDegree[key] = 0
	}

	for _, issue := range issues {
		key, _ := issue["key"].(string)

		// Get dependencies (issues that block this one)
		deps, err := depsFetcher(issue) // these are keys of issues that BLOCK the current issue
		if err != nil {
			return nil, fmt.Errorf("failed to get links for issue %s: %w", key, err)
		}

		for _, depKey := range deps {
			// Only consider dependencies that are in our provided list of issues
			if _, exists := issueMap[depKey]; exists {
				// depKey BLOCKS key.
				// Meaning key DEPENDS ON depKey.
				// For topological sort (Execution Order: Blocker -> Blocked):
				// Edge: depKey -> key

				graph[depKey] = append(graph[depKey], key)
				inDegree[key]++
			}
		}
	}

	// Kahn's Algorithm for Topological Sort
	queue := []string{}
	for key, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, key)
		}
	}

	// Sort queue for deterministic output
	sort.Strings(queue)

	var result []map[string]interface{}

	for len(queue) > 0 {
		// Pop front
		u := queue[0]
		queue = queue[1:]

		result = append(result, issueMap[u])

		for _, v := range graph[u] {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}

		// Sort queue again to maintain determinism at each level
		sort.Strings(queue)
	}

	if len(result) != len(issues) {
		return nil, fmt.Errorf("circular dependency detected or dependency check failure")
	}

	return result, nil
}
