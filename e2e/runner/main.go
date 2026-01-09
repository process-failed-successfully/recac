package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"recac/internal/jira"
	"recac/pkg/e2e/manager"
	"recac/pkg/e2e/scenarios"

	"github.com/joho/godotenv"
)

var (
	defaultRepo = "192.168.0.55:5000/recac-e2e"
	deployTag   = "1h"
	chartPath   = "./deploy/helm/recac"
	namespace   = "default"
	releaseName = "recac"
	repoURL     = "https://github.com/process-failed-successfully/recac-jira-e2e"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("E2E Test Failed: %v", err)
	}
}

func run() error {
	_ = godotenv.Load()

	var (
		scenarioName string
		provider     string
		model        string
		deployRepo   string
		targetRepo   string
		pullPolicy   string
		exactImage   string
		skipBuild    bool
		skipCleanup  bool
	)

	flag.StringVar(&scenarioName, "scenario", "http-proxy", "Scenario to run")
	flag.StringVar(&provider, "provider", "openrouter", "AI Provider")
	flag.StringVar(&model, "model", "mistralai/devstral-2512:free", "AI Model")
	flag.StringVar(&deployRepo, "repo", defaultRepo, "Docker repository for deployment")
	flag.StringVar(&targetRepo, "repo-url", repoURL, "Target Git repository for the agent")
	flag.StringVar(&pullPolicy, "pull-policy", "IfNotPresent", "Image pull policy (Always, IfNotPresent, Never)")
	flag.StringVar(&exactImage, "image", "", "Exact image name to use (overrides repo/timestamp logic)")
	flag.BoolVar(&skipBuild, "skip-build", false, "Skip docker build")
	flag.BoolVar(&skipCleanup, "skip-cleanup", false, "Skip cleanup on finish")
	flag.Parse()

	// Use targetRepo instead of hardcoded repoURL
	if targetRepo != "" {
		repoURL = targetRepo
	}

	// Validate Env
	required := []string{"JIRA_URL", "JIRA_USERNAME", "JIRA_API_TOKEN", "GITHUB_API_KEY", "OPENROUTER_API_KEY"}
	for _, env := range required {
		if os.Getenv(env) == "" {
			return fmt.Errorf("missing required env var: %s", env)
		}
	}
	// Note: SLACK/DISCORD tokens are optional for E2E but supported if present
	// Fallback/Default for API key if token not set
	if os.Getenv("JIRA_API_TOKEN") == "" && os.Getenv("JIRA_API_KEY") != "" {
		os.Setenv("JIRA_API_TOKEN", os.Getenv("JIRA_API_KEY"))
	}
	projectKey := os.Getenv("JIRA_PROJECT_KEY")

	ctx := context.Background()

	// Fallback for missing JIRA_PROJECT_KEY
	if projectKey == "" {
		log.Println("JIRA_PROJECT_KEY not set. Attempting to fetch default project...")
		tmpClient := jira.NewClient(os.Getenv("JIRA_URL"), os.Getenv("JIRA_USERNAME"), os.Getenv("JIRA_API_TOKEN"))
		var err error
		projectKey, err = tmpClient.GetFirstProjectKey(ctx)
		if err != nil {
			return fmt.Errorf("missing JIRA_PROJECT_KEY and failed to fetch default: %w", err)
		}
		log.Printf("Using default project key: %s", projectKey)
	}

	mgr := manager.NewJiraManager(os.Getenv("JIRA_URL"), os.Getenv("JIRA_USERNAME"), os.Getenv("JIRA_API_TOKEN"), projectKey)

	// 1. Build and Push
	var imageName string
	if exactImage != "" {
		imageName = exactImage
		log.Printf("Using exact image: %s", imageName)
	} else {
		imageName = fmt.Sprintf("%s-%d:%s", deployRepo, time.Now().Unix(), deployTag)
	}

	if !skipBuild {
		log.Println("=== Building and Pushing Image ===")
		bypass := fmt.Sprintf("CACHE_BYPASS=%d", time.Now().Unix())
		if err := runCommand("make", "image-prod", fmt.Sprintf("DEPLOY_IMAGE=%s", imageName), fmt.Sprintf("ARGS=--build-arg %s", bypass)); err != nil {
			return fmt.Errorf("failed to build image: %w", err)
		}
		if err := runCommand("docker", "push", imageName); err != nil {
			return fmt.Errorf("failed to push image: %w", err)
		}
	} else {
		log.Println("=== Skipping Build ===")
	}

	// 2. Setup Jira
	log.Println("=== Setting up Jira Scenario ===")
	if _, ok := scenarios.Registry[scenarioName]; !ok {
		return fmt.Errorf("unknown scenario: %s", scenarioName)
	}

	label, ticketMap, err := mgr.GenerateScenario(ctx, scenarioName, repoURL)
	if err != nil {
		return fmt.Errorf("failed to generate scenario: %w", err)
	}
	log.Printf("Scenario generated with label: %s", label)

	// Defer Cleanup
	defer func() {
		if !skipCleanup {
			log.Println("=== Cleaning up Jira ===")
			if err := mgr.Cleanup(context.Background(), label); err != nil {
				log.Printf("Failed cleanup: %v", err)
			}
		}
	}()

	// 3. Prepare Repository (Cleanup stale branches)
	log.Println("=== Preparing Repository (Cleaning stale branches) ===")
	if err := prepareRepo(repoURL, ticketMap); err != nil {
		log.Printf("Warning: Failed to prepare repository: %v", err)
	}

	// 4. Deploy Helm
	log.Println("=== Cleaning up old Jobs ===")
	_ = runCommand("kubectl", "delete", "jobs", "-n", namespace, "-l", "app=recac-agent", "--cascade=foreground", "--wait=true")

	// Wipe Database if postgres is used
	log.Println("=== Wiping PostgreSQL Database ===")
	// We use kubectl exec to run psql inside the postgres pod
	// We ignore errors if pod doesn't exist yet (first run)
	wipeCmd := "PGPASSWORD=changeit psql -U recac -d recac -c \"TRUNCATE observations, signals, project_features, file_locks;\""
	_ = runCommand("sh", "-c", fmt.Sprintf("POD_NAME=$(kubectl get pods -l app.kubernetes.io/instance=recac,app.kubernetes.io/name=postgresql -n default -o jsonpath='{.items[0].metadata.name}' 2>/dev/null) && [ -n \"$POD_NAME\" ] && kubectl exec $POD_NAME -n default -- sh -c %s", fmt.Sprintf("'%s'", wipeCmd)))

	log.Println("=== Deploying Helm Chart ===")
	lastColon := strings.LastIndex(imageName, ":")
	repoPart := imageName[:lastColon]
	tagPart := imageName[lastColon+1:]

	// Workaround: If pushing to 192.168.0.55 (plex-desktop), pull from localhost:5000
	// to avoid "http: server gave HTTP response to HTTPS client" errors in K3s.
	pullRepo := repoPart
	if strings.HasPrefix(repoPart, "192.168.0.55:5000") {
		pullRepo = strings.Replace(repoPart, "192.168.0.55:5000", "localhost:5000", 1)
		log.Printf("Detected local registry. Using %s for pull.", pullRepo)
	}

	helmLargestCmd := []string{
		"upgrade", "--install", releaseName, chartPath,
		"--namespace", namespace,
		"--set", fmt.Sprintf("image.repository=%s", pullRepo),
		"--set", fmt.Sprintf("image.tag=%s", tagPart),
		"--set", fmt.Sprintf("image.pullPolicy=%s", pullPolicy),
		"--set", "config.imagePullPolicy=IfNotPresent",
		"--set", "config.poller=jira",
		"--set", fmt.Sprintf("config.jira_label=%s", label),
		"--set", fmt.Sprintf("config.jira_query=labels = \"%s\" AND issuetype != Epic AND statusCategory != Done ORDER BY created ASC", label),
		"--set", "config.verbose=true",
		"--set", "config.interval=10s",
		"--set", "config.maxIterations=20",
		"--set", fmt.Sprintf("config.provider=%s", provider),
		"--set", fmt.Sprintf("config.model=%s", model),
		"--set", "config.dbType=postgres",
		"--set", "postgresql.enabled=true",
		"--set", "postgresql.image.repository=bitnami/postgresql",
		"--set", "postgresql.image.tag=latest",
		"--set", fmt.Sprintf("config.jiraUrl=%s", os.Getenv("JIRA_URL")),
		"--set", fmt.Sprintf("config.jiraUsername=%s", os.Getenv("JIRA_USERNAME")),
		"--set", fmt.Sprintf("secrets.openrouterApiKey=%s", os.Getenv("OPENROUTER_API_KEY")),
		"--set", fmt.Sprintf("secrets.jiraApiToken=%s", os.Getenv("JIRA_API_TOKEN")),
		"--set", fmt.Sprintf("secrets.ghApiKey=%s", os.Getenv("GITHUB_API_KEY")),
		"--set", fmt.Sprintf("secrets.ghEmail=%s", os.Getenv("GITHUB_EMAIL")),
		"--set", fmt.Sprintf("secrets.slackBotUserToken=%s", os.Getenv("SLACK_BOT_USER_TOKEN")),
		"--set", fmt.Sprintf("secrets.slackAppToken=%s", os.Getenv("SLACK_APP_TOKEN")),
		"--set", fmt.Sprintf("secrets.discordBotToken=%s", os.Getenv("DISCORD_BOT_TOKEN")),
		"--set", fmt.Sprintf("secrets.discordChannelId=%s", os.Getenv("DISCORD_CHANNEL_ID")),
	}

	// Conditionally set postgres registry if we are assuming a local mirroring setup
	if strings.HasPrefix(pullRepo, "localhost:5000") {
		helmLargestCmd = append(helmLargestCmd, "--set", "postgresql.global.imageRegistry=localhost:5000")
	}

	if err := runCommand("helm", helmLargestCmd...); err != nil {
		return fmt.Errorf("helm deploy failed: %w", err)
	}

	defer func() {
		if !skipCleanup {
			log.Println("=== Uninstalling Helm Release ===")
			_ = runCommand("helm", "uninstall", releaseName, "--namespace", namespace)
		}
	}()

	// 4. Wait for Execution
	log.Println("=== Waiting for Execution ===")
	// Check for Orchestrator Pod
	if err := waitForPod(namespace, fmt.Sprintf("app.kubernetes.io/name=%s", "recac"), 120*time.Second); err != nil {
		fmt.Println("!!! Orchestrator Pod Failed to Start !!!")
		printKubeDebugInfo(namespace)
		return fmt.Errorf("orchestrator pod failed to start: %w", err)
	}

	// Check for Agent Job
	log.Println("Waiting for Agent Job to start...")

	// Determine expected job name from ticket map (assuming single task for now or finding "PRIMES")
	var targetTicketID string
	if id, ok := ticketMap["PRIMES"]; ok {
		targetTicketID = id
	} else {
		// Fallback: Use the first one
		for _, id := range ticketMap {
			targetTicketID = id
			break
		}
	}

	expectedJobPrefix := fmt.Sprintf("recac-agent-%s", strings.ToLower(targetTicketID))
	log.Printf("Looking for job prefix: %s", expectedJobPrefix)

	jobName, err := waitForJob(namespace, expectedJobPrefix, 300*time.Second)
	if err != nil {
		printKubeDebugInfo(namespace)
		printLogs(namespace, fmt.Sprintf("app.kubernetes.io/name=%s", "recac"))
		return fmt.Errorf("agent job failed to start: %w", err)
	}
	log.Printf("Agent job started: %s", jobName)

	// Wait for Job Completion
	log.Println("Waiting for Agent Job to complete...")
	if err := waitForJobCompletion(namespace, jobName, 2400*time.Second); err != nil {
		printKubeDebugInfo(namespace)
		printLogs(namespace, "app=recac-agent")
		return fmt.Errorf("agent job failed to complete: %w", err)
	}

	// Print logs for debugging (especially for git push issues)
	cleanJobName := strings.TrimPrefix(jobName, "job.batch/")
	printLogs(namespace, fmt.Sprintf("job-name=%s", cleanJobName))

	// 5. Verify Results
	log.Println("=== Verifying Results ===")
	if err := verifyScenario(scenarioName, repoURL, ticketMap); err != nil {
		printKubeDebugInfo(namespace)
		return fmt.Errorf("verification failed: %w", err)
	}

	log.Println("=== E2E Test PASSED ===")
	return nil
}

