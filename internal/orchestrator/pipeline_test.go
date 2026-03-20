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
  max_retries: 2
  env_vars:
    GLOBAL_VAR: "global_value"
  tags:
    - global_tag
jobs:
  build:
    summary: Build application
    task: |
      npm install
      npm run build
    timeout: 30m
    env_vars:
      LOCAL_VAR: "local_value"
      GLOBAL_VAR: "overridden_value"
    tags:
      - local_tag
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

	items, err := ParsePipelineToWorkItems(yamlData, "", nil)
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
	require.NotNil(t, buildJob.MaxRetries)
	assert.Equal(t, 2, *buildJob.MaxRetries)
	assert.Equal(t, map[string]string{"GLOBAL_VAR": "overridden_value", "LOCAL_VAR": "local_value"}, buildJob.EnvVars)
	assert.Equal(t, []string{"global_tag", "local_tag"}, buildJob.Tags)

	// Test Job
	testJob, ok := jobMap["test"]
	require.True(t, ok)
	assert.Equal(t, "Run tests", testJob.Summary)
	assert.Equal(t, map[string]string{"GLOBAL_VAR": "global_value"}, testJob.EnvVars)
	assert.Equal(t, []string{"global_tag"}, testJob.Tags)
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

func TestParsePipelineToWorkItems_RunCondition(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Cond
defaults:
  repo_url: https://github.com/org/repo.git
jobs:
  job1:
    summary: Job 1
  job2:
    summary: Job 2
    depends_on: [job1]
    run_condition: on_failure
  job3:
    summary: Job 3
    depends_on: [job2]
    run_condition: always
`)
	items, err := ParsePipelineToWorkItems(yamlData, "", nil)
	require.NoError(t, err)
	assert.Len(t, items, 3)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	job1 := jobMap["job1"]
	assert.Empty(t, job1.RunCondition)

	job2 := jobMap["job2"]
	assert.Equal(t, "on_failure", job2.RunCondition)

	job3 := jobMap["job3"]
	assert.Equal(t, "always", job3.RunCondition)
}

func TestParsePipelineToWorkItems_MissingName(t *testing.T) {
	yamlData := []byte(`
jobs:
  build:
    summary: Build application
`)
	_, err := ParsePipelineToWorkItems(yamlData, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline must have a name")
}

func TestParsePipelineToWorkItems_NoJobs(t *testing.T) {
	yamlData := []byte(`
name: Empty Pipeline
`)
	_, err := ParsePipelineToWorkItems(yamlData, "", nil)
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
	_, err := ParsePipelineToWorkItems(yamlData, "", nil)
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
	_, err := ParsePipelineToWorkItems(yamlData, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout format for job 'build'")
}

func TestParsePipelineToWorkItems_InvalidDelay(t *testing.T) {
	yamlData := []byte(`
name: Bad Delay
jobs:
  build:
    summary: Build
    delay: invalid
`)
	_, err := ParsePipelineToWorkItems(yamlData, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid delay format for job 'build'")
}

func TestParsePipelineToWorkItems_DefaultDependsOn(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Default Deps
defaults:
  repo_url: https://github.com/org/repo.git
  depends_on: [setup]
jobs:
  setup:
    summary: Setup env
  build:
    summary: Build application
  test:
    summary: Run tests
    depends_on: [build]
`)
	items, err := ParsePipelineToWorkItems(yamlData, "", nil)
	require.NoError(t, err)
	assert.Len(t, items, 3)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	setupJob := jobMap["setup"]
	// Setup should ignore the global self-dependency
	assert.Empty(t, setupJob.DependsOn)

	buildJob := jobMap["build"]
	// Build should inherit the global setup dependency
	assert.Len(t, buildJob.DependsOn, 1)
	assert.Equal(t, setupJob.ID, buildJob.DependsOn[0])

	testJob := jobMap["test"]
	// Test should have both setup (global) and build (local)
	assert.Len(t, testJob.DependsOn, 2)
	assert.ElementsMatch(t, []string{setupJob.ID, buildJob.ID}, testJob.DependsOn)
}

func TestParsePipelineToWorkItems_Delay(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Delay
defaults:
  repo_url: https://github.com/org/repo.git
  delay: 1h
jobs:
  build:
    summary: Build application
  test:
    summary: Run tests
    depends_on: [build]
    delay: 2h
`)
	now := time.Now()
	items, err := ParsePipelineToWorkItems(yamlData, "", nil)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	buildJob := jobMap["build"]
	// Should use default 1h delay
	assert.Equal(t, 1*time.Hour, buildJob.Delay)
	assert.True(t, buildJob.RunAfter.IsZero())

	testJob := jobMap["test"]
	// Should override to 2h delay
	assert.Equal(t, 2*time.Hour, testJob.Delay)
	assert.True(t, testJob.RunAfter.IsZero())

	// Just so it compiles, use now or remove it.
	_ = now
}

func TestParsePipelineToWorkItems_InvalidYaml(t *testing.T) {
	yamlData := []byte(`
name: Invalid Yaml
jobs:
  build:
    summary: Build
    depends_on: [
`)
	_, err := ParsePipelineToWorkItems(yamlData, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal pipeline YAML")
}

func TestParsePipelineToWorkItems_Targeting(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Target
defaults:
  repo_url: https://github.com/org/repo.git
jobs:
  setup:
    summary: Setup
  build:
    summary: Build
    depends_on: [setup]
  test:
    summary: Run tests
    depends_on: [build]
  deploy:
    summary: Deploy
    depends_on: [test]
  other:
    summary: Other job
`)

	items, err := ParsePipelineToWorkItems(yamlData, "test", nil)
	require.NoError(t, err)

	// Should include "setup", "build", and "test", but not "deploy" or "other"
	assert.Len(t, items, 3)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	assert.Contains(t, jobMap, "setup")
	assert.Contains(t, jobMap, "build")
	assert.Contains(t, jobMap, "test")
	assert.NotContains(t, jobMap, "deploy")
	assert.NotContains(t, jobMap, "other")
}

func TestParsePipelineToWorkItems_InvalidTarget(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Invalid Target
jobs:
  setup:
    summary: Setup
`)

	_, err := ParsePipelineToWorkItems(yamlData, "unknown", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target job 'unknown' not found in pipeline")
}

func TestParsePipelineToWorkItems_Variables(t *testing.T) {
	yamlData := []byte(`
name: Var Pipeline
defaults:
  repo_url: ${DEFAULT_REPO}
jobs:
  job1:
    summary: Build ${APP_NAME}
    task: echo ${APP_NAME}
    env_vars:
      MY_VAR: ${MY_ENV_VAR}
      MISSING_VAR: ${MISSING}
`)

	vars := map[string]string{
		"DEFAULT_REPO": "https://github.com/my/repo",
		"APP_NAME":     "SuperApp",
		"MY_ENV_VAR":   "from_env",
	}

	items, err := ParsePipelineToWorkItems(yamlData, "", vars)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	job1 := jobMap["job1"]
	assert.Equal(t, "Build SuperApp", job1.Summary)
	assert.Equal(t, "echo SuperApp", job1.Description)
	assert.Equal(t, "https://github.com/my/repo", job1.RepoURL)
	assert.Equal(t, "from_env", job1.EnvVars["MY_VAR"])
	assert.Equal(t, "${MISSING}", job1.EnvVars["MISSING_VAR"]) // Should be left alone
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

	items, err := ParsePipelineToWorkItems(yamlData, "", nil)
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
