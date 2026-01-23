package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent/prompts"
	"recac/internal/git"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"strings"
	"time"
)

// RunLoop executes the autonomous agent loop.
func (s *Session) RunLoop(ctx context.Context) error {
	// Guard: Ensure Notifier is initialized (mostly for tests using manual struct initialization)
	if s.Notifier == nil {
		s.Notifier = notify.NewManager(func(string, ...interface{}) {})
	}

	// Guard: Ensure SleepFunc is initialized
	if s.SleepFunc == nil {
		s.SleepFunc = time.Sleep
	}

	s.Logger.Info("entering autonomous run loop")
	// Note: We use the stored SlackThreadTS if available (from startup), otherwise we start a new thread here if needed?
	// But Start() is called before RunLoop(), so s.SlackThreadTS should be set if notifications are enabled.
	// If it's a resume and we don't have the TS persisted, we might start a new thread.
	// For now, let's just log if it's not set.
	if s.GetSlackThreadTS() == "" {
		// Try to send a start message if we missed it (e.g. manual RunLoop call)
		ts, _ := s.Notifier.Notify(ctx, notify.EventStart, fmt.Sprintf("Session Started for Project: %s", s.Project), "")
		s.SetSlackThreadTS(ts)
	} else {
		// Just log context update if needed, but "Session Started" is redundant if checking duplicates.
		// User complained about DUPLICATE messages. If Start() already sent one, RunLoop shouldn't send another top-level one.
		// So we ONLY send if s.SlackThreadTS is empty.
	}

	// Guardrail: Ensure app_spec.txt exists (Source of Truth)
	// We skip this check for Mock mode users who might not have set it up, but for real usage it's mandatory.
	// Actually, user said "Immediately fail if there is no app_spec.txt", so we enforce it strict.
	specPath := filepath.Join(s.Workspace, "app_spec.txt")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("CRITICAL ERROR: app_spec.txt not found in workspace (%s). This file is required as the source of truth for the project.", s.Workspace)
	}

	// Load agent state if it exists (for session restoration)
	if err := s.LoadAgentState(); err != nil {
		fmt.Printf("Warning: Failed to load agent state: %v\n", err)
		// Continue anyway - state will be created on first save
	}

	// Load DB history if available
	if s.DBStore != nil {
		history, err := s.DBStore.QueryHistory(s.Project, 5)
		if err == nil && len(history) > 0 {
			fmt.Printf("Loaded %d previous observations from DB history.\n", len(history))
		}
	}

	// Startup Check: If feature list exists and all passed, mark COMPLETED
	features := s.loadFeatures()
	if len(features) > 0 {
		allPassed := true
		for _, f := range features {
			if !(f.Passes || f.Status == "done" || f.Status == "implemented") {
				allPassed = false
				break
			}
		}
		if allPassed {
			fmt.Println("All features passed! Triggering Project Complete flow.")
			if err := s.createSignal("COMPLETED"); err != nil {
				fmt.Printf("Warning: Failed to create COMPLETED signal: %v\n", err)
			}

			// Final Phase: UI Verification Check
			s.Notifier.Notify(ctx, notify.EventProjectComplete, fmt.Sprintf("Project %s is COMPLETE!", s.Project), s.GetSlackThreadTS())
		}
	}

	// Ensure cleanup on exit (defer cleanup)
	defer func() {
		containerID := s.GetContainerID()
		if containerID != "" {
			fmt.Printf("Cleaning up container: %s\n", containerID)
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if s.Docker != nil {
				if err := s.Docker.StopContainer(cleanupCtx, containerID); err != nil {
					fmt.Printf("Warning: Failed to cleanup container: %v\n", err)
				} else {
					fmt.Println("Container cleaned up successfully")
				}
			}
		}
	}()

	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check Max Iterations
		currentIteration := s.GetIteration()
		if s.MaxIterations > 0 && currentIteration >= s.MaxIterations {
			s.Logger.Info("reached max iterations", "max_iterations", s.MaxIterations)
			return ErrMaxIterations
		}

		newIteration := s.IncrementIteration()
		s.Logger.Info("starting iteration", "iteration", newIteration, "task_id", s.SelectedTaskID, "agent_provider", s.AgentProvider, "agent_model", s.AgentModel)
		if s.SelectedTaskID != "" {
			// Log task description snippet for debugging context
			descSnippet := ""
			if len(s.SpecContent) > 50 {
				descSnippet = s.SpecContent[:50] + "..."
			} else {
				descSnippet = s.SpecContent
			}
			s.Logger.Info("assigned task details", "task_id", s.SelectedTaskID, "desc_snippet", descSnippet)
		}

		// Ensure feature list is synced and mirror is up to date
		features = s.loadFeatures()

		// Single-Task Termination: If we are assigned a specific task and it's done, exit.
		if s.SelectedTaskID != "" {
			for _, f := range features {
				if f.ID == s.SelectedTaskID && (f.Passes || f.Status == "done" || f.Status == "implemented") {
					s.Logger.Info("task completed", "task_id", s.SelectedTaskID)
					return nil
				}
			}
		}

		// Handle Lifecycle Role Transitions (Agent-QA-Manager-Cleaner workflow)
		// Prioritize these checks at the beginning of the iteration
		if s.hasSignal("PROJECT_SIGNED_OFF") {
			// MERGE GUARDRAIL: Check for upstream conflicts before accepting sign-off
			if s.BaseBranch != "" {
				s.Logger.Info("checking for upstream changes", "branch", s.BaseBranch)

				// Git Recovery/Retry Loop
				maxRetries := 3
				gitClient := git.NewClient()
				success := false

				for i := 0; i < maxRetries; i++ {
					// 1. Fix Permissions
					if err := s.fixPermissions(ctx); err != nil {
						fmt.Printf("Warning: Failed to fix permissions (attempt %d/%d): %v\n", i+1, maxRetries, err)
					}

					// 2. Fetch
					if err := gitClient.Fetch(s.Workspace, "origin", s.BaseBranch); err == nil {
						// Stash (ignore errors)
						_ = gitClient.Stash(s.Workspace)

						// 3. Attempt Merge
						if err := gitClient.Merge(s.Workspace, "origin/"+s.BaseBranch); err != nil {
							s.Logger.Warn("merge failed", "attempt", i+1, "max", maxRetries, "error", err)

							// ENSURE WE ABORT to clear unmerged files
							_ = gitClient.AbortMerge(s.Workspace)

							// RECOVERY STRATEGIES
							if i < maxRetries-1 {
								s.Logger.Info("attempting git recovery")

								// Recovery Step 1: Remove Locks
								if err := gitClient.Recover(s.Workspace); err != nil {
									s.Logger.Warn("recover failed", "error", err)
								}

								// Recovery Step 2: Clean aggressively
								if err := gitClient.Clean(s.Workspace); err != nil {
									s.Logger.Warn("clean failed", "error", err)
								}

								// Recovery Step 3: Hard Reset to origin/current_feature_branch
								// This is safer than just 'reset --hard' without target
								cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
								cmd.Dir = s.Workspace
								if out, err := cmd.Output(); err == nil {
									currBranch := strings.TrimSpace(string(out))
									_ = gitClient.ResetHard(s.Workspace, "origin", currBranch)
								}
							} else {
								// Final Failure
								s.Logger.Error("critical merge failure", "branch", s.BaseBranch, "attempts", maxRetries)
							}
						} else {
							// Success
							success = true
							if err := gitClient.StashPop(s.Workspace); err != nil {
								s.Logger.Warn("restore stash failed", "error", err)
							}
							s.Logger.Info("branch up-to-date with base")
							break
						}
					} else {
						s.Logger.Warn("fetch failed", "attempt", i+1, "max", maxRetries, "error", err)
						gitClient.Recover(s.Workspace) // Try recovering for next loop
					}
					s.SleepFunc(2 * time.Second)
				}

				if !success {
					s.Logger.Warn("merge conflict or persistent git error, revoking sign-off", "branch", s.BaseBranch)

					// BRUTAL RECOVERY: If standard recovery fails, delete remote feature branch
					// and let the agent start clean on next iteration.
					if s.JiraTicketID != "" {
						cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
						cmd.Dir = s.Workspace
						if out, err := cmd.Output(); err == nil {
							featureBranch := strings.TrimSpace(string(out))
							if featureBranch != s.BaseBranch && !strings.Contains(featureBranch, "HEAD") {
								fmt.Printf("[%s] BRUTAL RECOVERY: Deleting remote branch %s to clear conflict.\n", s.JiraTicketID, featureBranch)
								_ = gitClient.DeleteRemoteBranch(s.Workspace, "origin", featureBranch)
							}
						}
						// Hard reset to base branch state to ensures clean slate
						fmt.Printf("[%s] Resetting workspace to %s...\n", s.JiraTicketID, s.BaseBranch)
						_ = gitClient.ResetHard(s.Workspace, "origin", s.BaseBranch)
					}

					s.clearSignal("PROJECT_SIGNED_OFF")
					s.EnsureConflictTask()
					s.clearSignal("QA_PASSED")
					s.clearSignal("COMPLETED")
					continue
				}
			}

			// CRITICAL: Guardrail against premature sign-off.
			// Validate that ALL features are actually passing before accepting the sign-off.
			features := s.loadFeatures()
			incompleteFeatures := []string{}
			for _, f := range features {
				if !(f.Passes || f.Status == "done" || f.Status == "implemented") {
					incompleteFeatures = append(incompleteFeatures, f.ID)
				}
			}

			if len(incompleteFeatures) > 0 {

				s.Logger.Warn("premature project sign-off detected", "incomplete_features", incompleteFeatures)

				// Revoke signal
				s.clearSignal("PROJECT_SIGNED_OFF")
				// Also clear QA_PASSED to force re-verification
				s.clearSignal("QA_PASSED")
				// Also clear COMPLETED to force re-check
				s.clearSignal("COMPLETED")

				s.Logger.Info("returning to coding phase")
				continue
			}

			if s.SelectedTaskID != "" {
				fmt.Println("Project signed off. Sub-session exiting.")
				return nil
			}

			// Auto-Merge Logic
			if s.AutoMerge && s.BaseBranch != "" {
				fmt.Printf("Auto-Merge enabled. Preparing to merge changes into base branch: %s\n", s.BaseBranch)

				// 0. COMMIT WORK: Ensure any pending changes are committed before merging
				// We use a more careful commit strategy to avoid re-adding ignored files
				commitCmd := exec.Command("sh", "-c", "git add . && git commit -m 'feat: implemented features for "+s.Project+"' || echo 'Nothing to commit'")
				commitCmd.Dir = s.Workspace
				if out, err := commitCmd.CombinedOutput(); err != nil {
					fmt.Printf("Warning: Failed to auto-commit work: %v\nOutput: %s\n", err, out)
				} else {
					fmt.Printf("Auto-committed work: %s\n", strings.TrimSpace(string(out)))
				}

				fmt.Printf("Merging changes into base branch: %s\n", s.BaseBranch)
				gitClient := git.NewClient()
				// Actually, we are IN the workspace, so we can get current branch name
				// But simpler: checkout BaseBranch -> Merge Previous -> Push

				// 1. Get current branch name
				cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
				cmd.Dir = s.Workspace
				out, err := cmd.Output()
				if err != nil {
					fmt.Printf("Warning: Failed to get current branch for auto-merge: %v\n", err)
				} else {
					featureBranch := strings.TrimSpace(string(out))

					// 2. Checkout Base Branch
					if err := gitClient.Checkout(s.Workspace, s.BaseBranch); err != nil {
						fmt.Printf("Warning: Auto-merge failed (checkout base): %v\n", err)
					} else {
						// 3. Merge Feature Branch
						if err := gitClient.Merge(s.Workspace, featureBranch); err != nil {
							fmt.Printf("Warning: Auto-merge failed (merge): %v\n", err)
							// ENSURE WE ABORT
							_ = gitClient.AbortMerge(s.Workspace)
							_ = gitClient.Recover(s.Workspace)
						} else {
							// 4. Push Base Branch
							if err := gitClient.Push(s.Workspace, s.BaseBranch); err != nil {
								fmt.Printf("Warning: Auto-merge failed (push): %v\n", err)
								// If push fails (likely race), abort the merge locally too so we can retry from clean state
								_ = gitClient.AbortMerge(s.Workspace)
							} else {
								fmt.Printf("Successfully auto-merged %s into %s and pushed.\n", featureBranch, s.BaseBranch)

								// DELETE REMOTE FEATURE BRANCH (Cleanup)
								// This keeps the repo clean and prevents branch accumulation
								fmt.Printf("[%s] Deleting remote feature branch %s...\n", s.Project, featureBranch)
								if err := gitClient.DeleteRemoteBranch(s.Workspace, "origin", featureBranch); err != nil {
									fmt.Printf("[%s] Warning: Failed to delete remote branch: %v\n", s.Project, err)
								}

								// 6. Capture Commit SHA for links
								commitSHA := ""
								shaCmd := exec.Command("git", "rev-parse", "HEAD")
								shaCmd.Dir = s.Workspace
								if shaOut, err := shaCmd.Output(); err == nil {
									commitSHA = strings.TrimSpace(string(shaOut))
								}

								// 7. Transition Jira and notify with commit link
								gitLink := s.RepoURL
								if commitSHA != "" {
									gitLink = fmt.Sprintf("%s/commit/%s", s.RepoURL, commitSHA)
								}
								s.completeJiraTicket(ctx, gitLink)
							}
						}
						// 5. Checkout back to feature branch (nice to have)
						_ = gitClient.Checkout(s.Workspace, featureBranch)
					}
				}
			} else {
				// No auto-merge or no base branch. Just push the feature branch and complete.
				cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
				cmd.Dir = s.Workspace
				if out, err := cmd.Output(); err == nil {
					featureBranch := strings.TrimSpace(string(out))
					// Push current branch
					gitClient := git.NewClient()
					if err := gitClient.Push(s.Workspace, featureBranch); err == nil {
						gitLink := fmt.Sprintf("%s/tree/%s", s.RepoURL, featureBranch)
						s.completeJiraTicket(ctx, gitLink)
					}
				}
			}

			s.Logger.Info("project signed off, running cleaner agent")
			if err := s.runCleanerAgent(ctx); err != nil {
				s.Logger.Error("cleaner agent error", "error", err)
			}
			s.Logger.Info("cleaner agent complete, session finished")
			return nil
		}

		// Global Lifecycle Transitions (QA/Manager) - Main Session Only
		if s.SelectedTaskID == "" {
			if s.hasSignal("QA_PASSED") {
				fmt.Println("QA passed. Running Manager agent for final review...")
				if err := s.runManagerAgent(ctx); err != nil {
					fmt.Printf("Manager agent error: %v\n", err)
					fmt.Println("Manager review failed. Returning to coding phase.")
				} else {
					// Manager approved - create PROJECT_SIGNED_OFF
					if err := s.createSignal("PROJECT_SIGNED_OFF"); err != nil {
						fmt.Printf("Warning: Failed to create PROJECT_SIGNED_OFF: %v\n", err)
					}
					fmt.Println("Manager approved. Project signed off.")
					s.Notifier.Notify(ctx, notify.EventSuccess, fmt.Sprintf("Project %s Signed Off by Manager!", s.Project), s.GetSlackThreadTS())
					continue // Next iteration will run Cleaner
				}
			}

			if s.hasSignal("COMPLETED") {
				// Skip QA if requested (useful for smoketests/verification)
				if s.SkipQA {
					fmt.Println("SkipQA enabled. Bypassing QA agent and Manager review.")
					s.createSignal("PROJECT_SIGNED_OFF")
					s.clearSignal("COMPLETED")
					continue
				}

				fmt.Println("Project marked as COMPLETED. Running QA agent...")
				if err := s.runQAAgent(ctx); err != nil {
					fmt.Printf("QA agent error: %v\n", err)
					// QA failed - clear COMPLETED and continue coding
					s.clearSignal("COMPLETED")
					fmt.Println("QA checks failed. Returning to coding phase.")
				} else {
					// QA passed - create QA_PASSED
					if err := s.createSignal("QA_PASSED"); err != nil {
						fmt.Printf("Warning: Failed to create QA_PASSED signal: %v\n", err)
					}
					fmt.Println("QA checks passed. Moving to Manager review.")
					continue // Next iteration will run Manager
				}
			}
		}

		// Select appropriate prompt and role
		prompt, role, isManager, err := s.SelectPrompt()
		if err != nil {
			fmt.Printf("Error selecting prompt: %v\n", err)
			break
		}

		// Multi-Agent Coding Sprint Delegation
		if role == prompts.CodingAgent && s.MaxAgents > 1 {
			fmt.Printf("Delegating to Multi-Agent Orchestrator (role: %s, max-agents: %d)\n", role, s.MaxAgents)
			orchestrator := NewOrchestrator(s.DBStore, s.Docker, s.Workspace, s.Image, s.Agent, s.Project, s.AgentProvider, s.AgentModel, s.MaxAgents, s.GetSlackThreadTS())
			if err := orchestrator.Run(ctx); err != nil {
				fmt.Printf("Orchestrator sprint failed: %v\n", err)
			}
			// After orchestrator finishes (barrier), we continue the next iteration in the main loop
			if s.checkAutoQA() {
				fmt.Println("Project automatically marked as completed after multi-agent sprint.")
			}
			continue
		}

		// Run iteration using determined prompt
		executionOutput, err := s.RunIteration(ctx, prompt, isManager)

		// Check for Agent/API Error (e.g. 413, Network, etc)
		if err != nil {
			s.Logger.Error("iteration failed", "error", err)
			s.SleepFunc(5 * time.Second) // Backoff
			continue                     // Retry loop without tripping no-op breaker
		}

		// Circuit Breaker: No-Op Check
		if err := s.checkNoOpBreaker(executionOutput); err != nil {
			fmt.Println(err)
			s.Notifier.Notify(ctx, notify.EventFailure, fmt.Sprintf("Project %s Failed: %v", s.Project, err), s.GetSlackThreadTS())
			s.Notifier.AddReaction(ctx, s.GetSlackThreadTS(), "x")
			return ErrNoOp // Exit loop with error
		}

		// Circuit Breaker: Stalled Progress Check
		passingCount := s.checkFeatures()
		if err := s.checkStalledBreaker(role, passingCount); err != nil {
			telemetry.TrackAgentStall(s.Project)
			fmt.Println(err)
			s.Notifier.Notify(ctx, notify.EventFailure, fmt.Sprintf("Project %s Stalled: %v", s.Project, err), s.GetSlackThreadTS())
			s.Notifier.AddReaction(ctx, s.GetSlackThreadTS(), "x")
			return ErrStalled // Exit loop with error
		}

		// Save agent state periodically (every iteration)
		if err := s.SaveAgentState(); err != nil {
			fmt.Printf("Warning: Failed to save agent state: %v\n", err)
		}

		// Push progress to remote periodically (to ensure visibility in Jira/Git)
		s.pushProgress(ctx)

		s.SleepFunc(1 * time.Second)
	}

	// Save final agent state before exiting
	// Save final agent state before exiting
	if err := s.SaveAgentState(); err != nil {
		s.Logger.Warn("failed to save final agent state", "error", err)
	}

	s.Logger.Info("session complete")
	return nil
}

