package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"recac/internal/docker"

	"github.com/spf13/viper"
)

// Function variables for mocking
var (
	execLookPath        = exec.LookPath
	dockerClientFactory = func() (DockerClient, error) {
		return docker.NewClient("recac-doctor")
	}
	viperConfigFileUsed     = viper.ConfigFileUsed
	checkDockerConnectivity = checkDockerConnectivityFunc
	checkDiskSpaceFunc      = checkDiskSpace
)

// DockerClient defines the interface for Docker client operations needed by the doctor.
type DockerClient interface {
	CheckDaemon(ctx context.Context) error
	CheckSocket(ctx context.Context) error
	CheckImage(ctx context.Context, imageRef string) (bool, error)
	Close() error
}

// GetDoctor returns a string containing the results of the environment checks.
func GetDoctor() string {
	var builder strings.Builder

	builder.WriteString("RECAC Doctor\n")
	builder.WriteString("------------\n")

	// Check 1: Configuration
	builder.WriteString(checkConfig())

	// Check 2: Dependencies
	builder.WriteString(checkDependencies())

	// Check 3: Disk Space
	builder.WriteString(checkDiskSpaceReport())

	// Check 4: Docker Connectivity & Requirements
	dockerCli, err := dockerClientFactory()
	builder.WriteString(checkDockerConnectivity(dockerCli, err))
	if err == nil {
		defer dockerCli.Close()
	}

	return builder.String()
}

func checkConfig() string {
	if cfgFile := viperConfigFileUsed(); cfgFile != "" {
		return fmt.Sprintf("[✔] Configuration: %s found\n", cfgFile)
	}
	return "[✖] Configuration: Missing config file\n"
}

func checkDependencies() string {
	var builder strings.Builder
	dependencies := []string{"git", "docker"}
	for _, dep := range dependencies {
		_, err := execLookPath(dep)
		if err != nil {
			builder.WriteString(fmt.Sprintf("[✖] Dependency: %s not found in PATH\n", dep))
		} else {
			builder.WriteString(fmt.Sprintf("[✔] Dependency: %s found in PATH\n", dep))
		}
	}
	return builder.String()
}

func checkDiskSpaceReport() string {
	ok, err := checkDiskSpaceFunc()
	if err != nil {
		return fmt.Sprintf("[✖] Disk Space: Could not check disk space: %v\n", err)
	}
	if !ok {
		return "[✖] Disk Space: Low disk space detected (< 1GB free)\n"
	}
	return "[✔] Disk Space: Sufficient disk space available\n"
}

// checkDiskSpace checks if there's sufficient disk space available.
// Returns true if disk space is OK (> 1GB free), false otherwise.
func checkDiskSpace() (bool, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return false, err
	}

	// Calculate free space in bytes
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	// Convert to GB (1GB = 1024^3 bytes)
	freeGB := float64(freeBytes) / (1024 * 1024 * 1024)

	// Check if we have at least 1GB free
	if freeGB < 1.0 {
		return false, nil
	}

	return true, nil
}

func checkDockerConnectivityFunc(cli DockerClient, err error) string {
	if err != nil {
		return fmt.Sprintf("[✖] Docker: Failed to create client: %v\n", err)
	}

	var builder strings.Builder
	ctx := context.Background()

	// Check Daemon
	if err := cli.CheckDaemon(ctx); err != nil {
		if strings.Contains(err.Error(), "Is the docker daemon running?") {
			return "[✖] Docker: Daemon not running or socket permission error\n"
		}
		return fmt.Sprintf("[✖] Docker: Daemon not reachable: %v\n", err)
	}
	builder.WriteString("[✔] Docker: Daemon is responsive\n")

	// Check Socket
	if err := cli.CheckSocket(ctx); err != nil {
		builder.WriteString(fmt.Sprintf("[✖] Docker: Socket is not accessible: %v\n", err))
	} else {
		builder.WriteString("[✔] Docker: Socket is accessible\n")
	}

	// Check Required Image
	// We use the default image for now. In the future, we might want to pass this in or read from config.
	imageRef := "ghcr.io/process-failed-successfully/recac-agent:latest"
	exists, err := cli.CheckImage(ctx, imageRef)
	if err != nil {
		builder.WriteString(fmt.Sprintf("[✖] Docker: Error checking image %s: %v\n", imageRef, err))
	} else if !exists {
		builder.WriteString(fmt.Sprintf("[✖] Docker: Required image %s is missing\n", imageRef))
	} else {
		builder.WriteString(fmt.Sprintf("[✔] Docker: Required image %s is present\n", imageRef))
	}

	return builder.String()
}
