package runner

// ISessionManager defines the interface for managing sessions.
// This allows for mocking in tests.
type ISessionManager interface {
	ListSessions() ([]*SessionState, error)
	LoadSession(name string) (*SessionState, error)
	GetSessionLogs(name string) (string, error)
	StartSession(name string, command []string, workspace string) (*SessionState, error)
	StopSession(name string) error
}
