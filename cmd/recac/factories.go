package main

import (
	"context"
	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/docker"
	"recac/internal/git"
	"recac/internal/k8s"
	"recac/internal/runner"
)

// dockerClientFactory is a factory function that can be overridden in tests.
var dockerClientFactory = func(project string) (IDockerClient, error) {
	return docker.NewClient(project)
}

// sessionManagerFactory is a variable that holds a function to create a session manager.
// This allows us to override it in tests to inject a mock session manager.
var sessionManagerFactory = func() (ISessionManager, error) {
	return runner.NewSessionManager()
}

// gitClientFactory is a factory function that can be overridden in tests.
var gitClientFactory = func() IGitClient {
	return git.NewClient()
}

// agentClientFactory is a factory function that can be overridden in tests.
var agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
	return cmdutils.GetAgentClient(ctx, provider, model, projectPath, projectName)
}

// k8sClientFactory is a factory function that can be overridden in tests.
var k8sClientFactory = func() (IK8sClient, error) {
	return k8s.NewClient()
}
