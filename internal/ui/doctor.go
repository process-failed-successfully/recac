package ui

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"recac/internal/agent"
	"recac/internal/jira"

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
	newAgentFunc            = agent.NewAgent
	newJiraClientFunc       = func(url, user, token string) JiraClient { return jira.NewClient(url, user, token) }
	httpHeadFunc            = http.Head
	runCommand              = func(name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		out, err := cmd.Output()
		return string(out), err
	}
	viperGetString = viper.GetString
)

type JiraClient interface {
	Authenticate(ctx context.Context) error
}

type Agent interface {
	Send(ctx context.Context, prompt string) (string, error)
}

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

	// Check 4: LLM Connectivity
	builder.WriteString(checkLLM())

	// Check 5: Jira Connectivity
	builder.WriteString(checkJira())

	// Check 6: Network Connectivity
	builder.WriteString(checkNetwork())

	// Check 7: Git Configuration
	builder.WriteString(checkGitConfig())

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

func checkLLM() string {
	provider := viperGetString("agent_provider")
	model := viperGetString("agent_model")
	apiKey := viperGetString("api_key")

	if provider == "" {
		return "[✖] LLM: Missing 'agent_provider' in config\n"
	}
	if apiKey == "" {
		// Some providers (like Ollama) might not need a key, but usually they do or it's ignored.
		// Let's warn if empty but proceed if provider is local like ollama?
		// Actually, ollama doesn't require api_key.
		if provider != "ollama" {
			return "[✖] LLM: Missing 'api_key' in config\n"
		}
	}

	// Use current directory for workDir and "doctor" for project name
	workDir := "."
	project := "doctor-check"

	ag, err := newAgentFunc(provider, apiKey, model, workDir, project)
	if err != nil {
		return fmt.Sprintf("[✖] LLM: Failed to initialize agent: %v\n", err)
	}

	// Send a simple prompt to verify connectivity
	// We use a very short prompt to minimize cost/time
	ctx := context.Background()
	_, err = ag.Send(ctx, "Reply with 'ok'")
	if err != nil {
		return fmt.Sprintf("[✖] LLM: Connection failed: %v\n", err)
	}

	return fmt.Sprintf("[✔] LLM: Connected to %s\n", provider)
}

func checkJira() string {
	jiraURL := viperGetString("jira_url")
	jiraUsername := viperGetString("jira_email")
	jiraToken := viperGetString("jira_token")

	if jiraURL == "" {
		return "[✖] Jira: Missing 'jira_url' in config\n"
	}
	if jiraUsername == "" || jiraToken == "" {
		return "[✖] Jira: Missing 'jira_email' or 'jira_token' in config\n"
	}

	client := newJiraClientFunc(jiraURL, jiraUsername, jiraToken)
	err := client.Authenticate(context.Background())
	if err != nil {
		return fmt.Sprintf("[✖] Jira: Authentication failed: %v\n", err)
	}

	return "[✔] Jira: Authenticated\n"
}

func checkNetwork() string {
	_, err := httpHeadFunc("https://github.com")
	if err != nil {
		return fmt.Sprintf("[✖] Network: Failed to reach GitHub: %v\n", err)
	}
	return "[✔] Network: Internet connectivity OK\n"
}

func checkGitConfig() string {
	var builder strings.Builder

	// Check user.name
	out, err := runCommand("git", "config", "user.name")
	if err != nil || strings.TrimSpace(out) == "" {
		builder.WriteString("[✖] Git: 'user.name' not configured\n")
	} else {
		builder.WriteString(fmt.Sprintf("[✔] Git: user.name=%s\n", strings.TrimSpace(out)))
	}

	// Check user.email
	out, err = runCommand("git", "config", "user.email")
	if err != nil || strings.TrimSpace(out) == "" {
		builder.WriteString("[✖] Git: 'user.email' not configured\n")
	} else {
		builder.WriteString(fmt.Sprintf("[✔] Git: user.email=%s\n", strings.TrimSpace(out)))
	}

	return builder.String()
}
