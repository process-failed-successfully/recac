package main

import (
	"bytes"
	"context"
	"errors"
	"recac/internal/agent"
	"recac/internal/jira"
	"testing"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAgentClient provides a mock implementation of an Agent for testing.
type mockAgentClient struct {
	SendStreamFunc func(ctx context.Context, prompt string, onChunk func(string)) (string, error)
}

func (m *mockAgentClient) Send(ctx context.Context, prompt string) (string, error) {
	// Not used by doctor, but required by the Agent interface.
	return "", nil
}

func (m *mockAgentClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.SendStreamFunc != nil {
		return m.SendStreamFunc(ctx, prompt, onChunk)
	}
	return "mock response", nil
}

// mockJiraClient provides a mock implementation of a Jira client for testing.
type mockJiraClient struct {
	AuthenticateFunc func(ctx context.Context) error
}

func (m *mockJiraClient) Authenticate(ctx context.Context) error {
	if m.AuthenticateFunc != nil {
		return m.AuthenticateFunc(ctx)
	}
	return nil
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// executeDoctorCommand is a helper to run the doctor command and capture its output.
func executeDoctorCommand(root *cobra.Command) (string, error) {
	b := new(bytes.Buffer)
	root.SetOut(b)
	root.SetErr(b)
	err := root.Execute()
	return b.String(), err
}

func TestDoctorCommand_AllPassing(t *testing.T) {
	// Setup a clean command for this test.
	root, _, _ := newRootCmd()
	root.AddCommand(doctorCmd)

	// Mock successful dependencies.
	getAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAgentClient{}, nil
	}
	getJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return &jira.Client{}, nil
	}
	checkDocker = func() error {
		return nil
	}
	checkConfig = func() error {
		return nil
	}

	output, err := executeDoctorCommand(root)

	require.NoError(t, err)
	assert.Contains(t, output, "Configuration File: ✅ PASSED")
	assert.Contains(t, output, "Docker Daemon: ✅ PASSED")
	assert.Contains(t, output, "AI Provider Connectivity: ✅ PASSED")
	assert.Contains(t, output, "Jira Connectivity: ✅ PASSED")
	assert.Contains(t, output, "All checks passed!")
}

func TestDoctorCommand_AIFails(t *testing.T) {
	root, _, _ := newRootCmd()
	root.AddCommand(doctorCmd)

	getAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAgentClient{
			SendStreamFunc: func(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
				return "", errors.New("invalid API key")
			},
		}, nil
	}
	getJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return &jira.Client{}, nil
	}
	checkDocker = func() error {
		return nil
	}
	checkConfig = func() error {
		return nil
	}

	output, err := executeDoctorCommand(root)
	require.Error(t, err)
	assert.Contains(t, output, "AI Provider Connectivity: ❌ FAILED")
	assert.Contains(t, output, "invalid API key")
	assert.Contains(t, output, "Some checks failed.")
}

func TestDoctorCommand_JiraFails(t *testing.T) {
	root, _, _ := newRootCmd()
	root.AddCommand(doctorCmd)

	getAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAgentClient{}, nil
	}
	getJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return nil, errors.New("missing token")
	}
	checkDocker = func() error {
		return nil
	}
	checkConfig = func() error {
		return nil
	}

	output, err := executeDoctorCommand(root)
	require.Error(t, err)
	assert.Contains(t, output, "Jira Connectivity: ❌ FAILED")
	assert.Contains(t, output, "missing token")
	assert.Contains(t, output, "Some checks failed.")
}

func TestDoctorCommand_DockerFails(t *testing.T) {
	root, _, _ := newRootCmd()
	root.AddCommand(doctorCmd)

	getAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAgentClient{}, nil
	}
	getJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return &jira.Client{}, nil
	}
	checkDocker = func() error {
		return errors.New("daemon not running")
	}
	checkConfig = func() error {
		return nil
	}

	output, err := executeDoctorCommand(root)
	require.Error(t, err)
	assert.Contains(t, output, "Docker Daemon: ❌ FAILED")
	assert.Contains(t, output, "daemon not running")
	assert.Contains(t, output, "Some checks failed.")
}
