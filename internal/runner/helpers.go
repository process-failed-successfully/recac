package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FilterEnv filters environment variables to prevent leaking sensitive secrets
// to child processes executed by the agent.
// It uses a denylist approach to block known sensitive keys while allowing
// standard system and tool variables to pass through.
func FilterEnv(env []string) []string {
	var safeEnv []string
	sensitivePatterns := []string{
		"API_KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL",
		"OPENAI_", "ANTHROPIC_", "GEMINI_", "JIRA_", "SLACK_",
		"AWS_", "GCP_", "AZURE_", "GH_", "GITHUB_", "GITLAB_",
	}

	for _, e := range env {
		// Split KEY=VALUE to check KEY only
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 0 {
			continue
		}
		key := strings.ToUpper(parts[0])

		isSensitive := false
		for _, p := range sensitivePatterns {
			if strings.Contains(key, p) {
				isSensitive = true
				break
			}
		}

		// Exception: RECAC_PROJECT_ID is safe and needed for context
		if key == "RECAC_PROJECT_ID" {
			isSensitive = false
		}

		// Exception: GIT_SSH or GIT_ASKPASS might be needed, but if they contain "KEY" or "TOKEN",
		// they are blocked by the generic check.
		// Usually GIT_SSH_COMMAND shouldn't contain the key itself, but the command.
		// However, some people put keys in env vars referenced by commands.
		// We err on the side of safety.

		if !isSensitive {
			safeEnv = append(safeEnv, e)
		}
	}
	return safeEnv
}

// fixPasswdDatabase ensures the host user exists in the container's /etc/passwd.
// This prevents "you do not exist in the passwd database" errors when using sudo.
func (s *Session) fixPasswdDatabase(ctx context.Context, containerUser string) {
	parts := strings.Split(containerUser, ":")
	if len(parts) < 1 {
		return
	}
	uid := parts[0]
	gid := uid // Default gid to uid if not specified
	if len(parts) > 1 {
		gid = parts[1]
	}

	fmt.Printf("Fixing container passwd database for UID:GID %s:%s...\n", uid, gid)

	// 1. Ensure GID exists
	groupCheckCmd := []string{"getent", "group", gid}
	groupOut, groupErr := s.Docker.ExecAsUser(ctx, s.GetContainerID(), "root", groupCheckCmd)
	if groupErr != nil || strings.TrimSpace(groupOut) == "" {
		// Try groupadd first
		groupAddCmd := []string{"groupadd", "-g", gid, "appgroup"}
		if _, err := s.Docker.ExecAsUser(ctx, s.GetContainerID(), "root", groupAddCmd); err != nil {
			// Fallback to Alpine addgroup
			groupAddCmd = []string{"addgroup", "-g", gid, "appgroup"}
			if _, err := s.Docker.ExecAsUser(ctx, s.GetContainerID(), "root", groupAddCmd); err != nil {
				fmt.Printf("Warning: Failed to create group %s: %v\n", gid, err)
			}
		}
	}

	// 2. Ensure UID exists
	userCheckCmd := []string{"getent", "passwd", uid}
	userOut, userErr := s.Docker.ExecAsUser(ctx, s.GetContainerID(), "root", userCheckCmd)
	if userErr != nil || strings.TrimSpace(userOut) == "" {
		// Try useradd first
		userAddCmd := []string{"useradd", "-u", uid, "-g", gid, "-m", "-s", "/bin/sh", "-d", "/workspace", "appuser"}
		if _, err := s.Docker.ExecAsUser(ctx, s.GetContainerID(), "root", userAddCmd); err != nil {
			// Fallback to Alpine adduser
			// adduser -u UID -G appgroup -h /workspace -s /bin/sh -D appuser
			userAddCmd = []string{"adduser", "-u", uid, "-G", "appgroup", "-h", "/workspace", "-s", "/bin/sh", "-D", "appuser"}
			if _, err := s.Docker.ExecAsUser(ctx, s.GetContainerID(), "root", userAddCmd); err != nil {
				fmt.Printf("Warning: Failed to create user %s: %v\n", uid, err)
			}
		}
	}
}

// findAgentBridgeBinary hunts for the agent-bridge binary on the host
func (s *Session) findAgentBridgeBinary() (string, error) {
	// 0. Try Standard Location (Container / System Install)
	if _, err := os.Stat("/usr/local/bin/agent-bridge"); err == nil {
		return "/usr/local/bin/agent-bridge", nil
	}

	// 1. Try CWD
	srcPath, err := filepath.Abs("agent-bridge")
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		// 2. Try Project Root (assuming we are in internal/runner or a sub-test dir)
		dir, _ := os.Getwd()
		for i := 0; i < 5; i++ { // Guard against infinite loop
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				// Found root
				srcPath = filepath.Join(dir, "agent-bridge")
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Verify existence
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return "", fmt.Errorf("agent-bridge binary not found at %s. Did you run 'make bridge'?", srcPath)
	}
	return srcPath, nil
}

// runInitScript checks for init.sh in the workspace and executes it if present.
// Failures are logged as warnings but do not stop the session.
func (s *Session) runInitScript(ctx context.Context) {
	initPath := filepath.Join(s.Workspace, "init.sh")
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		return
	}

	fmt.Println("Found init.sh. Executing in container...")

	// 1. Ensure executable
	if s.UseLocalAgent {
		if err := os.Chmod(initPath, 0755); err != nil {
			fmt.Printf("Warning: Failed to make init.sh executable: %v\n", err)
			return
		}
	} else {
		if _, err := s.Docker.ExecAsUser(ctx, s.GetContainerID(), "root", []string{"chmod", "+x", "init.sh"}); err != nil {
			fmt.Printf("Warning: Failed to make init.sh executable: %v\n", err)
			return
		}
	}

	// 2. Execute Async
	fmt.Println("Found init.sh. Launching in background (10m timeout)...")
	go func() {
		asyncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		var output string
		var err error

		if s.UseLocalAgent {
			// Local Execution
			cmd := exec.CommandContext(asyncCtx, "/bin/sh", "-c", "./init.sh")
			cmd.Dir = s.Workspace
			var outBuf bytes.Buffer
			cmd.Stdout = &outBuf
			cmd.Stderr = &outBuf
			err = cmd.Run()
			output = outBuf.String()
		} else {
			// Docker Execution
			output, err = s.Docker.ExecAsUser(asyncCtx, s.GetContainerID(), "root", []string{"/bin/sh", "-c", "./init.sh"})
		}

		if err != nil {
			if asyncCtx.Err() == context.DeadlineExceeded {
				fmt.Printf("Warning: init.sh execution timed out after 10 minutes.\n")
			} else {
				fmt.Printf("Warning: init.sh execution failed: %v\n", err)
			}
		} else if len(output) > 0 {
			fmt.Printf("async init.sh finished. Output:\n%s\n", output)
		} else {
			fmt.Println("async init.sh finished successfully.")
		}
	}()
}
