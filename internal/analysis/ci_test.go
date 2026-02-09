package analysis

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeGitHubWorkflow_Empty(t *testing.T) {
	findings, err := AnalyzeGitHubWorkflow("")
	assert.NoError(t, err)
	assert.Empty(t, findings)
}

func TestAnalyzeGitHubWorkflow_Bad(t *testing.T) {
	content := `name: Bad
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@latest
      - uses: actions/setup-go@v4
`
	findings, err := AnalyzeGitHubWorkflow(content)
	assert.NoError(t, err)

	ruleMap := make(map[string]bool)
	for _, f := range findings {
		ruleMap[f.Rule] = true
	}

	assert.True(t, ruleMap["missing_permissions"], "Should detect missing permissions")
	assert.True(t, ruleMap["missing_timeout"], "Should detect missing timeout")
	assert.True(t, ruleMap["unpinned_action"], "Should detect @latest")
	assert.True(t, ruleMap["action_ref_tag"], "Should detect tag ref")
	assert.True(t, ruleMap["missing_cache"], "Should detect missing cache")
}

func TestAnalyzeGitHubWorkflow_Good(t *testing.T) {
	content := `name: Good
permissions:
  contents: read
jobs:
  build:
    timeout-minutes: 10
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab
      - uses: actions/setup-go@fac6363333333333333333333333333333333333
        with:
          cache: true
`
	findings, err := AnalyzeGitHubWorkflow(content)
	assert.NoError(t, err)
	// We expect 0 findings because:
	// - permissions exist
	// - timeout exists
	// - checkout uses long SHA (>= 40 chars)
	// - setup-go uses long SHA and has cache in 'with'

	// Let's debug if it fails
	for _, f := range findings {
		t.Logf("Unexpected finding: %v", f)
	}
	assert.Empty(t, findings)
}
