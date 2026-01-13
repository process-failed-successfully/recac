package main

import (
	"recac/internal/git"
	"recac/internal/runner"
)

var (
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManager()
	}

	gitClientFactory = func() IGitClient {
		return git.NewClient()
	}
)
