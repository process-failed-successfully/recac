package main

import (
	"recac/internal/git"
	"recac/internal/runner"
)

// sessionManagerFactory is a variable that holds a function to create a session manager.
// This allows us to override it in tests to inject a mock session manager.
var sessionManagerFactory = func() (ISessionManager, error) {
	return runner.NewSessionManager()
}

// gitClientFactory is a factory function that can be overridden in tests.
var gitClientFactory = func() IGitClient {
	return git.NewClient()
}
