package main

import "recac/internal/runner"

// ISessionManager defines the interface for session management.
type ISessionManager interface {
	ListSessions() ([]*runner.SessionState, error)
	SaveSession(*runner.SessionState) error
	LoadSession(name string) (*runner.SessionState, error)
	StopSession(name string) error
	PauseSession(name string) error
	ResumeSession(name string) error
	GetSessionLogs(name string) (string, error)
	GetSessionLogContent(name string, lines int) (string, error)
	StartSession(name string, command []string, workspace string) (*runner.SessionState, error)
	GetSessionPath(name string) string
	IsProcessRunning(pid int) bool
	RemoveSession(name string, force bool) error
	RenameSession(oldName, newName string) error
	SessionsDir() string
	GetSessionGitDiffStat(name string) (string, error)
	ArchiveSession(name string) error
	UnarchiveSession(name string) error
	ListArchivedSessions() ([]*runner.SessionState, error)
}

// IGitClient defines the interface for git operations.
type IGitClient interface {
	Checkout(repoPath, commitOrBranch string) error
	Diff(repoPath, commitA, commitB string) (string, error)
	DiffStat(repoPath, commitA, commitB string) (string, error)
	CurrentCommitSHA(repoPath string) (string, error)
}
