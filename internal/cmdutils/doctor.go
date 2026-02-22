package cmdutils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/viper"
)

// CheckResult represents the result of a single check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "passed", "failed", "warning"
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// Function variables for mocking
var (
	execLookPath            = exec.LookPath
	viperConfigFileUsed     = viper.ConfigFileUsed
	newDockerClient         = func() (DockerClient, error) {
		return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	}
	// Deprecated: kept for backward compatibility if needed, but not used in RunChecks
	clientNewClientWithOpts = client.NewClientWithOpts
	// Deprecated: kept for backward compatibility
	checkDockerConnectivity = checkDockerConnectivityFunc
)

// DockerClient defines the interface for Docker client operations needed by the doctor.
type DockerClient interface {
	Ping(ctx context.Context) (types.Ping, error)
}

// RunChecks performs all checks and returns a slice of results.
func RunChecks() []CheckResult {
	var results []CheckResult

	// Helper function
	addResult := func(name string, err error, successMsg string) {
		status := "passed"
		msg := successMsg
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
			msg = ""
		}
		results = append(results, CheckResult{
			Name:    name,
			Status:  status,
			Message: msg,
			Error:   errMsg,
		})
	}

	// 1. Configuration
	if cfgFile := viperConfigFileUsed(); cfgFile != "" {
		addResult("Configuration", nil, fmt.Sprintf("Found at %s", cfgFile))
	} else {
		addResult("Configuration", fmt.Errorf("Config file not found"), "")
	}

	// 2. Dependencies
	dependencies := []string{"git", "docker", "go"}
	for _, dep := range dependencies {
		path, err := execLookPath(dep)
		if err != nil {
			addResult("Dependency: "+dep, fmt.Errorf("Not found in PATH"), "")
		} else {
			addResult("Dependency: "+dep, nil, fmt.Sprintf("Found at %s", path))
		}
	}

	// 3. Docker Connectivity
	dockerCli, err := newDockerClient()
	msg, err := checkDockerConnectivityFuncInternal(dockerCli, err)
	addResult("Docker Daemon", err, msg)

	// 4. Kubernetes (Optional)
	if _, err := execLookPath("kubectl"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "kubectl", "version", "--client")
		out, err := cmd.CombinedOutput()
		if err != nil {
			results = append(results, CheckResult{
				Name:    "Kubernetes",
				Status:  "warning",
				Message: "kubectl found but error checking version",
				Error:   err.Error(),
			})
		} else {
			lines := strings.Split(string(out), "\n")
			version := ""
			if len(lines) > 0 {
				version = lines[0]
			}
			addResult("Kubernetes", nil, fmt.Sprintf("kubectl found: %s", version))
		}
	}

	// 5. Jira Connectivity
	ctx := context.Background()
	jiraClient, err := GetJiraClient(ctx)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Jira API",
			Status:  "warning",
			Message: "Not configured",
			Error:   err.Error(),
		})
	} else {
		ctxAuth, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := jiraClient.Authenticate(ctxAuth); err != nil {
			addResult("Jira API", fmt.Errorf("Authentication failed: %w", err), "")
		} else {
			addResult("Jira API", nil, fmt.Sprintf("Connected as %s", jiraClient.Username))
		}
	}

	// 6. AI Provider Connectivity
	provider := viper.GetString("provider")
	if provider == "" {
		provider = "gemini" // Default
	}
	model := viper.GetString("model")
	agentClient, err := GetAgentClient(ctx, provider, model, "", "recac-doctor")
	if err != nil {
		addResult("AI Provider", fmt.Errorf("Failed to initialize client: %w", err), "")
	} else {
		ctxPing, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		resp, err := agentClient.Send(ctxPing, "Reply with 'ok'")
		if err != nil {
			addResult("AI Provider", fmt.Errorf("Ping failed: %w", err), "")
		} else {
			cleanResp := strings.TrimSpace(resp)
			if len(cleanResp) > 50 {
				cleanResp = cleanResp[:47] + "..."
			}
			addResult("AI Provider", nil, fmt.Sprintf("Connected to %s (Response: %s)", provider, cleanResp))
		}
	}

	// 7. Workspace
	cwd, err := os.Getwd()
	if err != nil {
		addResult("Workspace", fmt.Errorf("Failed to get CWD: %w", err), "")
	} else {
		testFile := "recac_write_test"
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			addResult("Workspace", fmt.Errorf("No write permission in %s: %w", cwd, err), "")
		} else {
			os.Remove(testFile)
			addResult("Workspace", nil, fmt.Sprintf("Writable: %s", cwd))
		}
	}

	return results
}

// GetDoctor returns a string containing the results of the environment checks.
// It uses RunChecks internally.
func GetDoctor() string {
	results := RunChecks()
	var builder strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	builder.WriteString(titleStyle.Render("RECAC Doctor\n"))
	builder.WriteString(titleStyle.Render("------------\n"))

	for _, res := range results {
		switch res.Status {
		case "passed":
			builder.WriteString(fmt.Sprintf("%s %s: %s\n", successStyle.Render("✔"), res.Name, res.Message))
		case "failed":
			builder.WriteString(fmt.Sprintf("%s %s: %s\n", errorStyle.Render("✖"), res.Name, res.Error))
		case "warning":
			builder.WriteString(fmt.Sprintf("%s %s: %s (%s)\n", warningStyle.Render("!"), res.Name, res.Message, res.Error))
		}
	}
	return builder.String()
}

// GetDoctorJSON returns the results in JSON format.
func GetDoctorJSON() string {
	results := RunChecks()
	b, _ := json.MarshalIndent(results, "", "  ")
	return string(b)
}

func checkDockerConnectivityFuncInternal(cli DockerClient, err error) (string, error) {
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	_, err = cli.Ping(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "Is the docker daemon running?") {
			return "", fmt.Errorf("daemon not running or socket permission error")
		}
		return "", fmt.Errorf("failed to ping daemon: %w", err)
	}

	return "Daemon is responsive", nil
}

// Deprecated: kept for backward compatibility
func checkDockerConnectivityFunc(cli DockerClient, err error) string {
	msg, err := checkDockerConnectivityFuncInternal(cli, err)
	if err != nil {
		return fmt.Sprintf("[✖] Docker: %v\n", err)
	}
	return fmt.Sprintf("[✔] Docker: %s\n", msg)
}
