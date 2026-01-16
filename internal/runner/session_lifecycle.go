package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"strings"

	"github.com/spf13/viper"
)

// runQAAgent runs quality assurance checks on the feature list.
// Returns error if QA fails, nil if QA passes.
func (s *Session) runQAAgent(ctx context.Context) error {
	s.Logger.Info("QA agent running quality checks")

	var qaAgent agent.Agent
	if s.QAAgent != nil {
		qaAgent = s.QAAgent
	} else {
		var err error
		// Resolve Config
		provider := s.AgentProvider
		if provider == "" {
			provider = viper.GetString("agents.qa.provider")
			if provider == "" {
				provider = viper.GetString("provider") // Fallback to global setting
				if provider == "" {
					provider = "gemini"
				}
			}
		}

		model := s.AgentModel
		if model == "" {
			model = viper.GetString("agents.qa.model")
			if model == "" {
				model = viper.GetString("model") // Fallback to global setting
				if model == "" {
					model = "gemini-1.5-flash-latest" // Ultimate fallback
				}
			}
		}
		apiKey := viper.GetString("agents.qa.api_key")
		if apiKey == "" {
			// Fallback to global API key
			apiKey = viper.GetString("api_key")
			if apiKey == "" {
				// Try provider-specific env vars
				if provider == "openrouter" {
					apiKey = os.Getenv("OPENROUTER_API_KEY")
				} else if provider == "gemini" || provider == "gemini-cli" {
					apiKey = os.Getenv("GEMINI_API_KEY")
				} else if provider == "openai" {
					apiKey = os.Getenv("OPENAI_API_KEY")
				}

				// Final catch-all if still empty (legacy support)
				if apiKey == "" {
					apiKey = os.Getenv("GEMINI_API_KEY")
				}
			}
		}

		s.Logger.Info("initializing QA agent", "provider", provider, "model", model)
		qaAgent, err = agent.NewAgent(provider, apiKey, model, s.Workspace, s.Project)
		if err != nil {
			return fmt.Errorf("failed to create QA agent: %w", err)
		}
	}

	// 1. Get Prompt
	prompt, err := prompts.GetPrompt(prompts.QAAgent, nil)
	if err != nil {
		return fmt.Errorf("failed to load QA prompt: %w", err)
	}

	// 2. Send to Agent
	s.Logger.Info("sending verification instructions to QA agent")
	response, err := qaAgent.Send(ctx, prompt) // Use qaAgent
	if err != nil {
		return fmt.Errorf("QA Agent failed to respond: %w", err)
	}
	s.Logger.Info("QA agent response received", "chars", len(response))

	// 2.5 Execute Commands
	if _, err := s.ProcessResponse(ctx, response); err != nil {
		s.Logger.Warn("QA agent command execution failed", "error", err)
	}

	// 3. Check Result File (.qa_result)
	qaResultPath := filepath.Join(s.Workspace, ".qa_result")
	data, err := os.ReadFile(qaResultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("QA Agent did not produce .qa_result file")
		}
		return fmt.Errorf("failed to read .qa_result: %w", err)
	}
	defer os.Remove(qaResultPath) // Cleanup

	result := strings.TrimSpace(string(data))
	s.Logger.Info("QA result", "result", result)

	if result == "PASS" {
		if err := s.createSignal("QA_PASSED"); err != nil {
			s.Logger.Warn("failed to create QA_PASSED signal", "error", err)
		}
		s.Logger.Info("QA passed")
		return nil
	}

	s.Logger.Error("QA failed")
	return fmt.Errorf("QA failed with result: %s", result)
}

