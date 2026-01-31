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
	execCommand             = exec.Command
	clientNewClientWithOpts = client.NewClientWithOpts
	viperConfigFileUsed     = viper.ConfigFileUsed
	checkDockerConnectivity = checkDockerConnectivityFunc
)

// DockerClient defines the interface for Docker client operations needed by the doctor.
type DockerClient interface {
	Ping(ctx context.Context) (types.Ping, error)
}

// Diagnostic represents the result of a single health check.
type Diagnostic struct {
	Name       string
	Status     string // "PASS", "FAIL", "WARN"
	Message    string
	CanAutoFix bool
	FixID      string
}

// Diagnose runs all health checks and returns a list of diagnostics.
func Diagnose() []Diagnostic {
	var diagnostics []Diagnostic

	// Check 1: Configuration
	diagnostics = append(diagnostics, checkConfigDiagnostic())

	// Check 2: Dependencies
	diagnostics = append(diagnostics, checkDependenciesDiagnostic()...)

	// Check 3: Docker Connectivity
	diagnostics = append(diagnostics, checkDockerDiagnostic())

	// Check 4: Git Identity
	diagnostics = append(diagnostics, checkGitIdentityDiagnostic())

	return diagnostics
}

// GetDoctor returns a string containing the results of the environment checks.
// It is maintained for backward compatibility.
func GetDoctor() string {
	results := Diagnose()
	var builder strings.Builder

	builder.WriteString("RECAC Doctor\n")
	builder.WriteString("------------\n")

	for _, d := range results {
		symbol := "[?]"
		if d.Status == "PASS" {
			symbol = "[✔]"
		} else if d.Status == "FAIL" {
			symbol = "[✖]"
		} else if d.Status == "WARN" {
			symbol = "[!]"
		}

		builder.WriteString(fmt.Sprintf("%s %s: %s\n", symbol, d.Name, d.Message))
	}

	return builder.String()
}

func checkConfigDiagnostic() Diagnostic {
	if cfgFile := viperConfigFileUsed(); cfgFile != "" {
		return Diagnostic{
			Name:    "Configuration",
			Status:  "PASS",
			Message: fmt.Sprintf("%s found", cfgFile),
		}
	}
	return Diagnostic{
		Name:       "Configuration",
		Status:     "FAIL",
		Message:    "Missing config file",
		CanAutoFix: true,
		FixID:      "fix_config",
	}
}

func checkDependenciesDiagnostic() []Diagnostic {
	var results []Diagnostic
	dependencies := []string{"git", "docker"}
	for _, dep := range dependencies {
		_, err := execLookPath(dep)
		if err != nil {
			results = append(results, Diagnostic{
				Name:    "Dependency",
				Status:  "FAIL",
				Message: fmt.Sprintf("%s not found in PATH", dep),
			})
		} else {
			results = append(results, Diagnostic{
				Name:    "Dependency",
				Status:  "PASS",
				Message: fmt.Sprintf("%s found in PATH", dep),
			})
		}
	}
	return results
}

func checkDockerDiagnostic() Diagnostic {
	dockerCli, err := clientNewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	msg := checkDockerConnectivity(dockerCli, err)

	// Rely on the output of checkDockerConnectivity (which handles Ping and is mocked in tests)
	status := "PASS"
	cleanMsg := strings.TrimSpace(msg)

	if strings.Contains(msg, "[✖]") {
		status = "FAIL"
	}

	// Remove legacy prefixes if present for clean message
	if strings.HasPrefix(cleanMsg, "[✔]") || strings.HasPrefix(cleanMsg, "[✖]") {
		parts := strings.SplitN(cleanMsg, ": ", 2)
		if len(parts) > 1 {
			cleanMsg = parts[1]
		}
	}

	return Diagnostic{
		Name:    "Docker",
		Status:  status,
		Message: cleanMsg,
	}
}

func checkGitIdentityDiagnostic() Diagnostic {
	// Check user.name
	nameCmd := execCommand("git", "config", "user.name")
	if err := nameCmd.Run(); err != nil {
		return Diagnostic{
			Name:       "Git Identity",
			Status:     "FAIL",
			Message:    "git user.name not configured",
			CanAutoFix: true,
			FixID:      "fix_git_identity",
		}
	}

	// Check user.email
	emailCmd := execCommand("git", "config", "user.email")
	if err := emailCmd.Run(); err != nil {
		return Diagnostic{
			Name:       "Git Identity",
			Status:     "FAIL",
			Message:    "git user.email not configured",
			CanAutoFix: true,
			FixID:      "fix_git_identity",
		}
	}

	return Diagnostic{
		Name:    "Git Identity",
		Status:  "PASS",
		Message: "User identity configured",
	}
}

func checkConfig() string {
	d := checkConfigDiagnostic()
	symbol := "[✔]"
	if d.Status == "FAIL" {
		symbol = "[✖]"
	}
	return fmt.Sprintf("%s %s: %s\n", symbol, d.Name, d.Message)
}

func checkDependencies() string {
	var builder strings.Builder
	diags := checkDependenciesDiagnostic()
	for _, d := range diags {
		symbol := "[✔]"
		if d.Status == "FAIL" {
			symbol = "[✖]"
		}
		builder.WriteString(fmt.Sprintf("%s %s: %s\n", symbol, d.Name, d.Message))
	}
	return builder.String()
}

// Kept for backward compatibility if needed by tests mocking this specific function signature
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
