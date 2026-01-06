package jira

import (
	"fmt"
	"sort"
	"strings"
)

// DependencyGraph represents the dependency relationship between tickets
type DependencyGraph struct {
	// BlockedBy maps a ticket key to the list of tickets that block it
	BlockedBy map[string][]string
	// Blocks maps a ticket key to the list of tickets it blocks
	Blocks map[string][]string
	// AllTickets contains all ticket keys in the graph
	AllTickets map[string]bool
}

// NewDependencyGraph creates a new empty dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		BlockedBy:  make(map[string][]string),
		Blocks:     make(map[string][]string),
		AllTickets: make(map[string]bool),
	}
}

// AddDependency records that 'blocker' blocks 'blocked'
func (g *DependencyGraph) AddDependency(blocker, blocked string) {
	g.BlockedBy[blocked] = append(g.BlockedBy[blocked], blocker)
	g.Blocks[blocker] = append(g.Blocks[blocker], blocked)
	g.AllTickets[blocker] = true
	g.AllTickets[blocked] = true
}

// AddTicket adds a ticket to the graph even if it has no dependencies
func (g *DependencyGraph) AddTicket(ticket string) {
	g.AllTickets[ticket] = true
}

// GetReadyTickets returns a list of tickets that have no active blockers from the provided completed set
func (g *DependencyGraph) GetReadyTickets(completed map[string]bool) []string {
	var ready []string
	for ticket := range g.AllTickets {
		if completed[ticket] {
			continue
		}

		isBlocked := false
		for _, blocker := range g.BlockedBy[ticket] {
			if !completed[blocker] {
				isBlocked = true
				break
			}
		}

		if !isBlocked {
			ready = append(ready, ticket)
		}
	}
	sort.Strings(ready)
	return ready
}

// TopologicalSort returns tickets sorted by dependency (blockers first)
// This is useful for sequential execution or planning.
// If a cycle is detected, it returns a partial list and an error.
func (g *DependencyGraph) TopologicalSort() ([]string, error) {
	visited := make(map[string]bool)
	tempMark := make(map[string]bool)
	var sorted []string
	var visit func(string) error

	visit = func(n string) error {
		if tempMark[n] {
			// Cycle detected
			return fmt.Errorf("cycle detected involving %s", n)
		}
		if visited[n] {
			return nil
		}
		tempMark[n] = true

		// Visit blockers first (dependencies)
		for _, blocker := range g.BlockedBy[n] {
			if err := visit(blocker); err != nil {
				return err
			}
		}

		tempMark[n] = false
		visited[n] = true
		sorted = append(sorted, n)
		return nil
	}

	keys := make([]string, 0, len(g.AllTickets))
	for k := range g.AllTickets {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Deterministic order

	for _, k := range keys {
		if !visited[k] {
			if err := visit(k); err != nil {
				return sorted, err
			}
		}
	}

	return sorted, nil
}

// BuildGraphFromIssues constructs a DependencyGraph from a list of Jira issues
// using the Client to parse blockers if necessary, or assuming blockers are pre-loaded.
func BuildGraphFromIssues(issues []map[string]interface{}, getBlockers func(map[string]interface{}) []string) *DependencyGraph {
	g := NewDependencyGraph()

	// Map issues by Key for easy lookup
	issueMap := make(map[string]map[string]interface{})
	for _, issue := range issues {
		key, _ := issue["key"].(string)
		if key != "" {
			issueMap[key] = issue
			g.AddTicket(key)
		}
	}

	for key, issue := range issueMap {
		// Get blockers (e.g. "KEY (Status)")
		rawBlockers := getBlockers(issue)
		for _, rb := range rawBlockers {
			// Parse "KEY (Status)"
			parts := strings.Split(rb, " (")
			if len(parts) > 0 {
				blockerKey := parts[0]

				// Only consider blockers that are in our scope (issues list)
				if _, exists := issueMap[blockerKey]; exists {
					// Ignore self-references
					if blockerKey != key {
						g.AddDependency(blockerKey, key)
					}
				}
			}
		}
	}

	return g
}

// ResolveDependencies sorts issues by dependency order (blockers first).
// It acts as a helper wrapper around DependencyGraph.TopologicalSort.
// fetchBlockers is a function that returns a list of blocker keys for a given issue.
func ResolveDependencies(issues []map[string]interface{}, fetchBlockers func(map[string]interface{}) ([]string, error)) ([]map[string]interface{}, error) {
	// Wrapper to match BuildGraphFromIssues expectation (masking error for graph build, but we can handle it)

	// First, map issues by key for quick retrieval
	issueMap := make(map[string]map[string]interface{})
	for _, issue := range issues {
		key, _ := issue["key"].(string)
		if key != "" {
			issueMap[key] = issue
		}
	}

	// Create Graph manually to handle errors from fetchBlockers
	g := NewDependencyGraph()
	for _, issue := range issues {
		key, _ := issue["key"].(string)
		if key == "" {
			continue // Skip issues without keys
		}
		g.AddTicket(key)

		blockers, err := fetchBlockers(issue)
		if err != nil {
			return nil, err
		}

		for _, blockerKey := range blockers {
			// Ignore self-references
			if blockerKey == key {
				continue
			}
			// Only consider blockers that are in the list
			if _, exists := issueMap[blockerKey]; exists {
				g.AddDependency(blockerKey, key)
			}
		}
	}

	sortedKeys, err := g.TopologicalSort()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(sortedKeys))
	for _, k := range sortedKeys {
		if i, ok := issueMap[k]; ok {
			result = append(result, i)
		}
	}

	return result, nil
}
