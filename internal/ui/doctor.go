package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/viper"

	"recac/internal/agent"
	"recac/internal/jira"
)

// Function variables for mocking
var (
	execLookPath            = exec.LookPath
	execCommand             = exec.Command
	clientNewClientWithOpts = client.NewClientWithOpts
	viperConfigFileUsed     = viper.ConfigFileUsed
	askOneFunc              = survey.AskOne

	// Check functions exposed as variables for mocking
	checkConfigurationFunc = checkConfigurationImpl
	checkDependenciesFunc  = checkDependenciesImpl
	checkDockerFunc        = checkDockerImpl
	checkGitIdentityFunc   = checkGitIdentityImpl
	checkJiraFunc          = checkJiraImpl
	checkAgentFunc         = checkAgentImpl
)

// DockerClient defines the interface for Docker client operations needed by the doctor.
type DockerClient interface {
	Ping(ctx context.Context) (types.Ping, error)
}

// Diagnostic represents a single system check result.
type Diagnostic struct {
	Component string
	Status    string // "OK", "FAIL", "WARN"
	Message   string
	Fixable   bool
	Fix       func() error
}

// ConfigFixer is a function injected by the main package to run the setup wizard.
var ConfigFixer func() error

// Diagnose runs all system checks and returns a list of diagnostics.
func Diagnose(ctx context.Context) []Diagnostic {
	var results []Diagnostic

	// 1. Configuration
	results = append(results, checkConfigurationFunc())

	// 2. Dependencies
	results = append(results, checkDependenciesFunc()...)

	// 3. Docker Connectivity
	results = append(results, checkDockerFunc())

	// 4. Git Identity
	results = append(results, checkGitIdentityFunc())

	// 5. Jira Connectivity (if configured)
	if viper.GetString("jira_url") != "" {
		results = append(results, checkJiraFunc(ctx))
	}

	// 6. AI Agent Connectivity (if configured)
	if viper.GetString("agent_provider") != "" {
		results = append(results, checkAgentFunc(ctx))
	}

	return results
}

// GetDoctor returns a string containing the results of the environment checks.
// Maintained for backward compatibility.
func GetDoctor() string {
	results := Diagnose(context.Background())
	var builder strings.Builder

	builder.WriteString("RECAC Doctor\n")
	builder.WriteString("------------\n")

	for _, res := range results {
		symbol := "[✖]"
		if res.Status == "OK" {
			symbol = "[✔]"
		} else if res.Status == "WARN" {
			symbol = "[!]"
		}
		builder.WriteString(fmt.Sprintf("%s %s: %s\n", symbol, res.Component, res.Message))
	}

	return builder.String()
}

func checkConfigurationImpl() Diagnostic {
	cfgFile := viperConfigFileUsed()
	if cfgFile != "" {
		return Diagnostic{
			Component: "Configuration",
			Status:    "OK",
			Message:   fmt.Sprintf("%s found", cfgFile),
		}
	}
	return Diagnostic{
		Component: "Configuration",
		Status:    "FAIL",
		Message:   "Missing config file",
		Fixable:   true,
		Fix: func() error {
			if ConfigFixer != nil {
				return ConfigFixer()
			}
			return fmt.Errorf("setup wizard not available")
		},
	}
}

func checkDependenciesImpl() []Diagnostic {
	var results []Diagnostic
	dependencies := []string{"git", "docker"}
	for _, dep := range dependencies {
		_, err := execLookPath(dep)
		if err != nil {
			results = append(results, Diagnostic{
				Component: "Dependency",
				Status:    "FAIL",
				Message:   fmt.Sprintf("%s not found in PATH", dep),
				Fixable:   false,
			})
		} else {
			results = append(results, Diagnostic{
				Component: "Dependency",
				Status:    "OK",
				Message:   fmt.Sprintf("%s found in PATH", dep),
			})
		}
	}
	return results
}

func checkDockerImpl() Diagnostic {
	cli, err := clientNewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return Diagnostic{
			Component: "Docker",
			Status:    "FAIL",
			Message:   fmt.Sprintf("Failed to create client: %v", err),
			Fixable:   false,
		}
	}

	_, err = cli.Ping(context.Background())
	if err != nil {
		msg := fmt.Sprintf("Failed to ping daemon: %v", err)
		if strings.Contains(err.Error(), "Is the docker daemon running?") {
			msg = "Daemon not running or socket permission error"
		}
		return Diagnostic{
			Component: "Docker",
			Status:    "FAIL",
			Message:   msg,
			Fixable:   false,
		}
	}

	return Diagnostic{
		Component: "Docker",
		Status:    "OK",
		Message:   "Daemon is responsive",
	}
}

