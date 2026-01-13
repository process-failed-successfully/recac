package git

import "context"

// GitClient is an interface for interacting with Git.
type GitClient interface {
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
}