// RunIteration executes a single turn of the autonomous agent.
func (s *Session) RunIteration(ctx context.Context, prompt string, isManager bool) (string, error) {
	role := "Agent"
	if isManager {
		role = "Manager"
	}
	s.Logger.Info("agent role selected", "role", role)

	// Send to Agent
	s.Logger.Info("sending prompt to agent")
	var response string
	var err error

	if s.StreamOutput {
		fmt.Print("Agent Response: ")
		response, err = s.Agent.SendStream(ctx, prompt, func(chunk string) {
			fmt.Print(chunk)
		})
		fmt.Println() // Newline after stream
	} else {
		response, err = s.Agent.Send(ctx, prompt)
	}

	if err != nil {
		s.Logger.Error("agent error, retrying", "error", err)
		return "", err
	}

	s.Logger.Info("agent response received", "role", role, "chars", len(response))

	// Repetition Mitigation
	truncated, wasTruncated := TruncateRepetitiveResponse(response)
	if wasTruncated {
		s.Logger.Warn("agent response truncated due to repetition")
		response = truncated + "\n\n[RESPONSE TRUNCATED DUE TO REPETITION DETECTED]"
	}

	// Security Scan
	if s.Scanner != nil {
		findings, err := s.Scanner.Scan(response)
		if err != nil {
			s.Logger.Warn("security scan failed", "error", err)
		} else if len(findings) > 0 {
			s.Logger.Error("security violation detected")
			for _, f := range findings {
				s.Logger.Error("security finding", "type", f.Type, "desc", f.Description, "line", f.Line)
			}
			return "", fmt.Errorf("security violation detected")
		} else {
			s.Logger.Info("security scan passed")
		}
	}

	// Save observation to DB (only if safe)
	if s.DBStore != nil {
		telemetry.TrackDBOp(s.Project)
		if err := s.DBStore.SaveObservation(s.Project, role, response); err != nil {
			s.Logger.Error("failed to save observation to DB", "error", err)
		} else {
			s.Logger.Debug("saved observation to DB")
		}
	}

	// Process Response (Execute Commands & Check Blockers)
	executionOutput, execErr := s.ProcessResponse(ctx, response)

	// Save System Output to DB (Feedback Loop)
	if s.DBStore != nil && executionOutput != "" {
		telemetry.TrackDBOp(s.Project)
		// Use "System" role for tool outputs
		if err := s.DBStore.SaveObservation(s.Project, "System", executionOutput); err != nil {
			s.Logger.Error("failed to save system output to DB", "error", err)
		} else {
			s.Logger.Debug("saved system output to DB")
		}
	}

	return executionOutput, execErr
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

	allPass := true
	for _, f := range features {
		if !(f.Passes || f.Status == "done" || f.Status == "implemented") {
			allPass = false
			break
		}
	}

	if allPass {
		if err := s.createSignal("COMPLETED"); err != nil {
			fmt.Printf("Warning: Failed to create COMPLETED signal: %v\n", err)
		}
		return true
	}

	return false
}
