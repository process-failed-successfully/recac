package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
)

func TestClient_ServerVersion(t *testing.T) {
	c, mock := NewMockClient()

	mock.ServerVersionFunc = func(ctx context.Context) (types.Version, error) {
		return types.Version{Version: "mock-v1"}, nil
	}

	v, err := c.ServerVersion(context.Background())
	if err != nil {
		t.Fatalf("ServerVersion failed: %v", err)
	}
	if v.Version != "mock-v1" {
		t.Errorf("Expected version mock-v1, got %s", v.Version)
	}
}

func TestClient_CheckDaemon(t *testing.T) {
	c, mock := NewMockClient()

	// Test success
	mock.PingFunc = func(ctx context.Context) (types.Ping, error) {
		return types.Ping{}, nil
	}
	if err := c.CheckDaemon(context.Background()); err != nil {
		t.Errorf("CheckDaemon failed: %v", err)
	}

	// Test failure
	mock.PingFunc = func(ctx context.Context) (types.Ping, error) {
		return types.Ping{}, errors.New("daemon down")
	}
	if err := c.CheckDaemon(context.Background()); err == nil {
		t.Error("CheckDaemon should have failed")
	}
}

func TestClient_CheckSocket(t *testing.T) {
	c, mock := NewMockClient()

	// Test success
	mock.PingFunc = func(ctx context.Context) (types.Ping, error) {
		return types.Ping{}, nil
	}
	if err := c.CheckSocket(context.Background()); err != nil {
		t.Errorf("CheckSocket failed: %v", err)
	}

	// Test failure
	mock.PingFunc = func(ctx context.Context) (types.Ping, error) {
		return types.Ping{}, errors.New("socket unreachable")
	}
	if err := c.CheckSocket(context.Background()); err == nil {
		t.Error("CheckSocket should have failed")
	}
}

func TestClient_ImageExists(t *testing.T) {
	c, mock := NewMockClient()

	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{
			{
				RepoTags: []string{"my-image:latest", "other:tag"},
			},
		}, nil
	}

	// Test exists
	exists, err := c.ImageExists(context.Background(), "my-image:latest")
	if err != nil {
		t.Fatalf("ImageExists failed: %v", err)
	}
	if !exists {
		t.Error("ImageExists returned false for existing image")
	}

	// Test not exists
	exists, err = c.ImageExists(context.Background(), "not-exists:latest")
	if err != nil {
		t.Fatalf("ImageExists failed: %v", err)
	}
	if exists {
		t.Error("ImageExists returned true for non-existing image")
	}
}

func TestClient_CheckImage(t *testing.T) {
	c, mock := NewMockClient()

	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{
			{
				ID:       "sha256:1234567890123456",
				RepoTags: []string{"my-image:latest"},
			},
		}, nil
	}

	// Test exists by tag
	exists, err := c.CheckImage(context.Background(), "my-image:latest")
	if err != nil {
		t.Fatalf("CheckImage failed: %v", err)
	}
	if !exists {
		t.Error("CheckImage returned false for existing tag")
	}

	// Test exists by ID (assuming ID match includes prefix if logic requires it)
	// The current implementation compares imageRef == img.ID[:12]
	// So we need to provide the prefix part.
	// img.ID is sha256:1234567890123456
	// img.ID[:12] is sha256:12345
	exists, err = c.CheckImage(context.Background(), "sha256:12345")
	if err != nil {
		t.Fatalf("CheckImage failed: %v", err)
	}
	if !exists {
		t.Error("CheckImage returned false for existing ID")
	}

	// Test failure
	exists, err = c.CheckImage(context.Background(), "missing:latest")
	if err != nil {
		t.Fatalf("CheckImage failed: %v", err)
	}
	if exists {
		t.Error("CheckImage returned true for missing image")
	}
}

func TestClient_PullImage(t *testing.T) {
	c, mock := NewMockClient()

	// Test success
	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("{}")), nil
	}
	if err := c.PullImage(context.Background(), "img:tag"); err != nil {
		t.Errorf("PullImage failed: %v", err)
	}

	// Test failure on API call
	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return nil, errors.New("pull error")
	}
	if err := c.PullImage(context.Background(), "img:tag"); err == nil {
		t.Error("PullImage should have failed")
	}
}
