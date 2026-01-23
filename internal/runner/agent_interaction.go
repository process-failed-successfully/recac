package runner

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"strings"

	"github.com/spf13/viper"
)

// SelectPrompt determines which prompt to send based on current state.
func (s *Session) SelectPrompt() (string, string, bool, error) {
	// 1. Initializer (Session 1)
	// 1. Initializer Check (Run if feature_list.json is missing or empty)
	// Only for main session (not sub-sessions) and not if ManagerFirst is active on iteration 1
	if s.SelectedTaskID == "" {
		runInitializer := false

		// If ManagerFirst is requested on Iteration 1, we skip Initializer for now
		// (Manager might create it, or we'll loop back and hit this again later if Manager doesn't)
		if s.GetIteration() == 1 && s.ManagerFirst {
			// Manager First: Skip Initializer, go straight to Manager prompt
			// ... (existing logic for ManagerFirst)
			qaReport := "Initial Planning Phase. No code implemented yet."
			prompt, err := prompts.GetPrompt(prompts.ManagerReview, map[string]string{
				"qa_report": qaReport,
			})
			return prompt, prompts.ManagerReview, true, err
		}

		// Check for existing features (DB, Injected, or File)
		features := s.loadFeatures()
		if len(features) > 0 {
			// Features exist, so we don't need to run Initializer.
			// s.loadFeatures() automatically syncs to file if found in DB.
		} else {
			// No features found anywhere. Run Initializer.
			fmt.Println("Feature list not found (in DB, Content, or File). Running Initializer.")
			runInitializer = true
		}

		if runInitializer {
			spec, _ := s.ReadSpec()
			prompt, err := prompts.GetPrompt(prompts.Initializer, map[string]string{
				"spec": spec,
			})
			return prompt, prompts.Initializer, false, err
		}
	}

	// 2. Manager Review (Triggered by file or frequency) - Main Session Only
	if s.SelectedTaskID == "" && (s.GetIteration()%s.ManagerFrequency == 0 || s.hasSignal("TRIGGER_MANAGER")) {
		// Cleanup signal
		s.clearSignal("TRIGGER_MANAGER")

		features := s.loadFeatures()

		qaReport := RunQA(features)

		vars := map[string]string{
			"qa_report": qaReport.String(),
		}

		// Inject Stall Warning if active
		if s.hasSignal("STALLED_WARNING") {
			s.clearSignal("STALLED_WARNING") // Clear after consuming
			vars["stall_warning"] = fmt.Sprintf("CRITICAL WARNING: The Coding Agent has stalled for %d iterations. You must intervene. Review their recent history and provide specific redirection instructions or STOP the project.", s.StalledCount)
		}

		prompt, err := prompts.GetPrompt(prompts.ManagerReview, vars)
		return prompt, prompts.ManagerReview, true, err
	}

	// 3. Coding Agent (Default)
	var historyStr string
	if s.DBStore != nil {
		// Limit history size to prevent context exhaustion (413 errors)
		const MaxHistoryChars = 25000                     // approx 6k tokens, safe for most models
		obs, err := s.DBStore.QueryHistory(s.Project, 20) // Fetch more, but we'll filter by size
		if err == nil {
			var sb strings.Builder

			// Calculate how many observations fit within the limit
			// obs is ordered by created_at DESC (Newest First)
			var includedObs []db.Observation
			currentSize := 0

			for _, o := range obs {
				// Estimate size: Content + Overhead
				size := len(o.Content) + len(o.AgentID) + 20
				if currentSize+size > MaxHistoryChars {
					break
				}
				includedObs = append(includedObs, o)
				currentSize += size
			}

			// Build string in Chronological Order (Oldest -> Newest)
			// includedObs is still [Newest, ..., Oldest-Fitting]
			for i := len(includedObs) - 1; i >= 0; i-- {
				sb.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", includedObs[i].AgentID, includedObs[i].Content))
			}
			historyStr = sb.String()
		}
	}

	vars := map[string]string{
		"history": historyStr,
	}

	// Populate task-specific variables if set
	// 4. Deterministic Task Assignment (User Request: Remove agent reliance on jq)
	// Find the first pending feature and assign it explicitly.
	var assignedFeature *db.Feature
	features := s.loadFeatures() // Refresh from DB/File

	for i := range features {
		if features[i].Status != "done" && !features[i].Passes {
			assignedFeature = &features[i]
			break
		}
	}

	if assignedFeature != nil {
		vars["task_id"] = assignedFeature.ID
		vars["task_description"] = assignedFeature.Description
		vars["exclusive_paths"] = strings.Join(assignedFeature.Dependencies.ExclusiveWritePaths, ", ")
		vars["read_only_paths"] = strings.Join(assignedFeature.Dependencies.ReadOnlyPaths, ", ")

		// s.SelectedTaskID = assignedFeature.ID // DO NOT SET THIS: It prevents Manager interruptions in subsequent turns.
	} else {
		// All done?
		vars["task_id"] = "NONE_ALL_COMPLETE"
		vars["task_description"] = "All features are marked as done/passing. Please run final verification and signal completion."
		vars["exclusive_paths"] = "none"
		vars["read_only_paths"] = "all"
	}
	if s.SelectedTaskID != "" {
		features := s.loadFeatures()
		var target db.Feature
		for _, f := range features {
			if f.ID == s.SelectedTaskID {
				target = f
				break
			}
		}

		if target.ID != "" {
			vars["task_id"] = target.ID

			// Defensive Truncation: Restrict description size to prevent context exhaustion
			desc := target.Description
			const MaxDescriptionChars = 20000
			if len(desc) > MaxDescriptionChars {
				s.Logger.Warn("task description truncated", "original_len", len(desc), "limit", MaxDescriptionChars)
				desc = desc[:MaxDescriptionChars] + "\n\n... [Description Truncated due to size] ..."
			}
			vars["task_description"] = desc

			vars["exclusive_paths"] = strings.Join(target.Dependencies.ExclusiveWritePaths, ", ")
			vars["read_only_paths"] = strings.Join(target.Dependencies.ReadOnlyPaths, ", ")
		} else {
			vars["task_id"] = s.SelectedTaskID
			vars["task_description"] = "No description found in feature_list.json"
			vars["exclusive_paths"] = "None"
			vars["read_only_paths"] = "None"
		}
	} else {
		vars["task_id"] = "Multiple/Not Assigned"
		vars["task_description"] = "Continue implementing pending features in feature_list.json"
		vars["exclusive_paths"] = "All available files"
		vars["read_only_paths"] = "All available files"
	}

	prompt, err := prompts.GetPrompt(prompts.CodingAgent, vars)
	return prompt, prompts.CodingAgent, false, err
}

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

	// 1.5 Clear any existing QA signal to ensure fresh result
	s.clearSignal("QA_PASSED")

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

	// 3. Check DB Signal (Authoritative)
	// We read the raw signal value. "true" = PASS, "false" (or missing) = FAIL.
	// Note: checking "false" explicitly allows us to distinguish between "agent said fail" and "agent did nothing".
	val, err := s.DBStore.GetSignal(s.Project, "QA_PASSED")
	s.Logger.Info("QA result signal check", "signal", val, "error", err)

	if err == nil && val == "true" {
		s.Logger.Info("QA passed (signal verified)")
		return nil
	}

	if val == "false" {
		s.Logger.Error("QA failed (explicit signal)")
		return fmt.Errorf("QA Agent explicitly signaled failure")
	}

	// Fallback/Legacy/Missing Signal
	s.Logger.Error("QA failed (no signal set)",
		"agent_response_snippet", response[:min(len(response), 1000)])
	return fmt.Errorf("QA Agent did not signal success (QA_PASSED!=true)")
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
