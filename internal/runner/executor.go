package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"recac/internal/telemetry"

	"github.com/spf13/viper"
)

// ProcessResponse handles the raw output from the agent (parsing blocks, executing commands).
// Returns the "System Output" to be fed back to the agent in the next turn.
func (s *Session) ProcessResponse(ctx context.Context, response string) (string, error) {
	// Parse Code Blocks: Look for ```bash ... ``` or ```sh ... ```
	// We want to execute them sequentially.
	// If one fails, we stop and return the error output.

	re := regexp.MustCompile("(?s)```(?:bash|sh|cmd)(.*?)```")
	matches := re.FindAllStringSubmatch(response, -1)

	var parsedOutput strings.Builder

	for i, match := range matches {
		cmdScript := strings.TrimSpace(match[1])
		if cmdScript == "" {
			continue
		}

		output, err := s.executeCommandBlock(ctx, cmdScript, i+1, len(matches))
		parsedOutput.WriteString(output)

		if err != nil {
			// Fail Fast: Do not execute subsequent commands if the current one fails
			break
		}
	}

	// Check for Blockers
	if err := s.checkBlockers(ctx); err != nil {
		return "", err
	}

	// Metrics Collection
	metrics := struct {
		Commands      int
		FilesModified int
		OutputLines   int
	}{
		Commands: len(matches),
	}

	// Heuristic for files modified (counting write operations)
	for _, match := range matches {
		script := match[1]
		if strings.Contains(script, " > ") || strings.Contains(script, " >> ") || strings.Contains(script, "touch ") {
			metrics.FilesModified++
		}
	}

	// Calculate output lines from the accumulated buffer
	metrics.OutputLines = strings.Count(parsedOutput.String(), "\n")

	s.Logger.Info("iteration metrics",
		"commands_executed", metrics.Commands,
		"files_modified_est", metrics.FilesModified,
		"output_lines", metrics.OutputLines,
		"response_chars", len(response))

	return parsedOutput.String(), nil
}

// checkBlockers checks for blocker signals in DB or files.
func (s *Session) checkBlockers(ctx context.Context) error {
	// Check for Blocker Signal (DB)
	if s.DBStore != nil {
		blockerMsg, err := s.DBStore.GetSignal(s.Project, "BLOCKER")
		if err == nil && blockerMsg != "" {
			fmt.Printf("\n!!! AGENT BLOCKED: %s !!!\n", blockerMsg)
			fmt.Println("Waiting for blocker to be resolved...")
			return ErrBlocker
		}
	}

	// Check for Blocker File (e.g. blockers.txt)
	// Some agents might write a blocker file if they are stuck
	files := []string{"recac_blockers.txt", "blockers.txt"}
	for _, bf := range files {
		if _, err := os.Stat(filepath.Join(s.Workspace, bf)); err == nil {
			// Check content
			content, _ := os.ReadFile(filepath.Join(s.Workspace, bf))
			blockerContent := strings.TrimSpace(string(content))
			lowerContent := strings.ToLower(blockerContent)

			// Check for blocking content (filtering out false positives)
			// We check if any of the false positive phrases are present. If so, it's NOT a blocker.
			isFalsePositive := strings.Contains(lowerContent, "no blockers") ||
				strings.Contains(lowerContent, "passed") ||
				strings.Contains(lowerContent, "none") ||
				strings.Contains(lowerContent, "initial setup complete") ||
				strings.Contains(lowerContent, "ui verification required")

			isBlocker := blockerContent != "" && !isFalsePositive

			if isBlocker {
				fmt.Printf("\n!!! AGENT REPORTED BLOCKER: %s !!!\n%s\n", bf, blockerContent)
				s.Logger.Warn("agent reported blocker file", "file", bf)
				s.Logger.Warn("blocker content", "content", blockerContent)
				s.Logger.Info("session stopping to allow human resolution")
				return ErrBlocker
			} else if blockerContent != "" {
				// False positive or safe content: delete it so we don't process it again
				os.Remove(filepath.Join(s.Workspace, bf))
			}
		}
	}
	return nil
}

// executeCommandBlock handles the execution of a single command block.
func (s *Session) executeCommandBlock(ctx context.Context, cmdScript string, index, total int) (string, error) {
	s.Logger.Info("executing command block", "index", index, "total", total, "script", cmdScript)

	// Security Scan
	if s.Scanner != nil {
		findings, err := s.Scanner.Scan(cmdScript)
		if err != nil {
			s.Logger.Warn("security scanner error", "error", err, "script", cmdScript)
		}
		if len(findings) > 0 {
			s.Logger.Warn("security violation: blocked dangerous command", "script", cmdScript, "findings", findings)
			return fmt.Sprintf("\n[BLOCKED] Command %d blocked by security scanner: %s\n", index, findings[0].Description), nil
		}
	}

	// Heuristic: If block starts with '{' or '[' and parses as JSON, it's likely data mislabeled as bash.
	if (strings.HasPrefix(cmdScript, "{") || strings.HasPrefix(cmdScript, "[")) && json.Valid([]byte(cmdScript)) {
		s.Logger.Warn("Skipping execution of likely JSON data block mislabeled as bash", "snippet", cmdScript[:min(len(cmdScript), 50)])
		return fmt.Sprintf("\n[Skipped JSON Block %d - Use 'cat' to write files]\n", index), nil
	}

	// Get timeout from config
	timeoutSeconds := viper.GetInt("bash_timeout")
	if timeoutSeconds == 0 {
		timeoutSeconds = 600 // Default 10 minutes
	}

	// Create timeout context for this specific command
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

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

		// Telemetry: Build Failure
		if strings.Contains(cmdScript, "go build") || strings.Contains(cmdScript, "npm run build") || strings.Contains(cmdScript, "make build") {
			telemetry.TrackBuildResult(s.Project, false)
		}

		return result, fmt.Errorf("command execution failed: %w", err)
	}

	// Output Truncation to prevent context exhaustion
	const MaxOutputChars = 20000
	truncatedOutput := output
	if len(output) > MaxOutputChars {
		truncatedOutput = output[:MaxOutputChars] + fmt.Sprintf("\n... [Output Truncated. Total length: %d chars] ...", len(output))
		// Also truncate for display to avoid flooding user console
		s.Logger.Info("command output truncated", "truncated_output", truncatedOutput)
	} else {
		if len(output) > 0 {
			s.Logger.Info("command output", "output", output)
		}
	}

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

	return fmt.Sprintf("Command Output:\n%s\n", truncatedOutput), nil
}

// runCleanerAgent removes temporary files listed in temp_files.txt.
func (s *Session) runCleanerAgent(ctx context.Context) error {
	tempFilesPath := filepath.Join(s.Workspace, "temp_files.txt")
	if _, err := os.Stat(tempFilesPath); os.IsNotExist(err) {
		return nil // Nothing to clean
	}

	content, err := os.ReadFile(tempFilesPath)
	if err != nil {
		return err
	}

	files := strings.Split(string(content), "\n")
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		// Security: Prevent traversal
		if strings.Contains(f, "..") || strings.HasPrefix(f, "/") {
			s.Logger.Warn("skipping unsafe cleanup path", "path", f)
			continue
		}

		path := filepath.Join(s.Workspace, f)
		if err := os.Remove(path); err != nil {
			s.Logger.Warn("failed to remove temp file", "path", path, "error", err)
		} else {
			s.Logger.Info("removed temp file", "path", path)
		}
	}

	// Remove the list file itself
	if err := os.Remove(tempFilesPath); err != nil {
		s.Logger.Warn("failed to remove temp_files.txt", "error", err)
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
