package git

import "context"

// GitClient is an interface for interacting with Git.
type IClient interface {
	DiffStat(workspace, startCommit, endCommit string) (string, error)
	CurrentCommitSHA(workspace string) (string, error)
	Clone(ctx context.Context, repoURL, directory string) error
	RepoExists(directory string) bool
	Config(directory, key, value string) error
	ConfigAddGlobal(key, value string) error
	RemoteBranchExists(directory, remote, branch string) (bool, error)
	Fetch(directory, remote, branch string) error
	Checkout(directory, branch string) error
	CheckoutNewBranch(directory, branch string) error
	Push(directory, branch string) error
	Pull(directory, remote, branch string) error
	Stash(directory string) error
	Merge(directory, branchName string) error
	AbortMerge(directory string) error
	Recover(directory string) error
	Clean(directory string) error
	ResetHard(directory, remote, branch string) error
	StashPop(directory string) error
	DeleteRemoteBranch(directory, remote, branch string) error
	CurrentBranch(directory string) (string, error)
	Commit(directory, message string) error
	Diff(directory, startCommit, endCommit string) (string, error)
	DiffStaged(directory string) (string, error)
	SetRemoteURL(directory, name, url string) error
	DeleteLocalBranch(directory, branch string) error
	LocalBranchExists(directory, branch string) (bool, error)
	Log(directory string, args ...string) ([]string, error)

	// Bisect commands
	BisectStart(directory, badCommit, goodCommit string) error
	BisectGood(directory string) error
	BisectBad(directory string) error
	BisectSkip(directory string) error
	BisectReset(directory string) error
	BisectRun(directory, scriptPath string) (string, error)
	BisectLog(directory string) (string, error)
}