func checkGitIdentityImpl() Diagnostic {
	// Check user.name
	cmdName := execCommand("git", "config", "user.name")
	nameOut, errName := cmdName.Output()

	// Check user.email
	cmdEmail := execCommand("git", "config", "user.email")
	emailOut, errEmail := cmdEmail.Output()

	hasName := errName == nil && strings.TrimSpace(string(nameOut)) != ""
	hasEmail := errEmail == nil && strings.TrimSpace(string(emailOut)) != ""

	if hasName && hasEmail {
		return Diagnostic{
			Component: "Git Identity",
			Status:    "OK",
			Message:   "User name and email are set",
		}
	}

	return Diagnostic{
		Component: "Git Identity",
		Status:    "FAIL",
		Message:   "Git user.name or user.email is missing",
		Fixable:   true,
		Fix:       fixGitIdentityAction,
	}
}

func fixGitIdentityAction() error {
	var name, email string

	err := askOneFunc(&survey.Input{
		Message: "Enter Git User Name:",
	}, &name)
	if err != nil {
		return err
	}

	err = askOneFunc(&survey.Input{
		Message: "Enter Git User Email:",
	}, &email)
	if err != nil {
		return err
	}

	if name != "" {
		if err := execCommand("git", "config", "--global", "user.name", name).Run(); err != nil {
			return fmt.Errorf("failed to set git user.name: %w", err)
		}
	}
	if email != "" {
		if err := execCommand("git", "config", "--global", "user.email", email).Run(); err != nil {
			return fmt.Errorf("failed to set git user.email: %w", err)
		}
	}
	return nil
}

func checkJiraImpl(ctx context.Context) Diagnostic {
	url := viper.GetString("jira_url")
	user := viper.GetString("jira_email") // Assuming jira_email is username/email
	token := viper.GetString("jira_token")

	if user == "" || token == "" {
		return Diagnostic{
			Component: "Jira",
			Status:    "WARN",
			Message:   "Jira URL configured but missing credentials",
			Fixable:   true,
			Fix: func() error {
				if ConfigFixer != nil {
					return ConfigFixer()
				}
				return nil
			},
		}
	}

	c := jira.NewClient(url, user, token)
	err := c.Authenticate(ctx)
	if err != nil {
		return Diagnostic{
			Component: "Jira",
			Status:    "FAIL",
			Message:   fmt.Sprintf("Authentication failed: %v", err),
			Fixable:   true,
			Fix: func() error {
				if ConfigFixer != nil {
					return ConfigFixer()
				}
				return nil
			},
		}
	}

	return Diagnostic{
		Component: "Jira",
		Status:    "OK",
		Message:   "Authenticated successfully",
	}
}

func checkAgentImpl(ctx context.Context) Diagnostic {
	provider := viper.GetString("agent_provider")
	model := viper.GetString("agent_model")
	apiKey := viper.GetString("api_key")

	// Some providers might handle key differently, but NewAgent handles it
	// We use a dummy project and workdir
	a, err := agent.NewAgent(provider, apiKey, model, ".", "recac-doctor")
	if err != nil {
		return Diagnostic{
			Component: "AI Agent",
			Status:    "FAIL",
			Message:   fmt.Sprintf("Failed to initialize agent: %v", err),
			Fixable:   true,
			Fix: func() error {
				if ConfigFixer != nil {
					return ConfigFixer()
				}
				return nil
			},
		}
	}

	// Ping with a simple prompt
	// Use a short timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = a.Send(ctx, "Hello, are you there? Reply with 'yes'.")
	if err != nil {
		return Diagnostic{
			Component: "AI Agent",
			Status:    "FAIL",
			Message:   fmt.Sprintf("Connection failed: %v", err),
			Fixable:   true,
			Fix: func() error {
				if ConfigFixer != nil {
					return ConfigFixer()
				}
				return nil
			},
		}
	}

	return Diagnostic{
		Component: "AI Agent",
		Status:    "OK",
		Message:   "Connected successfully",
	}
}
