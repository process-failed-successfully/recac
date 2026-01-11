package main

import "fmt"

// MockGitClient provides a mock implementation of the gitClient interface for testing.
type MockGitClient struct {
	DiffFunc             func(workspace, fromSHA, toSHA string) (string, error)
	CurrentCommitSHAFunc func(workspace string) (string, error)
}

func (m *MockGitClient) Diff(workspace, fromSHA, toSHA string) (string, error) {
	if m.DiffFunc != nil {
		return m.DiffFunc(workspace, fromSHA, toSHA)
	}
	return "", fmt.Errorf("DiffFunc not implemented")
}

func (m *MockGitClient) CurrentCommitSHA(workspace string) (string, error) {
	if m.CurrentCommitSHAFunc != nil {
		return m.CurrentCommitSHAFunc(workspace)
	}
	return "", fmt.Errorf("CurrentCommitSHAFunc not implemented")
}
