package orchestrator

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePipelineToWorkItems_ContinueOnError(t *testing.T) {
	yamlData := []byte(`
name: Pipeline COE
defaults:
  repo_url: https://github.com/org/repo.git
  continue_on_error: true
jobs:
  job1:
    summary: Job 1 uses default COE
  job2:
    summary: Job 2 overrides to false
    continue_on_error: false
`)
	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.NoError(t, err)
	assert.Len(t, items, 2)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		if item.Summary == "Job 1 uses default COE" {
			jobMap["job1"] = item
		} else {
			jobMap["job2"] = item
		}
	}

	job1 := jobMap["job1"]
	assert.True(t, job1.ContinueOnError)

	job2 := jobMap["job2"]
	assert.False(t, job2.ContinueOnError)
}
