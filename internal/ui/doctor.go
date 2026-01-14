package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/viper"
)

// Function variables for mocking
var (
	execLookPath            = exec.LookPath
	clientNewClientWithOpts = client.NewClientWithOpts
)

// DockerClient defines the interface for Docker client operations needed by the doctor.
type DockerClient interface {
	Ping(ctx context.Context) (types.Ping, error)
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
	dockerCli, err := clientNewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	builder.WriteString(checkDockerConnectivity(dockerCli, err))

	return builder.String()
}

func checkConfig() string {
	if cfgFile := viper.ConfigFileUsed(); cfgFile != "" {
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

func checkDockerConnectivity(cli DockerClient, err error) string {
	if err != nil {
		return fmt.Sprintf("[✖] Docker: Failed to create client: %v\n", err)
	}

	_, err = cli.Ping(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "Is the docker daemon running?") {
			return "[✖] Docker: Daemon not running or socket permission error\n"
		}
		return fmt.Sprintf("[✖] Docker: Failed to ping daemon: %v\n", err)
	}

	return "[✔] Docker: Daemon is responsive\n"
}
