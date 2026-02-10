package doctor

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Global variables for mocking
var (
	execCommand = exec.Command
	osStat      = os.Stat
	httpGet     = http.Get
)

// SystemCheck checks OS and Go environment.
type SystemCheck struct {
	BaseCheck
}

func NewSystemCheck() *SystemCheck {
	return &SystemCheck{BaseCheck: BaseCheck{CheckName: "System"}}
}

func (c *SystemCheck) Run(ctx context.Context) CheckResult {
	// Check Go version
	out, err := execCommand("go", "version").Output()
	if err != nil {
		return c.Fail(fmt.Errorf("go binary not found or error: %w", err))
	}
	goVer := strings.TrimSpace(string(out))

	info := fmt.Sprintf("OS: %s/%s, Go: %s", runtime.GOOS, runtime.GOARCH, goVer)
	return c.Success(info)
}

// ConfigCheck validates the configuration file.
type ConfigCheck struct {
	BaseCheck
}

func NewConfigCheck() *ConfigCheck {
	return &ConfigCheck{BaseCheck: BaseCheck{CheckName: "Config"}}
}

func (c *ConfigCheck) Run(ctx context.Context) CheckResult {
	file := viper.ConfigFileUsed()
	if file == "" {
		return c.Fail(fmt.Errorf("no config file found"))
	}
	if _, err := osStat(file); os.IsNotExist(err) {
		return c.Fail(fmt.Errorf("config file %s does not exist", file))
	}

	// Validate basic fields
	provider := viper.GetString("provider")
	if provider == "" {
		return c.Fail(fmt.Errorf("provider not set in config"))
	}

	return c.Success(fmt.Sprintf("Config: %s, Provider: %s", file, provider))
}

// DependencyCheck verifies critical tools are installed.
type DependencyCheck struct {
	BaseCheck
	Tool string
}

func NewDependencyCheck(tool string) *DependencyCheck {
	return &DependencyCheck{BaseCheck: BaseCheck{CheckName: fmt.Sprintf("Dependency: %s", tool)}, Tool: tool}
}

func (c *DependencyCheck) Run(ctx context.Context) CheckResult {
	path, err := exec.LookPath(c.Tool)
	if err != nil {
		return c.Fail(fmt.Errorf("%s not found in PATH", c.Tool))
	}
	return c.Success(fmt.Sprintf("Found at %s", path))
}

// DockerCheck verifies Docker daemon status.
type DockerCheck struct {
	BaseCheck
}

func NewDockerCheck() *DockerCheck {
	return &DockerCheck{BaseCheck: BaseCheck{CheckName: "Docker"}}
}

func (c *DockerCheck) Run(ctx context.Context) CheckResult {
	// Simple check: docker info
	cmd := execCommand("docker", "info")
	if err := cmd.Run(); err != nil {
		return c.Fail(fmt.Errorf("daemon not running or permission denied: %w", err))
	}
	return c.Success("Daemon is running")
}

// NetworkCheck verifies internet connectivity.
type NetworkCheck struct {
	BaseCheck
	URL string
}

func NewNetworkCheck(url string) *NetworkCheck {
	if url == "" {
		url = "https://www.google.com"
	}
	return &NetworkCheck{BaseCheck: BaseCheck{CheckName: "Network"}, URL: url}
}

func (c *NetworkCheck) Run(ctx context.Context) CheckResult {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(c.URL)
	if err != nil {
		return c.Fail(fmt.Errorf("failed to reach %s: %w", c.URL, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return c.Fail(fmt.Errorf("received status %d from %s", resp.StatusCode, c.URL))
	}
	return c.Success(fmt.Sprintf("Connected to %s", c.URL))
}

// AuthCheck verifies API keys (basic format check for now).
type AuthCheck struct {
	BaseCheck
}

func NewAuthCheck() *AuthCheck {
	return &AuthCheck{BaseCheck: BaseCheck{CheckName: "Auth"}}
}

func (c *AuthCheck) Run(ctx context.Context) CheckResult {
	provider := viper.GetString("provider")
	// If provider is mock or local, skip
	if provider == "mock" || provider == "ollama" {
		return c.Skip(fmt.Sprintf("Provider '%s' does not require external auth", provider))
	}

	key := viper.GetString("api_key")
	if key == "" {
		// Fallback checks
		if provider == "openai" {
			key = os.Getenv("OPENAI_API_KEY")
		} else if provider == "anthropic" {
			key = os.Getenv("ANTHROPIC_API_KEY")
		} else if provider == "gemini" {
			key = os.Getenv("GEMINI_API_KEY")
		}
	}

	if key == "" {
		return c.Fail(fmt.Errorf("API key not found for provider %s", provider))
	}

	// Basic format validation
	if len(key) < 5 {
		return c.Fail(fmt.Errorf("API key seems too short"))
	}

	return c.Success(fmt.Sprintf("API Key present for %s (len: %d)", provider, len(key)))
}
