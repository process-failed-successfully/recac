package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"recac/internal/telemetry"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ProcessResponse parses the agent response for commands, executes them, and handles blockers.
func (s *Session) ProcessResponse(ctx context.Context, response string) (string, error) {
	// 1. Extract Bash Blocks (More robust regex to handle variations in LLM output)
	matches := bashBlockRegex.FindAllStringSubmatch(response, -1)

	// Safety valve: Prevent LLM loops from flooding the execution
	const maxCommandBlocks = 100
	if len(matches) > maxCommandBlocks {
		s.Logger.Warn("Safety valve tripped: truncated too many command blocks", "total", len(matches), "limit", maxCommandBlocks)
		matches = matches[:maxCommandBlocks]
	}

	var parsedOutput strings.Builder
	// Get timeout from config
	timeoutSeconds := viper.GetInt("bash_timeout")
	if timeoutSeconds == 0 {
		timeoutSeconds = 600 // Default 10 minutes
	}

	for i, match := range matches {
		cmdScript := strings.TrimSpace(match[1])
		if cmdScript == "" {
			continue
		}
		s.Logger.Info("executing command block", "index", i+1, "total", len(matches), "script", cmdScript)

		// Heuristic: If block starts with '{' or '[' and parses as JSON, it's likely data mislabeled as bash.
		if (strings.HasPrefix(cmdScript, "{") || strings.HasPrefix(cmdScript, "[")) && json.Valid([]byte(cmdScript)) {
			s.Logger.Warn("Skipping execution of likely JSON data block mislabeled as bash", "snippet", cmdScript[:min(len(cmdScript), 50)])
			parsedOutput.WriteString(fmt.Sprintf("\n[Skipped JSON Block %d - Use 'cat' to write files]\n", i+1))
			continue
		}

		// Create timeout context for this specific command
		cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)

		// Execute via Docker or Local
		var output string
		var err error

		if s.UseLocalAgent {
			// Execute Locally
			cmd := exec.CommandContext(cmdCtx, "/bin/bash", "-c", cmdScript)
			// Propagate Environment + Inject Project ID
			cmd.Env = append(os.Environ(), fmt.Sprintf("RECAC_PROJECT_ID=%s", s.Project))
			// Debug: Log key env vars for troubleshooting
			s.Logger.Info("[DEBUG] Local exec env vars",
				"RECAC_PROJECT_ID", s.Project,
				"RECAC_DB_TYPE", os.Getenv("RECAC_DB_TYPE"),
				"RECAC_DB_URL_set", os.Getenv("RECAC_DB_URL") != "")
			cmd.Dir = s.Workspace // Run in workspace
			// Capture Combined Output
			var outBuf bytes.Buffer
			cmd.Stdout = &outBuf
			cmd.Stderr = &outBuf
			err = cmd.Run()
			output = outBuf.String()
		} else {
			// Execute via Docker
			output, err = s.Docker.Exec(cmdCtx, s.GetContainerID(), []string{"/bin/bash", "-c", cmdScript})
		}

		cancel() // Ensure we release resources

		if err != nil {
			var errMsg string
			if cmdCtx.Err() == context.DeadlineExceeded {
				errMsg = fmt.Sprintf("Command timed out after %d seconds.", timeoutSeconds)
			} else if errors.Is(err, context.DeadlineExceeded) {
				errMsg = fmt.Sprintf("Command timed out after %d seconds.", timeoutSeconds)
			} else {
				errMsg = err.Error()
			}

			result := fmt.Sprintf("Command Failed: %s\nError: %s\nOutput:\n%s\n", cmdScript, errMsg, output)
			s.Logger.Error("command failed", "script", cmdScript, "error", errMsg)
			parsedOutput.WriteString(result)

			// Telemetry: Build Failure
			if strings.Contains(cmdScript, "go build") || strings.Contains(cmdScript, "npm run build") || strings.Contains(cmdScript, "make build") {
				telemetry.TrackBuildResult(s.Project, false)
			}
		} else {
			// Output Truncation to prevent context exhaustion
			const MaxOutputChars = 2000
			truncatedOutput := output
			if len(output) > MaxOutputChars {
				truncatedOutput = output[:MaxOutputChars] + fmt.Sprintf("\n... [Output Truncated. Total length: %d chars] ...", len(output))
				// Also truncate for display to avoid flooding user console
				s.Logger.Info("command output truncated", "truncated_output", truncatedOutput)
			} else {
				// result := fmt.Sprintf("Command Output:\n%s\n", output)
				if len(output) > 0 {
					s.Logger.Info("command output", "output", output)
				}
			}

			// Append valid (possibly truncated) output to the result buffer
			parsedOutput.WriteString(fmt.Sprintf("Command Output:\n%s\n", truncatedOutput))

			// Telemetry: Lines Generated (Approximate based on cat/echo)
			lines := strings.Count(cmdScript, "\n")
			telemetry.TrackLineGenerated(s.Project, lines)

			// Telemetry: Build Success
			if strings.Contains(cmdScript, "go build") || strings.Contains(cmdScript, "npm run build") || strings.Contains(cmdScript, "make build") {
				telemetry.TrackBuildResult(s.Project, true)
			}

			// Telemetry: Files Created/Modified
			if strings.Contains(cmdScript, "touch ") || strings.Contains(cmdScript, "> ") {
				telemetry.TrackFileCreated(s.Project)
			}
		}
	}

	// Check for Blocker Signal (DB)
	if s.DBStore != nil {
		blockerMsg, err := s.DBStore.GetSignal(s.Project, "BLOCKER")
		if err == nil && blockerMsg != "" {
			fmt.Printf("\n!!! AGENT BLOCKED: %s !!!\n", blockerMsg)
			fmt.Println("Waiting for blocker to be resolved...")
			return "", ErrBlocker
		}
	}

	// Legacy File Check (Deprecating, but keeping for compatibility)
	if s.Docker != nil {
		blockerFiles := []string{"recac_blockers.txt", "blockers.txt"}
		for _, bf := range blockerFiles {
			checkCmd := []string{"/bin/sh", "-c", fmt.Sprintf("test -f %s && cat %s", bf, bf)}
			blockerContent, err := s.Docker.Exec(ctx, s.GetContainerID(), checkCmd)
			trimmed := strings.TrimSpace(blockerContent)
			if err == nil && len(trimmed) > 0 {
				// Check for false positives (status messages instead of blockers)
				// 1. Normalize: lowercase and remove common comment/bullet chars (#, *, -, whitespace)
				cleanStr := strings.ToLower(trimmed)
				cleanStr = strings.ReplaceAll(cleanStr, "#", "")
				cleanStr = strings.ReplaceAll(cleanStr, "*", "")
				cleanStr = strings.ReplaceAll(cleanStr, "-", "")
				cleanStr = strings.Join(strings.Fields(cleanStr), " ") // Normalize internal whitespace

				isFalsePositive := strings.Contains(cleanStr, "no blockers") ||
					strings.HasPrefix(cleanStr, "none") ||
					strings.Contains(cleanStr, "no technical obstacles") ||
					strings.Contains(cleanStr, "progressing smoothly") ||
					strings.Contains(cleanStr, "initial setup complete") ||
					strings.Contains(cleanStr, "all requirements met") ||
					strings.Contains(cleanStr, "ready for next feature") ||
					strings.Contains(cleanStr, "ui verification required")

				if isFalsePositive {
					s.Logger.Info("ignoring false positive blocker", "file", bf, "content", trimmed)
					// Cleanup the file so it doesn't re-trigger
					s.Docker.Exec(ctx, s.GetContainerID(), []string{"rm", bf})
					continue
				}

				// Real Blocker found!
				s.Logger.Warn("agent reported blocker file", "file", bf)
				s.Logger.Warn("blocker content", "content", blockerContent)
				s.Logger.Info("session stopping to allow human resolution")
				return "", ErrBlocker
			}
		}
	}

	return parsedOutput.String(), nil
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

