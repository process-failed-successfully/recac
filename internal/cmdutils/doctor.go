package cmdutils

import (
	"context"
	"fmt"
	"os/exec"
	"recac/internal/agent"
	"strings"
	"time"

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
	newAgent                = agent.NewAgent
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

	// Check 4: AI Provider Connectivity
	builder.WriteString(checkAIProvider())

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

	_, err = cli.Ping(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "Is the docker daemon running?") {
			return "[✖] Docker: Daemon not running or socket permission error\n"
		}
		return fmt.Sprintf("[✖] Docker: Failed to ping daemon: %v\n", err)
	}

	return "[✔] Docker: Daemon is responsive\n"
}

func checkAIProvider() string {
	provider := viper.GetString("provider")
	if provider == "" {
		provider = viper.GetString("agent_provider")
	}

	model := viper.GetString("model")
	if model == "" {
		model = viper.GetString("agent_model")
	}

	apiKey := viper.GetString("secrets.api_key")
	if apiKey == "" {
		// Try common secret locations
		apiKey = viper.GetString("secrets.openrouterApiKey")
		if apiKey == "" {
			apiKey = viper.GetString("secrets.apiKey")
		}
	}

	if provider == "" {
		return "[?] AI: Provider not configured (check .recac.yaml)\n"
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize agent
	// Note: We use "doctor-check" as project name
	ag, err := newAgent(provider, apiKey, model, "", "doctor-check")
	if err != nil {
		return fmt.Sprintf("[✖] AI: Failed to initialize agent (%s/%s): %v\n", provider, model, err)
	}

	// Send test prompt
	start := time.Now()
	resp, err := ag.Send(ctx, "Hello, are you working?")
	if err != nil {
		return fmt.Sprintf("[✖] AI: Connection failed (%s/%s): %v\n", provider, model, err)
	}
	latency := time.Since(start)

	// Escape newlines to keep formatting clean before truncating
	cleanResp := strings.ReplaceAll(resp, "\n", " ")

	// Truncate response for display
	displayResp := cleanResp
	if len(displayResp) > 50 {
		displayResp = displayResp[:50] + "..."
	}

	return fmt.Sprintf("[✔] AI: Connected to %s/%s (Latency: %v)\n    Response: %q\n", provider, model, latency, displayResp)
}
