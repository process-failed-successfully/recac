package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
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
  require_approval: true
  retry_delay: 10s
  retry_backoff_multiplier: 1.5
  env_vars:
    GLOBAL_VAR: "global_value"
  tags:
    - global_tag
  webhook_url: https://example.com/webhook/global
jobs:
  build:
    summary: Build application
    task: |
      npm install
      npm run build
    timeout: 30m
    require_approval: false
    retry_delay: 5s
    retry_backoff_multiplier: 2.0
    env_vars:
      LOCAL_VAR: "local_value"
      GLOBAL_VAR: "overridden_value"
    tags:
      - local_tag
    webhook_url: https://example.com/webhook/build
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

	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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
	require.NotNil(t, buildJob.RequireApproval)
	assert.False(t, *buildJob.RequireApproval)
	require.NotNil(t, buildJob.RetryDelay)
	assert.Equal(t, 5*time.Second, *buildJob.RetryDelay)
	require.NotNil(t, buildJob.RetryBackoffMultiplier)
	assert.Equal(t, 2.0, *buildJob.RetryBackoffMultiplier)
	assert.Equal(t, map[string]string{"GLOBAL_VAR": "overridden_value", "LOCAL_VAR": "local_value"}, buildJob.EnvVars)
	assert.Equal(t, []string{"global_tag", "local_tag"}, buildJob.Tags)
	assert.Equal(t, "https://example.com/webhook/build", buildJob.WebhookURL)

	// Test Job
	testJob, ok := jobMap["test"]
	require.True(t, ok)
	assert.Equal(t, "Run tests", testJob.Summary)
	require.NotNil(t, testJob.RequireApproval)
	assert.True(t, *testJob.RequireApproval)
	require.NotNil(t, testJob.RetryDelay)
	assert.Equal(t, 10*time.Second, *testJob.RetryDelay)
	require.NotNil(t, testJob.RetryBackoffMultiplier)
	assert.Equal(t, 1.5, *testJob.RetryBackoffMultiplier)
	assert.Equal(t, map[string]string{"GLOBAL_VAR": "global_value"}, testJob.EnvVars)
	assert.Equal(t, []string{"global_tag"}, testJob.Tags)
	assert.Equal(t, 10, testJob.Priority)
	assert.Equal(t, "https://example.com/webhook/global", testJob.WebhookURL)
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
	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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
	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline must have a name")
}

