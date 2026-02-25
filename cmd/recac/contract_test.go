package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

type ContractMockAgent struct {
	ShouldPass bool
}

func (m *ContractMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	// Detect if this is a generation prompt or verification prompt
	if strings.Contains(prompt, "Generate a valid HTTP request") {
		// Return dummy request params
		resp := AIRequestParams{
			Method:  "GET",
			Path:    "/test",
			Headers: map[string]string{"X-Test": "true"},
			Query:   map[string]string{"foo": "bar"},
		}
		data, _ := json.Marshal(resp)
		return string(data), nil
	}

	if strings.Contains(prompt, "Check if the actual API response conforms") {
		// Return verification result
		resp := AIVerificationResult{
			Passed: m.ShouldPass,
			Reason: "Test reason",
		}
		if !m.ShouldPass {
			resp.Reason = "Verification failed"
		}
		data, _ := json.Marshal(resp)
		return string(data), nil
	}

	return "", fmt.Errorf("unknown prompt: %s", prompt)
}

func (m *ContractMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestContractVerify(t *testing.T) {
	// 1. Setup Mock Server (Target)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer ts.Close()

	// 2. Setup Mock Agent
	mockAg := &ContractMockAgent{ShouldPass: true}
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// 3. Create Temp Spec File
	specContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: OK
`
	tmpFile, err := os.CreateTemp("", "openapi-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write([]byte(specContent)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// 4. Run Command
	// We set flags directly because they are global vars in contract.go
	contractSpec = tmpFile.Name()
	contractTarget = ts.URL
	contractOutput = ""

	// Execute
	// We pass empty args because we use flags directly
	err = runContractVerify(contractVerifyCmd, []string{})
	assert.NoError(t, err)
}

func TestContractVerify_Failure(t *testing.T) {
	// 1. Setup Mock Server (Target)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	// 2. Setup Mock Agent (Fails Verification)
	mockAg := &ContractMockAgent{ShouldPass: false}
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// 3. Create Temp Spec File
	specContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /test:
    get:
      responses:
        '200':
          description: OK
`
	tmpFile, err := os.CreateTemp("", "openapi-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write([]byte(specContent)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// 4. Run Command
	contractSpec = tmpFile.Name()
	contractTarget = ts.URL
	contractOutput = ""

	err = runContractVerify(contractVerifyCmd, []string{})
	assert.Error(t, err)
	// We check error message, but exact wording might vary.
	// runContractVerify returns "contract verification failed" if not passed.
	assert.Contains(t, err.Error(), "contract verification failed")
}
