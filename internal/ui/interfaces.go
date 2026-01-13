package ui

import "recac/internal/runner"

// ISessionManager defines the interface for managing sessions that the UI needs.
// This helps to decouple the UI from the command-line implementation and prevent
// import cycles.
type ISessionManager interface {
	ListSessions() ([]*runner.SessionState, error)
}
