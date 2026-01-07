package scenarios

import (
	"fmt"
	"os/exec"
)

type HTTPProxyScenario struct{}

func (s *HTTPProxyScenario) Name() string {
	return "http-proxy"
}

func (s *HTTPProxyScenario) Description() string {
	return "A complex scenario requiring the implementation of a Golang HTTP Reverse Proxy with multiple phases."
}

func (s *HTTPProxyScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	return []TicketSpec{
		// Phase 1: Foundation
		{
			ID:      "INIT",
			Summary: fmt.Sprintf("[%s] Initialize Module", uniqueID),
			Desc:    fmt.Sprintf("Initialize go.mod and the basic project structure (cmd/, internal/, pkg/).\n\nRepo: %s", repoURL),
			Type:    "Task",
		},
		{
			ID:       "ERRORS",
			Summary:  fmt.Sprintf("[%s] Define Sentinel Errors", uniqueID),
			Desc:     fmt.Sprintf("Create internal/errors/errors.go and define standard errors for the proxy (ErrInvalidConfig, ErrBackendDown).\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"INIT"},
		},

		// Phase 2: Configuration
		{
			ID:       "CONFIG_STRUCT",
			Summary:  fmt.Sprintf("[%s] Define Config Struct", uniqueID),
			Desc:     fmt.Sprintf("Create internal/config/config.go with the Configuration struct (Port, TargetURL, Timeouts).\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"INIT"},
		},
		{
			ID:       "CONFIG_ENV",
			Summary:  fmt.Sprintf("[%s] Env Config Loader", uniqueID),
			Desc:     fmt.Sprintf("Implement loading configuration from Environment Variables in internal/config.\n Use 'os.Getenv' and strconv.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"CONFIG_STRUCT"},
		},
		{
			ID:       "CONFIG_VALID",
			Summary:  fmt.Sprintf("[%s] Config Validation", uniqueID),
			Desc:     fmt.Sprintf("Implement a Validate() method for the Config struct. Check for valid ports and URLs.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"CONFIG_STRUCT", "ERRORS"},
		},

		// Phase 3: Logging
		{
			ID:       "LOGGER",
			Summary:  fmt.Sprintf("[%s] Setup Logger", uniqueID),
			Desc:     fmt.Sprintf("Initialize a structured logger (slog) in internal/logger. Expose Info/Error helpers.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"INIT"},
		},

		// Phase 4: Middleware Base
		{
			ID:       "MW_BASE",
			Summary:  fmt.Sprintf("[%s] Middleware Type Def", uniqueID),
			Desc:     fmt.Sprintf("Define the Middleware type alias in internal/middleware.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"INIT"},
		},
		{
			ID:       "MW_REQID",
			Summary:  fmt.Sprintf("[%s] Request ID Middleware", uniqueID),
			Desc:     fmt.Sprintf("Implement middleware to inject X-Request-ID header if missing.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"MW_BASE"},
		},
		{
			ID:       "MW_LOG",
			Summary:  fmt.Sprintf("[%s] Logging Middleware", uniqueID),
			Desc:     fmt.Sprintf("Implement middleware to log incoming requests (Method, Path, Duration).\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"MW_BASE", "LOGGER"},
		},
		{
			ID:       "MW_RECOVERY",
			Summary:  fmt.Sprintf("[%s] Recovery Middleware", uniqueID),
			Desc:     fmt.Sprintf("Implement middleware to recover from panics and log the stack trace.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"MW_BASE", "LOGGER"},
		},

		// Phase 5: Proxy Logic
		{
			ID:       "BACKEND_STRUCT",
			Summary:  fmt.Sprintf("[%s] Define Backend Struct", uniqueID),
			Desc:     fmt.Sprintf("Create internal/proxy/backend.go to represent an upstream target (URL, status).\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"INIT"},
		},
		{
			ID:       "DIRECTOR",
			Summary:  fmt.Sprintf("[%s] Proxy Director", uniqueID),
			Desc:     fmt.Sprintf("Implement the Director function that rewrites the HTTP request to the target URL.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"BACKEND_STRUCT", "CONFIG_STRUCT"},
		},
		{
			ID:       "PROXY_HANDLER",
			Summary:  fmt.Sprintf("[%s] Reverse Proxy Handler", uniqueID),
			Desc:     fmt.Sprintf("Implement the main ServeHTTP using httputil.ReverseProxy and the Director.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"DIRECTOR", "MW_BASE"},
		},

		// Phase 6: Health Checks
		{
			ID:       "HEALTH_CHECK",
			Summary:  fmt.Sprintf("[%s] Health Check Logic", uniqueID),
			Desc:     fmt.Sprintf("Implement logic to ping the backend target to verify it is reachable.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"BACKEND_STRUCT"},
		},
		{
			ID:       "HEALTH_HANDLER",
			Summary:  fmt.Sprintf("[%s] Healthz Endpoint", uniqueID),
			Desc:     fmt.Sprintf("Expose /healthz endpoint returning 200 OK for k8s probes.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"MW_BASE"},
		},

		// Phase 7: Assembly
		{
			ID:       "SERVER_SETUP",
			Summary:  fmt.Sprintf("[%s] HTTP Server Config", uniqueID),
			Desc:     fmt.Sprintf("Configure http.Server with ReadTimeout, WriteTimeout from config.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"CONFIG_ENV", "CONFIG_VALID"},
		},
		{
			ID:       "MAIN_WIRING",
			Summary:  fmt.Sprintf("[%s] Main Wiring", uniqueID),
			Desc:     fmt.Sprintf("Implement cmd/proxy/main.go. Wire Config, Logger, Middleware, and start Server.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"SERVER_SETUP", "PROXY_HANDLER", "MW_LOG", "MW_RECOVERY", "HEALTH_HANDLER"},
		},
		{
			ID:       "SHUTDOWN",
			Summary:  fmt.Sprintf("[%s] Graceful Shutdown", uniqueID),
			Desc:     fmt.Sprintf("Handle SIGINT/SIGTERM to gracefully shut down the server in main.go.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"MAIN_WIRING"},
		},
		{
			ID:       "DOCKERFILE",
			Summary:  fmt.Sprintf("[%s] Create Dockerfile", uniqueID),
			Desc:     fmt.Sprintf("Create a multi-stage Dockerfile to build the proxy.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"MAIN_WIRING"},
		},
		{
			ID:       "README",
			Summary:  fmt.Sprintf("[%s] Documentation", uniqueID),
			Desc:     fmt.Sprintf("Write README.md explaining how to configure and run the proxy.\n\nRepo: %s", repoURL),
			Type:     "Task",
			Blockers: []string{"MAIN_WIRING"},
		},
	}
}

func (s *HTTPProxyScenario) Verify(repoPath string, ticketKeys map[string]string) error {
	// For the complex proxy scenario, we check if multiple agent branches exist
	// and if at least one of them has the basic structure.

	// 1. Check if any agent branch exists
	if err := checkAgentBranchExists(repoPath); err != nil {
		return err
	}

	// 2. Try to find the branch for the last ticket (DOCKERFILE or README)
	targetKey := ticketKeys["README"]
	if targetKey == "" {
		targetKey = ticketKeys["DOCKERFILE"]
	}

	branch, err := getSpecificAgentBranch(repoPath, targetKey)
	if err != nil {
		// Fallback to most recent agent branch
		branch, err = getAgentBranch(repoPath)
		if err != nil {
			return err
		}
	}

	// Checkout
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = repoPath
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout %s: %v\nOutput: %s", branch, err, out)
	}

	// 3. Verify Structure
	requiredPaths := []string{
		"go.mod",
		"cmd",
		"internal/config/config.go",
		"internal/proxy",
	}

	for _, p := range requiredPaths {
		checkCmd := exec.Command("ls", "-d", p)
		checkCmd.Dir = repoPath
		if err := checkCmd.Run(); err != nil {
			// List files for debugging
			lsCmd := exec.Command("ls", "-R")
			lsCmd.Dir = repoPath
			lsOut, _ := lsCmd.CombinedOutput()
			return fmt.Errorf("required path %s not found in branch %s\nFiles in repo:\n%s", p, branch, string(lsOut))
		}
	}

	fmt.Printf("Verification Successful: Found basic project structure in branch %s.\n", branch)
	return nil
}

func init() {
	Register(&HTTPProxyScenario{})
}
