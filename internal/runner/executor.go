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
	"recac/internal/telemetry"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var bashBlockRegex = regexp.MustCompile("(?s)```bash\\s*(.*?)\\s*```")

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

			// Fail Fast: Do not execute subsequent commands if the current one fails
			break
		} else {
			// Output Truncation to prevent context exhaustion
			const MaxOutputChars = 20000
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

// runCleanerAgent removes temporary files listed in temp_files.txt.
func (s *Session) runCleanerAgent(ctx context.Context) error {
	s.Logger.Info("cleaner agent running")

	// Check if temp_files.txt exists
	tempFilesPath := filepath.Join(s.Workspace, "temp_files.txt")
	if _, err := os.Stat(tempFilesPath); os.IsNotExist(err) {
		s.Logger.Info("no temp_files.txt found")
		return nil // Nothing to clean
	}

	data, err := os.ReadFile(tempFilesPath)
	if err != nil {
		return fmt.Errorf("failed to read temp_files.txt: %w", err)
	}

	// Parse temp files (one per line)
	lines := strings.Split(string(data), "\n")
	cleaned := 0
	errors := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		// Handle both relative and absolute paths
		var filePath string
		if filepath.IsAbs(line) {
			filePath = filepath.Clean(line)
		} else {
			filePath = filepath.Join(s.Workspace, line)
		}

		// Security Check: Ensure path is inside workspace
		// 1. Resolve symlinks in the target path
		realPath, err := filepath.EvalSymlinks(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // File doesn't exist, nothing to clean
			}
			s.Logger.Warn("failed to resolve symlinks", "path", filePath, "error", err)
			errors++
			continue
		}

		// 2. Resolve symlinks in the workspace path (to compare apples to apples)
		realWorkspace, err := filepath.EvalSymlinks(s.Workspace)
		if err != nil {
			s.Logger.Warn("failed to resolve workspace path", "path", s.Workspace, "error", err)
			errors++
			continue
		}

		// 3. Check containment
		rel, err := filepath.Rel(realWorkspace, realPath)
		if err != nil {
			s.Logger.Warn("failed to resolve path relative to workspace", "path", realPath, "error", err)
			errors++
			continue
		}

		// Check for path traversal
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			s.Logger.Warn("security violation: attempted path traversal in cleaner agent", "attempted_path", line, "resolved_path", realPath)
			errors++
			continue
		}

		if err := os.Remove(filePath); err != nil {
			if !os.IsNotExist(err) {
				s.Logger.Warn("failed to remove temp file", "file", line, "error", err)
				errors++
			}
		} else {
			s.Logger.Info("removed temp file", "file", line)
			cleaned++
		}
	}

	s.Logger.Info("cleaner agent complete", "removed", cleaned, "errors", errors)

	// Clear the temp_files.txt itself
	os.Remove(tempFilesPath)

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
