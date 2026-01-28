package main

import (
	"context"
	"io"
	"recac/internal/runner"

	corev1 "k8s.io/api/core/v1"
	k8sversion "k8s.io/apimachinery/pkg/version"
)

// IDockerClient defines the interface for Docker operations used by CLI commands.
// This allows mocking the Docker client in tests.
type IDockerClient interface {
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	Close() error
}

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
	StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error)
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
	DiffStaged(repoPath string) (string, error)
	DiffStat(repoPath, commitA, commitB string) (string, error)
	CurrentCommitSHA(repoPath string) (string, error)
	RepoExists(repoPath string) bool
	Commit(repoPath, message string) error
	Log(repoPath string, args ...string) ([]string, error)
	Fetch(repoPath, remote, branch string) error
	CurrentBranch(repoPath string) (string, error)
	CheckoutNewBranch(repoPath, branch string) error
	BisectStart(repoPath, bad, good string) error
	BisectGood(repoPath, rev string) error
	BisectBad(repoPath, rev string) error
	BisectReset(repoPath string) error
	BisectLog(repoPath string) ([]string, error)
	Tag(repoPath, version string) error
	DeleteTag(repoPath, version string) error
	PushTags(repoPath string) error
	LatestTag(repoPath string) (string, error)
	Run(repoPath string, args ...string) (string, error)
	DeleteLocalBranch(repoPath, branch string) error
	CreatePR(repoPath, title, body, base string) (string, error)
}

// IK8sClient defines the interface for Kubernetes operations.
type IK8sClient interface {
	ListPods(ctx context.Context, labelSelector string) ([]corev1.Pod, error)
	DeletePod(ctx context.Context, name string) error
	GetServerVersion() (*k8sversion.Info, error)
	GetNamespace() string
	GetPodLogs(ctx context.Context, name string, follow bool) (io.ReadCloser, error)
}
