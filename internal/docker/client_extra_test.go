package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestExecAsUser_Success(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		if config.User != "testuser" {
			t.Errorf("Expected user testuser, got %s", config.User)
		}
		return types.IDResponse{ID: "exec-id"}, nil
	}

	_, err := client.ExecAsUser(context.Background(), "container-id", "testuser", []string{"ls"})
	if err != nil {
		t.Fatalf("ExecAsUser failed: %v", err)
	}
}

func TestExec_CreateError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		return types.IDResponse{}, errors.New("create failed")
	}

	_, err := client.Exec(context.Background(), "container-id", []string{"ls"})
	if err == nil {
		t.Fatal("Exec expected error, got nil")
	}
}

func TestExec_AttachError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		return types.HijackedResponse{}, errors.New("attach failed")
	}

	_, err := client.Exec(context.Background(), "container-id", []string{"ls"})
	if err == nil {
		t.Fatal("Exec expected error, got nil")
	}
}

func TestRunContainer_Errors(t *testing.T) {
	client, mock := NewMockClient()

	// Test Create Error
	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		return container.CreateResponse{}, errors.New("create error")
	}

	_, err := client.RunContainer(context.Background(), "image", "/tmp", nil, "")
	if err == nil {
		t.Error("Expected error for create failure")
	}

	// Test Start Error
	mock.ContainerCreateFunc = nil // Reset
	mock.ContainerStartFunc = func(ctx context.Context, containerID string, options container.StartOptions) error {
		return errors.New("start error")
	}

		_, err = client.RunContainer(context.Background(), "image", "/tmp", nil, "")

		if err == nil {

			t.Error("Expected error for start failure")

		}

	}

	

	func TestCheckImage_Complex(t *testing.T) {

		client, mock := NewMockClient()

	

			mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {

	

				return []image.Summary{

	

					{

	

						ID:       "sha256:1234567890ab",

	

						RepoTags: []string{"myimage:v1"},

	

					},

	

				}, nil

	

			}

	

		

	

			// Match by ID short (must match prefix of ID including sha256:)

	

			// sha256:12345 is 12 chars

	

			exists, _ := client.CheckImage(context.Background(), "sha256:12345")

	

			if !exists {

	

				t.Error("Expected match by ID")

	

			}

	

		

	

		// Match by implicit latest

		mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {

			return []image.Summary{

				{RepoTags: []string{"myimage:latest"}},

			}, nil

		}

		exists, _ = client.CheckImage(context.Background(), "myimage")

		if !exists {

			t.Error("Expected match by implicit latest")

		}

	}

	

	func TestRunContainer_Pull(t *testing.T) {

		client, mock := NewMockClient()

		

		pullCalled := false

		mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {

			pullCalled = true

			return io.NopCloser(strings.NewReader("{}")), nil

		}

	

		client.RunContainer(context.Background(), "image", "/tmp", nil, "")

			if !pullCalled {

				t.Error("Expected ImagePull to be called")

			}

		}

		

		func TestPullImage_Errors(t *testing.T) {

			client, mock := NewMockClient()

		

			// Test JSON Error in stream

			mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {

				return io.NopCloser(strings.NewReader(`{"errorDetail": {"message": "pull error"}}`)), nil

			}

		

			if err := client.PullImage(context.Background(), "image"); err == nil {

				t.Error("Expected error for pull failure")

			}

		

			// Test Malformed JSON (should be ignored/continue?)

			// The code says: "Continue parsing even if one message fails"

			mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {

				return io.NopCloser(strings.NewReader(`{malformed`)), nil

			}

			if err := client.PullImage(context.Background(), "image"); err != nil {

				t.Errorf("Expected success (ignoring malformed), got: %v", err)

			}

		}

		

		func TestNewClient_Defaults(t *testing.T) {

			// This creates a real client, might fail if no docker.

			// But NewClient usually succeeds in creating the struct.

			c, err := NewClient("")

			if err == nil {

				defer c.Close()

				if c.project != "unknown" {

					t.Errorf("Expected default project 'unknown', got '%s'", c.project)

				}

			} else {

				t.Logf("Skipping NewClient test (docker not available?): %v", err)

			}

		}

		

	