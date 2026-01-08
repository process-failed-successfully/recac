package runner

// ISessionManager defines the interface for session management.
// This allows for mocking in tests.
type ISessionManager interface {
	ListSessions() ([]*SessionState, error)
	LoadSession(name string) (*SessionState, error)
	IsProcessRunning(pid int) bool
	StartSession(name string, command []string, workspace string) (*SessionState, error)
	StopSession(name string) error
	GetSessionLogs(name string) (string, error)
}

// SessionLister is an alias for ISessionManager for backward compatibility.
// It was previously used for mocking session listing.
type SessionLister interface {
	ListSessions() ([]*SessionState, error)
}