func verifyScenario(scenarioName, repo string, ticketMap map[string]string) error {
	scenario, ok := scenarios.Registry[scenarioName]
	if !ok {
		return fmt.Errorf("unknown scenario: %s", scenarioName)
	}

	// Clone to temp dir
	tmpDir, err := os.MkdirTemp("", "e2e-check")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	token := os.Getenv("GITHUB_API_KEY")
	authRepo := repo
	if !strings.Contains(repo, "@") {
		// Insert token into URL
		authRepo = strings.Replace(repo, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
	}

	log.Printf("Cloning repo to %s...", tmpDir)
	if err := runCommand("git", "clone", authRepo, tmpDir); err != nil {
		return fmt.Errorf("failed to clone: %w", err)
	}

	log.Printf("Running verification for scenario: %s", scenario.Name())
	return scenario.Verify(tmpDir, ticketMap)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForPod(ns, labelSelector string, timeout time.Duration) error {
	return runCommand("kubectl", "rollout", "status", "deployment/recac", "-n", ns, "--timeout", fmt.Sprintf("%.0fs", timeout.Seconds()))
}

func waitForJob(ns, namePrefix string, timeout time.Duration) (string, error) {
	start := time.Now()
	for time.Since(start) < timeout {
		cmd := exec.Command("kubectl", "get", "jobs", "-n", ns, "-o", "name")
		out, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.Contains(line, namePrefix) {
					return strings.TrimSpace(line), nil
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return "", fmt.Errorf("timeout waiting for job %s", namePrefix)
}

func waitForJobCompletion(ns, jobName string, timeout time.Duration) error {
	return runCommand("kubectl", "wait", "--for=condition=complete", jobName, "-n", ns, "--timeout", fmt.Sprintf("%.0fs", timeout.Seconds()))
}

func printLogs(ns, selector string) {
	fmt.Println("--- LOGS START ---")
	_ = runCommand("kubectl", "logs", "-l", selector, "-n", ns, "--all-containers=true", "--tail=300")
	fmt.Println("--- LOGS END ---")
}

func printKubeDebugInfo(ns string) {
	fmt.Println("--- KUBE DEBUG INFO START ---")
	fmt.Println(">> PODS <<")
	_ = runCommand("kubectl", "get", "pods", "-n", ns, "-o", "wide")
	fmt.Println(">> DESCRIBE PODS <<")
	_ = runCommand("kubectl", "describe", "pods", "-n", ns)
	fmt.Println(">> EVENTS <<")
	_ = runCommand("kubectl", "get", "events", "-n", ns, "--sort-by=.lastTimestamp")
	fmt.Println("--- KUBE DEBUG INFO END ---")
}

func prepareRepo(repoURL string, ticketMap map[string]string) error {
	token := os.Getenv("GITHUB_API_KEY")
	authRepo := repoURL
	if !strings.Contains(repoURL, "@") {
		authRepo = strings.Replace(repoURL, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
	}

	// Clone to temp dir
	tmpDir, err := os.MkdirTemp("", "e2e-cleanup")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := runCommand("git", "clone", "--depth", "1", authRepo, tmpDir); err != nil {
		return fmt.Errorf("failed to clone for preparation: %w", err)
	}

	// Delete ALL remote agent branches to avoid context pollution
	// git branch -r --list 'origin/agent/*'
	cmd := exec.Command("git", "-C", tmpDir, "branch", "-r", "--list", "origin/agent/*")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// line is origin/agent/MFLP-123
			parts := strings.Split(line, "origin/")
			if len(parts) > 1 {
				branch := parts[1]
				log.Printf("Deleting stale remote branch: %s", branch)
				_ = exec.Command("git", "-C", tmpDir, "push", "origin", "--delete", branch).Run()
			}
		}
	}

	return nil
}