// runManagerAgent runs manager review of the QA report.
// Returns error if manager rejects, nil if manager approves.
func (s *Session) runManagerAgent(ctx context.Context) error {
	s.Logger.Info("manager agent reviewing QA report")

	var managerAgent agent.Agent
	if s.ManagerAgent != nil {
		managerAgent = s.ManagerAgent
	} else {
		var err error
		// Resolve Config
		provider := s.AgentProvider
		if provider == "" {
			provider = viper.GetString("agents.manager.provider")
			if provider == "" {
				provider = viper.GetString("provider") // Fallback to global setting
				if provider == "" {
					provider = "gemini-cli"
				}
			}
		}
		model := s.AgentModel
		if model == "" {
			model = viper.GetString("agents.manager.model")
			if model == "" {
				model = viper.GetString("model")
				if model == "" {
					model = "gemini-1.5-pro-latest"
				}
			}
		}
		apiKey := viper.GetString("agents.manager.api_key")
		if apiKey == "" {
			apiKey = viper.GetString("api_key")
			if apiKey == "" {
				// Try provider-specific env vars
				if provider == "openrouter" {
					apiKey = os.Getenv("OPENROUTER_API_KEY")
				} else if provider == "gemini" || provider == "gemini-cli" {
					apiKey = os.Getenv("GEMINI_API_KEY")
				} else if provider == "openai" {
					apiKey = os.Getenv("OPENAI_API_KEY")
				}

				if apiKey == "" {
					apiKey = os.Getenv("GEMINI_API_KEY")
				}
			}
		}

		fmt.Printf("Initialising Manager Agent with provider: %s, model: %s\n", provider, model)
		managerAgent, err = agent.NewAgent(provider, apiKey, model, s.Workspace, s.Project)
		if err != nil {
			return fmt.Errorf("failed to create manager agent: %w", err)
		}
	}

	features := s.loadFeatures()
	qaReport := RunQA(features)

	// Create manager review prompt
	prompt, err := prompts.GetPrompt(prompts.ManagerReview, map[string]string{
		"qa_report": qaReport.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to load manager review prompt: %w", err)
	}

	// Send to agent for review
	s.Logger.Info("sending QA report to manager agent")
	response, err := managerAgent.Send(ctx, prompt) // Use managerAgent
	if err != nil {
		return fmt.Errorf("manager review request failed: %w", err)
	}

	s.Logger.Info("manager review response received", "chars", len(response))

	// Execute commands (e.g., creating PROJECT_SIGNED_OFF or deleting COMPLETED)
	// Execute commands (e.g., creating PROJECT_SIGNED_OFF or deleting COMPLETED)
	if _, err := s.ProcessResponse(ctx, response); err != nil {
		s.Logger.Warn("manager agent command execution failed", "error", err)
	}

	// Check for PROJECT_SIGNED_OFF signal
	if s.hasSignal("PROJECT_SIGNED_OFF") {
		s.Logger.Info("manager approved, project signed off via signal")
		return nil
	}

	// Fallback to legacy ratio check if no explicit signal was given
	if qaReport.CompletionRatio >= 1.0 {
		s.Logger.Info("manager approved (legacy/fallback), all features passing")
		return nil
	}

	// Manager rejected or didn't explicitly sign off
	// Manager rejected or didn't explicitly sign off
	s.Logger.Info("manager rejected or pending, project not signed off")
	s.clearSignal("QA_PASSED")
	s.clearSignal("COMPLETED")
	return fmt.Errorf("manager review did not result in sign-off (ratio: %.2f)", qaReport.CompletionRatio)
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
			filePath = line
		} else {
			filePath = filepath.Join(s.Workspace, line)
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

// checkAutoQA checks if all features pass and we haven't already passed QA/Completed
func (s *Session) checkAutoQA() bool {
	if s.hasSignal("QA_PASSED") || s.hasSignal("COMPLETED") || s.hasSignal("PROJECT_SIGNED_OFF") {
		return false
	}

	features := s.loadFeatures()
	if len(features) == 0 {
		return false
	}

	// Use shared logic from qa.go
	report := RunQA(features)

	if report.FailedFeatures == 0 {
		if err := s.createSignal("COMPLETED"); err != nil {
			fmt.Printf("Warning: Failed to create COMPLETED signal: %v\n", err)
		}
		return true
	}

	return false
}
