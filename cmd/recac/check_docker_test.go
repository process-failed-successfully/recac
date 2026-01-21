package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockCheckDockerClient struct {
	checkDaemonErr error
	checkSocketErr error
	checkImageRef  string
	checkImageRes  bool
	checkImageErr  error
	pullImageRef   string
	pullImageErr   error
	closeErr       error
}

func (m *mockCheckDockerClient) CheckDaemon(ctx context.Context) error {
	return m.checkDaemonErr
}

func (m *mockCheckDockerClient) CheckSocket(ctx context.Context) error {
	return m.checkSocketErr
}

func (m *mockCheckDockerClient) CheckImage(ctx context.Context, imageRef string) (bool, error) {
	m.checkImageRef = imageRef
	return m.checkImageRes, m.checkImageErr
}

func (m *mockCheckDockerClient) PullImage(ctx context.Context, imageRef string) error {
	m.pullImageRef = imageRef
	return m.pullImageErr
}

func (m *mockCheckDockerClient) Close() error {
	return m.closeErr
}

func TestCheckDockerCmd(t *testing.T) {
	// Restore factory after test
	originalFactory := checkDockerClientFactory
	defer func() { checkDockerClientFactory = originalFactory }()

	t.Run("Default Image", func(t *testing.T) {
		mock := &mockCheckDockerClient{
			checkImageRes: true,
		}
		checkDockerClientFactory = func(project string) (checkDockerClient, error) {
			return mock, nil
		}

		// Use executeCommand helper which handles root command setup and output capture
		_, err := executeCommand(rootCmd, "check-docker")
		assert.NoError(t, err)
		assert.Equal(t, "ghcr.io/process-failed-successfully/recac-agent:latest", mock.checkImageRef)
	})

	t.Run("Custom Image", func(t *testing.T) {
		mock := &mockCheckDockerClient{
			checkImageRes: true,
		}
		checkDockerClientFactory = func(project string) (checkDockerClient, error) {
			return mock, nil
		}

		_, err := executeCommand(rootCmd, "check-docker", "--image", "custom-image:tag")
		assert.NoError(t, err)
		assert.Equal(t, "custom-image:tag", mock.checkImageRef)
	})

	t.Run("Missing Image AutoFix", func(t *testing.T) {
		mock := &mockCheckDockerClient{
			checkImageRes: false, // Missing
		}
		checkDockerClientFactory = func(project string) (checkDockerClient, error) {
			return mock, nil
		}

		_, err := executeCommand(rootCmd, "check-docker", "--auto-fix")
		assert.NoError(t, err)

		// It should attempt to pull
		assert.Equal(t, "ghcr.io/process-failed-successfully/recac-agent:latest", mock.pullImageRef)
	})
}
