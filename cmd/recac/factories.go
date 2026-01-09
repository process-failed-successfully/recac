package main

import "recac/internal/runner"

var (
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManager()
	}
)
