package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"recac/internal/docker"

	"github.com/spf13/viper"
)

// Function variables for mocking
var (
	execLookPath    = exec.LookPath
	newDockerClient = func(project string) (DockerClient, error) {
		return docker.NewClient(project)
	}
	viperConfigFileUsed     = viper.ConfigFileUsed
	checkDockerConnectivity = checkDockerConnectivityFunc
)

// DockerClient defines the interface for Docker client operations needed by the doctor.
type DockerClient interface {
	CheckDaemon(ctx context.Context) error
	CheckSocket(ctx context.Context) error
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

	// Check 3: Docker Connectivity
	dockerCli, err := newDockerClient("recac-doctor")
	builder.WriteString(checkDockerConnectivity(dockerCli, err))

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

func checkDockerConnectivityFunc(cli DockerClient, err error) string {
	if err != nil {
		return fmt.Sprintf("[✖] Docker: Failed to create client: %v\n", err)
	}
	defer cli.Close()

	ctx := context.Background()

	// Check Daemon
	if err := cli.CheckDaemon(ctx); err != nil {
		return fmt.Sprintf("[✖] Docker: Daemon not reachable: %v\n", err)
	}

	// Check Socket
	if err := cli.CheckSocket(ctx); err != nil {
		return fmt.Sprintf("[✖] Docker: Socket not accessible: %v\n", err)
	}

	return "[✔] Docker: Daemon is responsive\n"
}
