package runner

// ISessionManager defines the interface for session management.
// This allows for mocking in tests.
type ISessionManager interface {
	ListSessions() ([]*SessionState, error)
	StartSession(name string, command []string, workspace string) (*SessionState, error)
	GetSession(name string) (*SessionState, error)
	UpdateSessionStatus(name, status string) error
	EndSession(name string) error
	IsProcessRunning(pid int) bool
	GetLogPath(name string) string
	GetStatePath(name string) string
}