func TestParsePipelineToWorkItems_NoJobs(t *testing.T) {
	yamlData := []byte(`
name: Empty Pipeline
`)
	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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
	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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
	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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
	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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
	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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
	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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
	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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

	items, err := ParsePipelineToWorkItems(yamlData, "test", nil, "")
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

	_, err := ParsePipelineToWorkItems(yamlData, "unknown", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target job 'unknown' not found in pipeline")
}

func TestParsePipelineToWorkItems_RequiredVariables(t *testing.T) {
	yamlData := []byte(`
name: Required Vars Pipeline
required_variables:
  - NEED_THIS
  - ALSO_NEED_THIS
jobs:
  job1:
    summary: Build
`)

	// Test 1: Missing all required variables
	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required variable 'NEED_THIS' is missing")

	// Test 2: Missing one required variable
	vars1 := map[string]string{
		"NEED_THIS": "present",
	}
	_, err = ParsePipelineToWorkItems(yamlData, "", vars1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required variable 'ALSO_NEED_THIS' is missing")

	// Test 3: Success (provided in vars)
	vars2 := map[string]string{
		"NEED_THIS":      "present",
		"ALSO_NEED_THIS": "also_present",
	}
	items, err := ParsePipelineToWorkItems(yamlData, "", vars2, "")
	require.NoError(t, err)
	assert.Len(t, items, 1)

	// Test 4: Success (one in vars, one in environment)
	t.Setenv("ALSO_NEED_THIS", "from_env")
	items, err = ParsePipelineToWorkItems(yamlData, "", vars1, "")
	require.NoError(t, err)
	assert.Len(t, items, 1)

	// Test 5: Success (one in yaml variables, one in vars)
	yamlDataWithVars := []byte(`
name: Required Vars Pipeline
required_variables:
  - NEED_THIS
  - ALSO_NEED_THIS
variables:
  ALSO_NEED_THIS: from_yaml
jobs:
  job1:
    summary: Build
`)
	items, err = ParsePipelineToWorkItems(yamlDataWithVars, "", vars1, "")
	require.NoError(t, err)
	assert.Len(t, items, 1)
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

	items, err := ParsePipelineToWorkItems(yamlData, "", vars, "")
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

func TestParsePipelineToWorkItems_NativeVariables(t *testing.T) {
	yamlData := []byte(`
name: Native Var Pipeline
variables:
  GLOBAL_VAR: "global_val"
  OVERRIDE_VAR: "global_override"
jobs:
  job1:
    summary: Build ${GLOBAL_VAR}
    task: echo ${OVERRIDE_VAR}
    variables:
      OVERRIDE_VAR: "local_val"
    env_vars:
      TEST_VAR: ${OVERRIDE_VAR}
`)

	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.NoError(t, err)
	assert.Len(t, items, 1)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	job1 := jobMap["job1"]
	assert.Equal(t, "Build global_val", job1.Summary)
	assert.Equal(t, "echo local_val", job1.Description)
	assert.Equal(t, "local_val", job1.EnvVars["TEST_VAR"])
}

func TestParsePipelineToWorkItems_Secrets(t *testing.T) {
	t.Setenv("MY_TEST_SECRET", "super_secret_value")

	yamlData := []byte(`
name: Secret Pipeline
secrets:
  - MY_TEST_SECRET
jobs:
  job1:
    summary: Job with secrets
`)

	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.NoError(t, err)
	assert.Len(t, items, 1)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	job1 := jobMap["job1"]
	assert.Equal(t, "super_secret_value", job1.EnvVars["MY_TEST_SECRET"])
}

func TestParsePipelineToWorkItems_MissingSecrets(t *testing.T) {
	yamlData := []byte(`
name: Missing Secret Pipeline
secrets:
  - MISSING_TEST_SECRET
jobs:
  job1:
    summary: Job with missing secrets
`)

	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required secret 'MISSING_TEST_SECRET' is missing from environment")
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

	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
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

func TestParsePipelineToWorkItems_DefaultCancelInProgress(t *testing.T) {
	yamlData := []byte(`
name: Cancel Default
defaults:
  repo_url: https://github.com/org/repo.git
  cancel_in_progress: true
jobs:
  job1:
    summary: Job 1 uses default
  job2:
    summary: Job 2 overrides to false
    cancel_in_progress: false
`)
	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.NoError(t, err)
	assert.Len(t, items, 2)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	job1 := jobMap["job1"]
	assert.True(t, job1.CancelInProgress)

	job2 := jobMap["job2"]
	assert.False(t, job2.CancelInProgress)
}

func TestParsePipelineToWorkItems_Stages(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Stages
stages:
  - build
  - test
  - deploy
jobs:
  build1:
    summary: Build App 1
    stage: build
  build2:
    summary: Build App 2
    stage: build
  test1:
    summary: Test App 1
    stage: test
  deploy1:
    summary: Deploy App
    stage: deploy
`)

	items, err := ParsePipelineToWorkItemsWithRunID(yamlData, "", nil, "stable", "")
	require.NoError(t, err)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		// ID format is 'pipeline-stages-<job-key>' because "stable" omits the suffix!
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-1]
		jobMap[jobKey] = item
	}

	build1 := jobMap["build1"]
	assert.Empty(t, build1.DependsOn)

	build2 := jobMap["build2"]
	assert.Empty(t, build2.DependsOn)

	test1 := jobMap["test1"]
	// test1 should depend on everything in build
	assert.Len(t, test1.DependsOn, 2)
	assert.ElementsMatch(t, []string{build1.ID, build2.ID}, test1.DependsOn)

	deploy1 := jobMap["deploy1"]
	// deploy1 should depend on everything in test
	assert.Len(t, deploy1.DependsOn, 1)
	assert.Equal(t, test1.ID, deploy1.DependsOn[0])
}

func TestParsePipelineToWorkItems_StagesWithEmptyIntermediate(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Stages Empty
stages:
  - build
  - empty_stage
  - test
jobs:
  build1:
    summary: Build App
    stage: build
  test1:
    summary: Test App
    stage: test
`)

	items, err := ParsePipelineToWorkItemsWithRunID(yamlData, "", nil, "stable", "")
	require.NoError(t, err)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-1]
		jobMap[jobKey] = item
	}

	build1 := jobMap["build1"]
	assert.Empty(t, build1.DependsOn)

	test1 := jobMap["test1"]
	// test1 should depend on build1 since empty_stage is skipped
	assert.Len(t, test1.DependsOn, 1)
	assert.Equal(t, build1.ID, test1.DependsOn[0])
}

func TestParsePipelineToWorkItems_InvalidStage(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Invalid Stage
stages:
  - build
jobs:
  build1:
    summary: Build
    stage: invalid_stage
`)

	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specifies unknown stage 'invalid_stage'")
}

func TestParsePipelineToWorkItems_StageWithoutStages(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Missing Stages List
jobs:
  build1:
    summary: Build
    stage: build
`)

	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no stages are defined in the pipeline")
}

