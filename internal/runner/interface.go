package runner

// ISessionManager defines the interface for session management.
type ISessionManager interface {
	ListSessions() ([]*SessionState, error)
	SaveSession(session *SessionState) error
	LoadSession(name string) (*SessionState, error)
	StopSession(name string) error
	GetSessionLogs(name string) (string, error)
}