// bootstrapGit sets up default git configuration in the container.
func (s *Session) bootstrapGit(ctx context.Context) error {
	containerID := s.GetContainerID()
	if containerID == "" {
		return fmt.Errorf("container not started")
	}

	email := viper.GetString("git_user_email")
	name := viper.GetString("git_user_name")

	if email == "" {
		email = "recac-agent@example.com"
	}
	if name == "" {
		name = "RECAC Agent"
	}

	// 1. Create Ignore content
	ignoreContent := `
# RECAC Agent Artifacts
.recac.db
.agent_state.json
.agent_state_*.json
.qa_result
manager_directives.txt
successes.txt
temp_files.txt
blockers.txt
questions.txt
app_spec.txt
feature_list.json
implementation_summary.txt
persistence_test.txt
*summary.txt
*.qa_result

# Agent Caches and Configs
.cache/
.config/
go/
node_modules/
dist/
build/
.npm/

# Logs
*.log
npm-debug.log*
yarn-debug.log*
yarn-error.log*
.pnpm-debug.log*
`
	// 1a. Write to workspace .gitignore (affects BOTH host and container git)
	gitignorePath := filepath.Join(s.Workspace, ".gitignore")
	var existingIgnore []byte
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existingIgnore = data
	}

	if !strings.Contains(string(existingIgnore), ".recac.db") {
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString("\n# Added by RECAC\n" + ignoreContent)
			f.Close()
			fmt.Println("Updated workspace .gitignore with agent patterns.")
		}
	}

	// 1b. Create Global Ignore File in container (redundancy/system-wide)
	if !s.UseLocalAgent {
		writeCmd := []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' > /etc/gitignore_global", ignoreContent)}
		if _, err := s.Docker.ExecAsUser(ctx, containerID, "root", writeCmd); err != nil {
			fmt.Printf("Warning: Failed to create global gitignore: %v\n", err)
		}

		fmt.Printf("Bootstrapping git config (email: %s, name: %s, excludes: /etc/gitignore_global)...\n", email, name)

		commands := [][]string{
			{"sudo", "git", "config", "--system", "user.email", email},
			{"sudo", "git", "config", "--system", "user.name", name},
			{"sudo", "git", "config", "--system", "safe.directory", "*"},
			{"sudo", "git", "config", "--system", "core.excludesFile", "/etc/gitignore_global"},
		}

		for _, cmd := range commands {
			// Use root to bootstrap system config robustly
			if _, err := s.Docker.ExecAsUser(ctx, containerID, "root", cmd); err != nil {
				return fmt.Errorf("failed to execute git bootstrap command %v: %w", cmd, err)
			}
		}
	} else {
		fmt.Println("Skipping system-level git bootstrap in local mode (relying on env vars).")
	}

	return nil
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

// fixPermissions ensures that all files in the workspace are owned by the host user.
// This prevents "Permission denied" errors when the host process (git) tries to modify/delete files created by the agent (root).
func (s *Session) fixPermissions(ctx context.Context) error {
	containerID := s.GetContainerID()
	if containerID == "" || containerID == "local" || s.Docker == nil {
		return nil
	}

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// We use the root user inside the container to chown everything to the host user
	// /workspace in container maps to s.Workspace on host
	// We use chown -R to be thorough
	cmd := []string{"chown", "-R", fmt.Sprintf("%s:%s", u.Uid, u.Gid), "/workspace"}

	// We suppress output unless error, to avoid spam
	output, err := s.Docker.ExecAsUser(ctx, containerID, "root", cmd)
	if err != nil {
		return fmt.Errorf("chown failed: %s, output: %s", err, output)
	}
	return nil
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