func TestParsePipelineToWorkItems_Templates(t *testing.T) {
	yamlData := []byte(`
name: Template Pipeline
templates:
  base-build:
    summary: Base Build
    task: make build
    agent_provider: openrouter
    agent_model: openai/gpt-4o-mini
    tags: [build]
    env_vars:
      GOOS: linux
      GOARCH: amd64
    max_retries: 3
    timeout: 10m

  go-test:
    extends: base-build # Testing if extends on template does anything? Wait, currently we only process extends on Jobs.
    # We will test Job extends Template here.

jobs:
  job1:
    summary: Build App
    extends: base-build
    tags: [app]
    env_vars:
      GOOS: windows # overrides template
      APP_NAME: MyApp

  job2:
    extends: base-build
    task: make special-build # overrides template
`)

	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.NoError(t, err)
	assert.Len(t, items, 2)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	job1 := jobMap["job1"]
	assert.Equal(t, "Build App", job1.Summary)
	assert.Equal(t, "make build", job1.Description)
	assert.Equal(t, "openrouter", job1.AgentProvider)
	assert.Equal(t, "openai/gpt-4o-mini", job1.AgentModel)
	assert.Equal(t, []string{"build", "app"}, job1.Tags)
	assert.Equal(t, map[string]string{"GOOS": "windows", "GOARCH": "amd64", "APP_NAME": "MyApp"}, job1.EnvVars)
	require.NotNil(t, job1.MaxRetries)
	assert.Equal(t, 3, *job1.MaxRetries)
	assert.Equal(t, 10*time.Minute, job1.Timeout)

	job2 := jobMap["job2"]
	assert.Equal(t, "Base Build", job2.Summary) // inherited
	assert.Equal(t, "make special-build", job2.Description) // overridden
	assert.Equal(t, []string{"build"}, job2.Tags)
}

func TestParsePipelineToWorkItems_UnknownTemplate(t *testing.T) {
	yamlData := []byte(`
name: Unknown Template Pipeline
jobs:
  job1:
    summary: Build
    extends: unknown-template
`)

	_, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extends unknown template 'unknown-template'")
}

func TestParsePipelineToWorkItems_Include(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an included YAML file
	includedYaml := []byte(`
templates:
  base-test:
    summary: Base Test
    task: npm run test
    tags: [test]
jobs:
  included-job:
    summary: Included Job
    task: echo "Included"
`)
	includedPath := filepath.Join(tmpDir, "included.yaml")
	require.NoError(t, os.WriteFile(includedPath, includedYaml, 0644))

	// Create the main YAML file
	mainYaml := []byte(`
name: Main Pipeline
include:
  - included.yaml
jobs:
  main-job:
    summary: Main Job
    extends: base-test
    depends_on: [included-job]
`)

	items, err := ParsePipelineToWorkItemsWithRunID(mainYaml, "", nil, "stable", tmpDir)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		// ID format is 'main-pipeline-<job-key>' because "stable" omits the suffix!
		// Find the original key which might have multiple dashes (e.g. main-job, included-job)
		// For our case, it's just the substring after 'main-pipeline-'
		jobKey := strings.TrimPrefix(item.ID, "main-pipeline-")
		jobMap[jobKey] = item
	}

	mainJob, exists := jobMap["main-job"]
	require.True(t, exists, "main-job not found in items")
	assert.Equal(t, "Main Job", mainJob.Summary)
	assert.Equal(t, "npm run test", mainJob.Description) // inherited from template
	assert.Equal(t, []string{"test"}, mainJob.Tags)
	assert.Len(t, mainJob.DependsOn, 1)
	assert.Contains(t, mainJob.DependsOn[0], "included-job")

	includedJob, exists := jobMap["included-job"]
	require.True(t, exists, "included-job not found in items")
	assert.Equal(t, "Included Job", includedJob.Summary)
	assert.Empty(t, includedJob.DependsOn)
}

