//go:build ignore

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"recac/internal/jira"

	"github.com/joho/godotenv"
)

type TicketSpec struct {
	ID       string // Internal ID for linking (e.g., "INIT", "CONFIG")
	Summary  string
	Desc     string
	Type     string
	Blockers []string // List of Internal IDs that block this ticket
	JiraKey  string   // Populated after creation
}

func main() {
	// Load .env
	_ = godotenv.Load()

	baseURL := os.Getenv("JIRA_URL")
	username := os.Getenv("JIRA_USERNAME")
	apiToken := os.Getenv("JIRA_API_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("JIRA_API_KEY")
	}
	projectKey := os.Getenv("JIRA_PROJECT_KEY")

	if baseURL == "" || username == "" || apiToken == "" {
		log.Fatal("Missing required environment variables: JIRA_URL, JIRA_USERNAME, JIRA_API_TOKEN")
	}

	client := jira.NewClient(baseURL, username, apiToken)
	ctx := context.Background()

	// Authenticate
	if err := client.Authenticate(ctx); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	if projectKey == "" {
		// Try to fetch via API manually since client doesn't export generic GetProjects
		pk, err := getFirstProjectKey(baseURL, username, apiToken)
		if err != nil {
			log.Fatalf("Failed to fetch project key: %v", err)
		}
		projectKey = pk
	}

	fmt.Printf("Using Project Key: %s\n", projectKey)
	fmt.Println("Authenticated with Jira.")

	uniqueID := time.Now().Format("20060102-150405")
	label := fmt.Sprintf("e2e-test-%s", uniqueID)
	repoURL := "https://github.com/process-failed-successfully/recac-jira-e2e"

	// Create Epic
	epicSummary := fmt.Sprintf("[E2E %s] Golang HTTP Proxy Implementation", uniqueID)
	epicDesc := fmt.Sprintf("Epic for the complete implementation of the Golang HTTP Proxy.\n\nRepo: %s", repoURL)
	epicKey, err := client.CreateTicket(ctx, projectKey, epicSummary, epicDesc, "Epic")
	if err != nil {
		// Fallback to "Task" if "Epic" type doesn't exist or is different name
		log.Printf("Failed to create Epic (maybe type name differs?), trying 'Task' as placeholder parent: %v", err)
		epicKey, err = client.CreateTicket(ctx, projectKey, epicSummary, epicDesc, "Task")
		if err != nil {
			log.Fatalf("Failed to create Epic/Parent ticket: %v", err)
		}
	}
	fmt.Printf("Created Epic: %s\n", epicKey)
	addLabel(client, ctx, epicKey, label)

	// Define Tickets
	tickets := []TicketSpec{
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

	// Double check we have about 20
	fmt.Printf("Generating %d tickets under Epic %s...\n", len(tickets), epicKey)

	// Map Internal ID -> Jira Key
	idToKey := make(map[string]string)

	// Create All Tickets
	for i := range tickets {
		t := &tickets[i] // pointer to update JiraKey
		fmt.Printf("Creating %s: %s\n", t.ID, t.Summary)
		key, err := client.CreateTicket(ctx, projectKey, t.Summary, t.Desc, t.Type)
		if err != nil {
			log.Fatalf("Failed to create ticket %s: %v", t.ID, err)
		}
		t.JiraKey = key
		idToKey[t.ID] = key
		fmt.Printf(" -> Created %s (%s)\n", t.ID, key)

		addLabel(client, ctx, key, label)

		// Link to Epic (Using "parent" field usually works for Next-Gen, or custom field for classic)
		// For robustness, we try `client.LinkToEpic` if we had one, or a generic field update.
		// Let's implement a simple `setParent` helper locally.
		if err := setParent(client, ctx, key, epicKey); err != nil {
			log.Printf("Warning: Failed to set parent epic for %s: %v", key, err)
		} else {
			fmt.Printf(" -> Linked %s to Epic %s\n", key, epicKey)
		}
	}

	// Link Dependencies
	fmt.Println("\nLinking Dependencies...")
	for _, t := range tickets {
		if len(t.Blockers) == 0 {
			continue
		}
		for _, blockerID := range t.Blockers {
			blockerKey, ok := idToKey[blockerID]
			if !ok {
				log.Printf("Warning: Blocker ID %s not found for %s", blockerID, t.ID)
				continue
			}

			// BlockerKey BLOCKS t.JiraKey
			if err := linkIssues(client, ctx, blockerKey, t.JiraKey, "Blocks"); err != nil {
				log.Printf("Failed to link %s blocks %s: %v", blockerKey, t.JiraKey, err)
			} else {
				fmt.Printf("Linked %s (%s) blocks %s (%s)\n", blockerID, blockerKey, t.ID, t.JiraKey)
			}
		}
	}

	fmt.Printf("\nDone! Use label: %s\n", label)
}

func setParent(c *jira.Client, ctx context.Context, issueKey, parentKey string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, issueKey)

	// Start with "parent" field (standard for subtasks and next-gen epics)
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"parent": map[string]interface{}{
				"key": parentKey,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		// If "parent" fails, it might be a Classic project using "Epic Link" custom field.
		// Finding the custom field ID is hard dynamically without querying /field.
		// For E2E we might assume NextGen or standard.
		return fmt.Errorf("failed to set parent (status %d)", resp.StatusCode)
	}
	return nil
}
func addLabel(c *jira.Client, ctx context.Context, key, label string) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, key)
	payload := map[string]interface{}{
		"update": map[string]interface{}{
			"labels": []map[string]interface{}{
				{"add": label},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal label payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Failed to create label request: %v", err)
		return
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		log.Printf("Failed to execute label request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		log.Printf("Failed to add label with status: %d", resp.StatusCode)
	} else {
		fmt.Printf("Label '%s' added to %s\n", label, key)
	}
}

func linkIssues(c *jira.Client, ctx context.Context, from, to, linkType string) error {
	url := fmt.Sprintf("%s/rest/api/3/issueLink", c.BaseURL)
	payload := map[string]interface{}{
		"type": map[string]string{
			"name": linkType,
		},
		"inwardIssue": map[string]interface{}{
			"key": to,
		},
		"outwardIssue": map[string]interface{}{
			"key": from,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal link payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create link request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute link request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return fmt.Errorf("failed to link issues with status: %d, body: %s", resp.StatusCode, buf.String())
	}

	return nil
}

func getFirstProjectKey(baseURL, username, apiToken string) (string, error) {
	url := fmt.Sprintf("%s/rest/api/3/project", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(username, apiToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to list projects: status %d", resp.StatusCode)
	}

	var projects []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return "", err
	}

	if len(projects) == 0 {
		return "", fmt.Errorf("no projects found")
	}

	if key, ok := projects[0]["key"].(string); ok {
		return key, nil
	}

	return "", fmt.Errorf("invalid project response format")
}
