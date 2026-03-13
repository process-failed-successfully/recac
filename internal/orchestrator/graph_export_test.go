package orchestrator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportGraphToMermaid(t *testing.T) {
	jobs := []JobInfo{
		{
			ID:     "job-1",
			Status: "Completed",
			WorkItem: WorkItem{
				DependsOn: []string{},
			},
		},
		{
			ID:     "job-2",
			Status: "Failed",
			WorkItem: WorkItem{
				DependsOn: []string{"job-1"},
			},
		},
		{
			ID:     "job.3", // Test sanitization
			Status: "Running",
			WorkItem: WorkItem{
				DependsOn: []string{"job-1", "job-unknown"}, // Test unknown dependency ignores edge
			},
		},
	}

	mermaid := ExportGraphToMermaid(jobs)

	// Check header and definitions
	assert.True(t, strings.HasPrefix(mermaid, "graph TD;\n"))
	assert.Contains(t, mermaid, "classDef default")
	assert.Contains(t, mermaid, "classDef completed")
	assert.Contains(t, mermaid, "classDef failed")

	// Check nodes mapping
	assert.Contains(t, mermaid, "job_1[\"job-1\n(Completed)\"]:::completed;")
	assert.Contains(t, mermaid, "job_2[\"job-2\n(Failed)\"]:::failed;")
	assert.Contains(t, mermaid, "job_3[\"job.3\n(Running)\"]:::running;")

	// Check edges
	assert.Contains(t, mermaid, "job_1 --> job_2;")
	assert.Contains(t, mermaid, "job_1 --> job_3;")

	// Edge for job-unknown should not exist
	assert.NotContains(t, mermaid, "job_unknown --> job_3;")
}

func TestExportGraphToDOT(t *testing.T) {
	jobs := []JobInfo{
		{
			ID:     "job-1",
			Status: "Completed",
			WorkItem: WorkItem{
				DependsOn: []string{},
			},
		},
		{
			ID:     "job-2",
			Status: "Failed",
			WorkItem: WorkItem{
				DependsOn: []string{"job-1"},
			},
		},
		{
			ID:     "job.3", // Test sanitization
			Status: "Running",
			WorkItem: WorkItem{
				DependsOn: []string{"job-1", "job-unknown"}, // Test unknown dependency ignores edge
			},
		},
	}

	dot := ExportGraphToDOT(jobs)

	// Check header
	assert.True(t, strings.HasPrefix(dot, "digraph G {\n"))

	// Check nodes mapping
	assert.Contains(t, dot, "\"job_1\" [label=\"job-1\\n(Completed)\", fillcolor=\"lightgreen\"];")
	assert.Contains(t, dot, "\"job_2\" [label=\"job-2\\n(Failed)\", fillcolor=\"lightcoral\"];")
	assert.Contains(t, dot, "\"job_3\" [label=\"job.3\\n(Running)\", fillcolor=\"lightblue\"];")

	// Check edges
	assert.Contains(t, dot, "\"job_1\" -> \"job_2\";")
	assert.Contains(t, dot, "\"job_1\" -> \"job_3\";")

	// Edge for job-unknown should not exist
	assert.NotContains(t, dot, "\"job_unknown\" -> \"job_3\";")
}
