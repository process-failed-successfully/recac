package main

import "recac/internal/runner"

// ISessionManager defines the interface for session management.
// This allows for mocking in tests.
type ISessionManager interface {
	ListSessions() ([]*runner.SessionState, error)
	StopSession(name string) error
	LoadSession(name string) (*runner.SessionState, error)
	StartSession(name string, command []string, workspace string) (*runner.SessionState, error)
	IsProcessRunning(pid int) bool
	GetSessionPath(name string) string
}
