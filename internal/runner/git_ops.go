package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/git"
	"recac/internal/notify"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

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
.recac.db-wal
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

// pushProgress commits and pushes the current state of the workspace to the current branch.
func (s *Session) pushProgress(ctx context.Context) {
	if s.Workspace == "" {
		return
	}

	gitClient := git.NewClient()
	if !gitClient.RepoExists(s.Workspace) {
		return
	}

	// Safety Check: Ensure state files are ignored
	_ = EnsureStateIgnored(s.Workspace)

	// Ensure Host has permissions to read what Agent (Root) wrote (e.g. .git/refs/heads/master)
	if !s.UseLocalAgent {
		if err := s.fixPermissions(ctx); err != nil {
			s.Logger.Warn("failed to fix permissions before push", "error", err)
		}
	}

	branch, err := gitClient.CurrentBranch(s.Workspace)
	if err != nil || branch == "" || branch == "main" || branch == "master" {
		// Don't auto-push to main/master
		return
	}

	s.Logger.Info("pushing progress to remote", "branch", branch)

	// Commit any changes (ignore error if nothing to commit)
	msg := fmt.Sprintf("chore: progress update (iteration %d)", s.GetIteration())
	_ = gitClient.Commit(s.Workspace, msg)

	// Workaround: Agent might have run 'git init' which resets HEAD to master in the container
	// We merge master into current branch to capture those commits if they exist
	if branch != "master" && branch != "main" {
		// Try explicit refs to avoid ambiguity or missing short names
		candidates := []string{"refs/heads/master", "refs/heads/main", "master", "main"}
		merged := false
		for _, ref := range candidates {
			if err := gitClient.Merge(s.Workspace, ref); err == nil {
				s.Logger.Info("merged stranded commits from ref", "ref", ref)
				merged = true
				break
			}
		}
		if !merged {
			s.Logger.Debug("no stranded commits merged from master/main")
		}
	}

	// Push progress
	if err := gitClient.Push(s.Workspace, branch); err != nil {
		s.Logger.Warn("failed to push progress", "error", err)
	}
}

// EnsureConflictTask checks if "Resolve Merge Conflicts" task exists, otherwise adds it.
func (s *Session) EnsureConflictTask() {
	if s.DBStore == nil {
		return
	}
	features := s.loadFeatures()
	conflictTaskID := "CONFLICT_RES"
	needsUpdate := false

	// Check if already exists/pending
	for idx, f := range features {
		if f.ID == conflictTaskID {
			if f.Status == "done" || f.Status == "implemented" || f.Passes {
				// Reset it to todo since we have a NEW conflict
				features[idx].Status = "todo"
				features[idx].Passes = false
				needsUpdate = true
			}
			break
		}
	}

	// Add new if not found (needsUpdate loop below handles the save)
	found := false
	for _, f := range features {
		if f.ID == conflictTaskID {
			found = true
			break
		}
	}

	if !found {
		newFeature := db.Feature{
			ID:          conflictTaskID,
			Category:    "Guardrail",
			Priority:    "Critical",
			Description: fmt.Sprintf("Resolve git merge conflicts with branch %s. Files contain conflict markers (<<<< HEAD). Fix them and commit.", s.BaseBranch),
			Status:      "todo",
			Passes:      false,
		}
		features = append(features, newFeature)
		needsUpdate = true
	}

	if needsUpdate {
		fl := db.FeatureList{Features: features}
		data, err := json.Marshal(fl)
		if err == nil {
			_ = s.DBStore.SaveFeatures(s.Project, string(data))
		}
	}
}

// completeJiraTicket performs the final Jira transition, adds a comment with the link, and sends a notification.
func (s *Session) completeJiraTicket(ctx context.Context, gitLink string) {
	if s.JiraClient == nil || (reflect.ValueOf(s.JiraClient).Kind() == reflect.Ptr && reflect.ValueOf(s.JiraClient).IsNil()) || s.JiraTicketID == "" {
		// Not a Jira session, but we still send a notification
		s.Notifier.Notify(ctx, notify.EventProjectComplete, fmt.Sprintf("Project %s is COMPLETE! Git: %s", s.Project, gitLink), s.GetSlackThreadTS())
		return
	}

	fmt.Printf("[%s] Finalizing Jira ticket...\n", s.JiraTicketID)

	// 1. Add Comment with Link
	comment := fmt.Sprintf("RECAC session completed successfully.\n\nGit Link: %s", gitLink)
	if err := s.JiraClient.AddComment(ctx, s.JiraTicketID, comment); err != nil {
		fmt.Printf("[%s] Warning: Failed to add Jira comment: %v\n", s.JiraTicketID, err)
	} else {
		fmt.Printf("[%s] Jira comment added with Git link.\n", s.JiraTicketID)
	}

	// 2. Transition to Done
	// We use "Done" as the default target status, but it could be configurable
	targetStatus := viper.GetString("jira.done_status")
	if targetStatus == "" {
		targetStatus = "Done"
	}

	fmt.Printf("[%s] Transitioning ticket to '%s'...\n", s.JiraTicketID, targetStatus)
	if err := s.JiraClient.SmartTransition(ctx, s.JiraTicketID, targetStatus); err != nil {
		fmt.Printf("[%s] Warning: Failed to transition Jira ticket to %s: %v\n", s.JiraTicketID, targetStatus, err)
	} else {
		fmt.Printf("[%s] Jira ticket transitioned to %s.\n", s.JiraTicketID, targetStatus)
	}

	// 3. Send Notification with Links
	jiraURL := viper.GetString("jira.url")
	if jiraURL == "" {
		jiraURL = os.Getenv("JIRA_URL")
	}
	jiraLink := fmt.Sprintf("%s/browse/%s", jiraURL, s.JiraTicketID)

	notificationMsg := fmt.Sprintf("Project %s is COMPLETE!\n\nJira: %s\nGit: %s", s.Project, jiraLink, gitLink)
	s.Notifier.Notify(ctx, notify.EventProjectComplete, notificationMsg, s.GetSlackThreadTS())
	s.Notifier.AddReaction(ctx, s.GetSlackThreadTS(), "white_check_mark")
}
