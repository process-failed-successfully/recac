package ui

import "recac/internal/runner"

// ISessionManager defines the interface for session management.
// This is a subset of the main ISessionManager, containing only what the UI needs.
type ISessionManager interface {
	ListSessions() ([]*runner.SessionState, error)
}
