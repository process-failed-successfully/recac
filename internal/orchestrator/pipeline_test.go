package orchestrator

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePipelineToWorkItems_Success(t *testing.T) {
	yamlData := []byte(`
name: Deploy Web App
defaults:
  repo_url: https://github.com/org/repo.git
  agent_provider: openrouter
  agent_model: openai/gpt-4o-mini
jobs:
  build:
    summary: Build application
    task: |
      npm install
      npm run build
    timeout: 30m
  test:
    summary: Run tests
    depends_on: [build]
    task: npm run test
    priority: 10
  deploy:
    summary: Deploy to staging
    depends_on: [test]
    task: ./deploy.sh
    repo_url: https://github.com/org/deploy-repo.git
    cancel_in_progress: true
    concurrency_group: deploy-staging
`)

	items, err := ParsePipelineToWorkItems(yamlData)
	require.NoError(t, err)
	assert.Len(t, items, 3)

	// Map by Original Job Key to easily check properties
	// Since IDs are dynamically generated with timestamps, we find them by matching the generated prefix
	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		// ID format is 'deploy-web-app-<job-key>-<timestamp>'
		parts := strings.Split(item.ID, "-")
		// the timestamp is the last part
		// job key is the second to last part
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	// Build Job
	buildJob, ok := jobMap["build"]
	require.True(t, ok)
	assert.Equal(t, "Build application", buildJob.Summary)
	assert.Equal(t, "npm install\nnpm run build\n", buildJob.Description)
	assert.Equal(t, "https://github.com/org/repo.git", buildJob.RepoURL)
	assert.Equal(t, "openrouter", buildJob.AgentProvider)
	assert.Equal(t, "openai/gpt-4o-mini", buildJob.AgentModel)
	assert.Equal(t, 30*time.Minute, buildJob.Timeout)
	assert.Empty(t, buildJob.DependsOn)

	// Test Job
	testJob, ok := jobMap["test"]
	require.True(t, ok)
	assert.Equal(t, "Run tests", testJob.Summary)
	assert.Equal(t, 10, testJob.Priority)
	assert.Len(t, testJob.DependsOn, 1)
	assert.Equal(t, buildJob.ID, testJob.DependsOn[0])

	// Deploy Job
	deployJob, ok := jobMap["deploy"]
	require.True(t, ok)
	assert.Equal(t, "https://github.com/org/deploy-repo.git", deployJob.RepoURL)
	assert.True(t, deployJob.CancelInProgress)
	assert.Equal(t, "deploy-staging", deployJob.ConcurrencyGroup)
	assert.Len(t, deployJob.DependsOn, 1)
	assert.Equal(t, testJob.ID, deployJob.DependsOn[0])
}

func TestParsePipelineToWorkItems_MissingName(t *testing.T) {
	yamlData := []byte(`
jobs:
  build:
    summary: Build application
`)
	_, err := ParsePipelineToWorkItems(yamlData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline must have a name")
}

func TestParsePipelineToWorkItems_NoJobs(t *testing.T) {
	yamlData := []byte(`
name: Empty Pipeline
`)
	_, err := ParsePipelineToWorkItems(yamlData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline must have at least one job")
}

func TestParsePipelineToWorkItems_InvalidDependency(t *testing.T) {
	yamlData := []byte(`
name: Bad Deps
jobs:
  test:
    summary: Test
    depends_on: [build]
`)
	_, err := ParsePipelineToWorkItems(yamlData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job 'test' depends on unknown job 'build'")
}

func TestParsePipelineToWorkItems_InvalidTimeout(t *testing.T) {
	yamlData := []byte(`
name: Bad Timeout
jobs:
  build:
    summary: Build
    timeout: invalid
`)
	_, err := ParsePipelineToWorkItems(yamlData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout format for job 'build'")
}

func TestParsePipelineToWorkItems_InvalidYaml(t *testing.T) {
	yamlData := []byte(`
name: Invalid Yaml
jobs:
  build:
    summary: Build
    depends_on: [
`)
	_, err := ParsePipelineToWorkItems(yamlData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal pipeline YAML")
}

func TestParsePipelineToWorkItems_Matrix(t *testing.T) {
	yamlData := []byte(`
name: Matrix Pipeline
defaults:
  repo_url: https://github.com/org/repo.git
jobs:
  lint:
    summary: Run linter
    task: npm run lint
  test:
    summary: Run tests
    depends_on: [lint]
    task: npm run test
    matrix:
      NODE_VERSION: ["16", "18"]
      OS: ["ubuntu-latest", "macos-latest"]
  deploy:
    summary: Deploy
    depends_on: [test]
    task: ./deploy.sh
`)

	items, err := ParsePipelineToWorkItems(yamlData)
	require.NoError(t, err)

	// lint: 1 job
	// test: 2 * 2 = 4 jobs
	// deploy: 1 job
	// Total: 6 jobs
	assert.Len(t, items, 6)

	// Find the jobs
	var lintJob WorkItem
	var testJobs []WorkItem
	var deployJob WorkItem

	for _, item := range items {
		if strings.Contains(item.ID, "matrix-pipeline-lint") {
			lintJob = item
		} else if strings.Contains(item.ID, "matrix-pipeline-test") {
			testJobs = append(testJobs, item)
		} else if strings.Contains(item.ID, "matrix-pipeline-deploy") {
			deployJob = item
		}
	}

	assert.NotEqual(t, "", lintJob.ID)
	assert.Len(t, testJobs, 4)
	assert.NotEqual(t, "", deployJob.ID)

	// Check test matrix permutations
	var envVarsFound []string
	for _, job := range testJobs {
		assert.Equal(t, []string{lintJob.ID}, job.DependsOn)
		envVarsFound = append(envVarsFound, fmt.Sprintf("%s-%s", job.EnvVars["NODE_VERSION"], job.EnvVars["OS"]))
	}
	assert.ElementsMatch(t, []string{
		"16-ubuntu-latest",
		"16-macos-latest",
		"18-ubuntu-latest",
		"18-macos-latest",
	}, envVarsFound)

	// Check deploy dependency resolution
	assert.Len(t, deployJob.DependsOn, 4)
	var expectedTestIDs []string
	for _, job := range testJobs {
		expectedTestIDs = append(expectedTestIDs, job.ID)
	}
	assert.ElementsMatch(t, expectedTestIDs, deployJob.DependsOn)
}