func TestParsePipelineToWorkItems_StagesWithExplicitDependsOn(t *testing.T) {
	yamlData := []byte(`
name: Pipeline Stages Explicit
stages:
  - build
  - test
jobs:
  setup:
    summary: Setup Environment
  build1:
    summary: Build App 1
    stage: build
    depends_on: [setup]
  test1:
    summary: Test App 1
    stage: test
`)

	items, err := ParsePipelineToWorkItemsWithRunID(yamlData, "", nil, "stable", "")
	require.NoError(t, err)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-1]
		jobMap[jobKey] = item
	}

	setup := jobMap["setup"]
	assert.Empty(t, setup.DependsOn)

	build1 := jobMap["build1"]
	// build1 depends on setup explicitly
	assert.Len(t, build1.DependsOn, 1)
	assert.Equal(t, setup.ID, build1.DependsOn[0])

	test1 := jobMap["test1"]
	// test1 depends on build1 via stage
	assert.Len(t, test1.DependsOn, 1)
	assert.Equal(t, build1.ID, test1.DependsOn[0])
}

func TestParsePipelineToWorkItems_MatrixExclude(t *testing.T) {
	yamlData := []byte(`
name: Matrix Pipeline
defaults:
  repo_url: https://github.com/org/repo.git
templates:
  base-test:
    exclude:
      - NODE_VERSION: "16"
        OS: "macos-latest"
jobs:
  test:
    extends: base-test
    summary: Run tests
    task: npm run test
    matrix:
      NODE_VERSION: ["16", "18"]
      OS: ["ubuntu-latest", "macos-latest"]
    exclude:
      - NODE_VERSION: "18"
        OS: "ubuntu-latest"
`)

	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.NoError(t, err)

	// test: 2 * 2 = 4 jobs, minus 2 excluded = 2 jobs
	assert.Len(t, items, 2)

	var envVarsFound []string
	for _, job := range items {
		envVarsFound = append(envVarsFound, fmt.Sprintf("%s-%s", job.EnvVars["NODE_VERSION"], job.EnvVars["OS"]))
	}
	assert.ElementsMatch(t, []string{
		"16-ubuntu-latest",
		"18-macos-latest",
	}, envVarsFound)
}

func TestParsePipelineToWorkItems_MatrixExcludeAll(t *testing.T) {
	yamlData := []byte(`
name: Matrix Exclude All Pipeline
jobs:
  test:
    summary: Run tests
    task: npm run test
    matrix:
      NODE_VERSION: ["16"]
    exclude:
      - NODE_VERSION: "16"
`)

	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.NoError(t, err)

	// All matrix combinations are excluded
	assert.Len(t, items, 0)
}

func TestParsePipelineToWorkItems_IfCondition(t *testing.T) {
	yamlData := []byte(`
name: Pipeline If Condition
defaults:
  repo_url: https://github.com/org/repo.git
  if: "${VAR1} == 'true'"
templates:
  base-test:
    if: "!${SKIP_TEST}"
jobs:
  job1:
    summary: Job 1 uses default if
  job2:
    summary: Job 2 overrides to specific if
    if: "'${VAR2}' != 'hello'"
  job3:
    summary: Job 3 extends template
    extends: base-test
`)
	items, err := ParsePipelineToWorkItems(yamlData, "", nil, "")
	require.NoError(t, err)
	assert.Len(t, items, 3)

	jobMap := make(map[string]WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	job1 := jobMap["job1"]
	assert.Equal(t, "${VAR1} == 'true'", job1.IfCondition)

	job2 := jobMap["job2"]
	assert.Equal(t, "'${VAR2}' != 'hello'", job2.IfCondition)

	job3 := jobMap["job3"]
	assert.Equal(t, "!${SKIP_TEST}", job3.IfCondition)
}
