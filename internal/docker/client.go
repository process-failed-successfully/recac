package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// APIClient defines the subset of Docker API methods we use.
// This allows for mocking in tests.
type APIClient interface {
	Ping(ctx context.Context) (types.Ping, error)
	ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error)
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	Close() error
}

// Client wraps the official Docker client to provide high-level orchestration methods.
type Client struct {
	api APIClient
}

// NewClient creates a new Docker client instance.
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &Client{api: cli}, nil
}

// Close closes the underlying docker client connection.
func (c *Client) Close() error {
	return c.api.Close()
}

// CheckDaemon verifies that the Docker daemon is running and reachable.
func (c *Client) CheckDaemon(ctx context.Context) error {
	_, err := c.api.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker daemon is not reachable: %w", err)
	}
	return nil
}

// CheckSocket verifies that the Docker socket is accessible.
// This is essentially the same as CheckDaemon, but provides a more specific error message.
func (c *Client) CheckSocket(ctx context.Context) error {
	_, err := c.api.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker socket is not accessible: %w", err)
	}
	return nil
}

// CheckImage verifies that a required Docker image exists locally.
// Returns true if the image exists, false otherwise.
func (c *Client) CheckImage(ctx context.Context, imageRef string) (bool, error) {
	images, err := c.api.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list images: %w", err)
	}

	// Normalize image reference: if no tag specified, assume :latest
	normalizedRef := imageRef
	if !strings.Contains(imageRef, ":") {
		normalizedRef = imageRef + ":latest"
	}

	// Check if the image exists by comparing repository tags
	for _, img := range images {
		for _, tag := range img.RepoTags {
			// Exact match
			if tag == imageRef || tag == normalizedRef {
				return true, nil
			}
		}
		// Check by image ID (short or full)
		if len(img.ID) >= 12 && len(imageRef) >= 12 && imageRef == img.ID[:12] {
			return true, nil
		}
		if imageRef == img.ID {
			return true, nil
		}
	}

	return false, nil
}

// PullImage pulls a Docker image from the registry.
// It returns an error if the pull fails.
// Progress logging should be handled by the caller.
func (c *Client) PullImage(ctx context.Context, imageRef string) error {
	reader, err := c.api.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}
	defer reader.Close()

	// Parse pull output to check for errors
	decoder := json.NewDecoder(reader)
	for {
		var msg jsonmessage.JSONMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			// Continue parsing even if one message fails
			continue
		}

		// Check for pull errors
		if msg.Error != nil {
			return fmt.Errorf("pull failed: %s", msg.Error.Message)
		}
	}

	return nil
}

// RunContainer starts a container with the specified image and mounts the workspace.
// It returns the container ID or an error.
func (c *Client) RunContainer(ctx context.Context, imageRef string, workspace string) (string, error) {
	// 1. Pull Image (Best effort)
	reader, err := c.api.ImagePull(ctx, imageRef, image.PullOptions{})
	if err == nil {
		defer reader.Close()
		io.Copy(io.Discard, reader) // Drain output
	}

	// 2. Create Container
	resp, err := c.api.ContainerCreate(ctx,
		&container.Config{
			Image:      imageRef,
			Tty:        true,       // Keep it running
			OpenStdin:  true,       // Keep stdin open
			WorkingDir: "/workspace",
			Cmd:        []string{"/bin/sh"}, // Default command to keep it alive
		},
		&container.HostConfig{
			Binds: []string{
				fmt.Sprintf("%s:/workspace", workspace),
			},
		}, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 3. Start Container
	if err := c.api.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

// Exec executes a command in a running container and returns the output (stdout + stderr).
func (c *Client) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	respID, err := c.api.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := c.api.ContainerExecAttach(ctx, respID.ID, container.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach exec: %w", err)
	}
	defer resp.Close()

	var outBuf, errBuf bytes.Buffer
	// stdcopy.StdCopy demultiplexes the stream if Tty is false. 
	// If Tty is true in ExecConfig, it's a raw stream.
	// We didn't set Tty in ExecConfig, so it defaults to false.
	_, err = stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to copy exec output: %w", err)
	}

	return outBuf.String() + errBuf.String(), nil
}

// StopContainer stops and removes the container.
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	// Stop
	if err := c.api.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		// Just log error?
	}
	
	// Remove
	return c.api.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

// ImageBuildOptions configures how an image is built.
type ImageBuildOptions struct {
	// BuildContext is the tar stream containing the build context.
	BuildContext io.Reader
	// Dockerfile is the path to the Dockerfile within the build context (default: "Dockerfile").
	Dockerfile string
	// Tag is the image tag to apply (e.g., "myimage:latest").
	Tag string
	// BuildArgs are build-time variables (e.g., map[string]*string{"VERSION": "1.0"}).
	BuildArgs map[string]*string
	// NoCache disables build cache if true.
	NoCache bool
}

// ImageBuild builds a Docker image from a build context and returns the image ID.
// The build progress is logged via the provided logger function (if non-nil).
func (c *Client) ImageBuild(ctx context.Context, opts ImageBuildOptions) (string, error) {
	if opts.BuildContext == nil {
		return "", fmt.Errorf("build context is required")
	}
	if opts.Tag == "" {
		return "", fmt.Errorf("image tag is required")
	}
	if opts.Dockerfile == "" {
		opts.Dockerfile = "Dockerfile"
	}

	buildOptions := build.ImageBuildOptions{
		Dockerfile: opts.Dockerfile,
		Tags:       []string{opts.Tag},
		BuildArgs:  opts.BuildArgs,
		NoCache:    opts.NoCache,
		Remove:     true, // Remove intermediate containers
	}

	resp, err := c.api.ImageBuild(ctx, opts.BuildContext, buildOptions)
	if err != nil {
		return "", fmt.Errorf("failed to start image build: %w", err)
	}
	defer resp.Body.Close()

	// Parse build output to extract image ID
	var imageID string
	decoder := json.NewDecoder(resp.Body)
	for {
		var msg jsonmessage.JSONMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			// Continue parsing even if one message fails
			continue
		}

		// Check for build errors
		if msg.Error != nil {
			return "", fmt.Errorf("build failed: %s", msg.Error.Message)
		}

		// Extract image ID from "Successfully built" message
		if msg.Stream != "" {
			if bytes.Contains([]byte(msg.Stream), []byte("Successfully built")) {
				// Try to extract image ID from stream
				// Format: "Successfully built <image-id>\n"
				parts := bytes.Fields([]byte(msg.Stream))
				if len(parts) >= 2 {
					imageID = string(parts[len(parts)-1])
				}
			}
		}

		// Also check Aux field for image ID
		if msg.Aux != nil {
			var aux map[string]interface{}
			if err := json.Unmarshal(*msg.Aux, &aux); err == nil {
				if id, ok := aux["ID"].(string); ok && id != "" {
					imageID = id
				}
			}
		}
	}

	if imageID == "" {
		// If we couldn't extract image ID, try to infer from tag
		// This is a fallback - ideally we should always get it from build output
		return opts.Tag, nil
	}

	return imageID, nil
}