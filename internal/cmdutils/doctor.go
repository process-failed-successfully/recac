package cmdutils

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
	viperConfigFileUsed     = viper.ConfigFileUsed
	checkDockerConnectivity = checkDockerConnectivityFunc
)

// DockerClient defines the interface for Docker client operations needed by the doctor.
type DockerClient interface {
	Ping(ctx context.Context) (types.Ping, error)
}

// GetDoctor returns a string containing the results of the environment checks
// and a boolean indicating if all checks passed.
func GetDoctor() (string, bool) {
	var builder strings.Builder
	allPassed := true

	builder.WriteString("RECAC Doctor\n")
	builder.WriteString("------------\n")

	// Check 1: Configuration
	res, ok := checkConfig()
	builder.WriteString(res)
	if !ok {
		allPassed = false
	}

	// Check 2: Dependencies
	res, ok = checkDependencies()
	builder.WriteString(res)
	if !ok {
		allPassed = false
	}

	// Check 3: Docker Connectivity
	dockerCli, err := clientNewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	res, ok = checkDockerConnectivity(dockerCli, err)
	builder.WriteString(res)
	if !ok {
		allPassed = false
	}

	return builder.String(), allPassed
}

// FixConfig creates a default configuration file if one is missing.
func FixConfig() error {
	// Simple fix: create default config if missing
	viper.SetDefault("provider", "gemini")
	viper.SetDefault("model", "gemini-pro")
	return viper.SafeWriteConfig()
}

func checkConfig() (string, bool) {
	if cfgFile := viperConfigFileUsed(); cfgFile != "" {
		return fmt.Sprintf("[✔] Configuration: %s found\n", cfgFile), true
	}
	return "[✖] Configuration: Missing config file\n", false
}

func checkDependencies() (string, bool) {
	var builder strings.Builder
	passed := true
	dependencies := []string{"git", "docker", "go"}
	for _, dep := range dependencies {
		_, err := execLookPath(dep)
		if err != nil {
			builder.WriteString(fmt.Sprintf("[✖] Dependency: %s not found in PATH\n", dep))
			passed = false
		} else {
			builder.WriteString(fmt.Sprintf("[✔] Dependency: %s found in PATH\n", dep))
		}
	}
	return builder.String(), passed
}

func checkDockerConnectivityFunc(cli DockerClient, err error) (string, bool) {
	if err != nil {
		return fmt.Sprintf("[✖] Docker: Failed to create client: %v\n", err), false
	}

	_, err = cli.Ping(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "Is the docker daemon running?") {
			return "[✖] Docker: Daemon not running or socket permission error\n", false
		}
		return fmt.Sprintf("[✖] Docker: Failed to ping daemon: %v\n", err), false
	}

	return "[✔] Docker: Daemon is responsive\n", true
}
