package main

import "recac/internal/runner"

// ISessionManager defines the interface for session management.
type ISessionManager interface {
	ListSessions() ([]*runner.SessionState, error)
	SaveSession(*runner.SessionState) error
	LoadSession(name string) (*runner.SessionState, error)
	StopSession(name string) error
	GetSessionLogs(name string) (string, error)
	StartSession(name string, command []string, workspace string) (*runner.SessionState, error)
	GetSessionPath(name string) string
	IsProcessRunning(pid int) bool
	RemoveSession(name string, force bool) error
	RenameSession(oldName, newName string) error
	SessionsDir() string
	GetSessionGitDiffStat(name string) (string, error)
	ReplaySession(originalSessionName string) (*runner.SessionState, error)
}
