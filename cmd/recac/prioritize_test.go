package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
)

// To support testing without importing root.go, we avoid relying on `rootCmd` being initialized for us.
// The code in `prioritize.go` registers with `rootCmd`.
// If we run `go test cmd/recac/prioritize.go cmd/recac/prioritize_test.go`, we need `rootCmd`.
// We will simply define a dummy `rootCmd` here if it's not defined, but since we can't redefine a variable that might be defined in `root.go` if we were to include it,
// AND we can't do conditional compilation in the same package easily...
//
// The best approach for THIS test file is to assume `root.go` is NOT present (as we invoked `go test file1 file2`)
// and define the missing variable `rootCmd` here.
// But `prioritize.go` uses `rootCmd`. So if we define it here, it works.
// BUT if we run `go test ./cmd/recac` (the standard way), `rootCmd` is defined in `root.go`, and we get a collision.
//
// So, we cannot define `rootCmd` here if we want to support standard testing.
// However, I previously tried running `go test file1 file2` and it failed because `rootCmd` was undefined.
// This implies I MUST include `root.go` OR define it.
//
// Strategy: I will rely on standard package testing `go test ./cmd/recac`.
// If I want to verify just this file quickly, I can't easily without mocking `root.go`.
//
// BUT, for the purpose of this task, I will rename this test file to `prioritize_standalone_test.go` and add `// +build ignore`? No.
//
// I will just invoke `go test ./cmd/recac -v -run TestPrioritize` which builds everything.
// This assumes the rest of the package compiles. I verified via `list_files` that `root.go` exists.

func TestPrioritizeCmd_EndToEnd(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "recac-prioritize-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	features := []db.Feature{
		{ID: "A", Priority: "Low", Description: "Task A", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"B"}}},
		{ID: "B", Priority: "High", Description: "Task B", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{}}},
		{ID: "C", Priority: "Medium", Description: "Task C", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"B"}}},
		{ID: "D", Priority: "High", Description: "Task D", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{}}},
	}

	featureList := db.FeatureList{
		ProjectName: "TestProject",
		Features:    features,
	}

	planFile := filepath.Join(tmpDir, "feature_list.json")
	data, _ := json.Marshal(featureList)
	if err := os.WriteFile(planFile, data, 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	// Run the command
	cmd := prioritizeCmd
	// Reset flags manually just in case
	prioritizeDryRun = false
	prioritizeOutput = ""

	err = cmd.RunE(cmd, []string{planFile})
	assert.NoError(t, err)

	// Verify the file was updated
	content, err := os.ReadFile(planFile)
	assert.NoError(t, err)

	var updatedList db.FeatureList
	err = json.Unmarshal(content, &updatedList)
	assert.NoError(t, err)

	assert.Equal(t, 4, len(updatedList.Features))

	ids := make([]string, len(updatedList.Features))
	for i, f := range updatedList.Features {
		ids[i] = f.ID
	}

	bIdx := indexOf("B", ids)
	cIdx := indexOf("C", ids)
	aIdx := indexOf("A", ids)

	assert.True(t, bIdx < cIdx, "B should come before C")
	assert.True(t, bIdx < aIdx, "B should come before A")

	assert.Contains(t, []string{"B", "D"}, ids[0])
	assert.Contains(t, []string{"B", "D"}, ids[1])
}

func TestPriorityComparison(t *testing.T) {
	assert.Equal(t, 1, comparePriority("High", "Medium"))
	assert.Equal(t, 1, comparePriority("Production", "MVP"))
	assert.Equal(t, 1, comparePriority("MVP", "POC"))

	assert.Equal(t, -1, comparePriority("Medium", "High"))
	assert.Equal(t, -1, comparePriority("Low", "Medium"))

	assert.Equal(t, 0, comparePriority("High", "High"))
	assert.Equal(t, 0, comparePriority("POC", "Low")) // Both 1
}

func TestTopologicalSortWithPriority(t *testing.T) {
	g := runner.NewTaskGraph()
	g.AddNode("A", "A", []string{"B"})
	g.Nodes["A"].Priority = "Low"

	g.AddNode("B", "B", []string{})
	g.Nodes["B"].Priority = "High"

	g.AddNode("C", "C", []string{"B"})
	g.Nodes["C"].Priority = "Medium"

	g.AddNode("D", "D", []string{})
	g.Nodes["D"].Priority = "High"

	order, err := topologicalSortWithPriority(g)
	assert.NoError(t, err)

	assert.Equal(t, 4, len(order))

	bIdx := indexOf("B", order)
	aIdx := indexOf("A", order)
	cIdx := indexOf("C", order)

	assert.True(t, bIdx < aIdx)
	assert.True(t, bIdx < cIdx)

	dIdx := indexOf("D", order)
	// D is High, C is Medium. Both ready after B? No.
	// D is ready immediately. B is ready immediately.
	// So D and B should be first 2.
	// C depends on B.

	assert.True(t, dIdx < cIdx)
}

func indexOf(val string, slice []string) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}
