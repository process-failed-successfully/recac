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
	defaultRepo  = "192.168.0.55:5000/recac-e2e"
	deployTag    = "1h"
	chartPath    = "./deploy/helm/recac"
	namespace    = "default"
	releaseName  = "recac"
	repoURL      = "https://github.com/process-failed-successfully/recac-jira-e2e"
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
		skipBuild    bool
		skipCleanup  bool
	)

	flag.StringVar(&scenarioName, "scenario", "http-proxy", "Scenario to run")
	flag.StringVar(&provider, "provider", "openrouter", "AI Provider")
	flag.StringVar(&model, "model", "mistralai/devstral-2512:free", "AI Model")
	flag.StringVar(&deployRepo, "repo", defaultRepo, "Docker repository for deployment")
	flag.StringVar(&targetRepo, "repo-url", repoURL, "Target Git repository for the agent")
	flag.StringVar(&pullPolicy, "pull-policy", "Always", "Image pull policy (Always, IfNotPresent, Never)")
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
	imageName := fmt.Sprintf("%s-%d:%s", deployRepo, time.Now().Unix(), deployTag)
	if !skipBuild {
		log.Println("=== Building and Pushing Image ===")
		if err := runCommand("make", "image-prod", fmt.Sprintf("DEPLOY_IMAGE=%s", imageName)); err != nil {
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

	// 3. Deploy Helm
	log.Println("=== Cleaning up old Jobs ===")
	_ = runCommand("kubectl", "delete", "jobs", "-n", namespace, "-l", "app=recac-agent", "--cascade=foreground", "--wait=true")

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
		"--set", "config.max_iterations=20",
		"--set", fmt.Sprintf("config.provider=%s", provider),
		"--set", fmt.Sprintf("config.model=%s", model),
		"--set", fmt.Sprintf("config.jiraUrl=%s", os.Getenv("JIRA_URL")),
		"--set", fmt.Sprintf("config.jiraUsername=%s", os.Getenv("JIRA_USERNAME")),
		"--set", fmt.Sprintf("secrets.openrouterApiKey=%s", os.Getenv("OPENROUTER_API_KEY")),
		"--set", fmt.Sprintf("secrets.jiraApiToken=%s", os.Getenv("JIRA_API_TOKEN")),
		"--set", fmt.Sprintf("secrets.ghApiKey=%s", os.Getenv("GITHUB_API_KEY")),
		"--set", fmt.Sprintf("secrets.ghEmail=%s", os.Getenv("GITHUB_EMAIL")),
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
		_ = runCommand("kubectl", "get", "pods", "-n", namespace)
		_ = runCommand("kubectl", "describe", "pods", "-l", fmt.Sprintf("app.kubernetes.io/name=%s", "recac"), "-n", namespace)
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
		printLogs(namespace, fmt.Sprintf("app.kubernetes.io/name=%s", "recac"))
		return fmt.Errorf("agent job failed to start: %w", err)
	}
	log.Printf("Agent job started: %s", jobName)

	// Wait for Job Completion
	log.Println("Waiting for Agent Job to complete...")
	if err := waitForJobCompletion(namespace, jobName, 600*time.Second); err != nil {
		printLogs(namespace, "app=recac-agent")
		return fmt.Errorf("agent job failed to complete: %w", err)
	}

	// Print logs for debugging (especially for git push issues)
	cleanJobName := strings.TrimPrefix(jobName, "job.batch/")
	printLogs(namespace, fmt.Sprintf("job-name=%s", cleanJobName))

	// 5. Verify Results
	log.Println("=== Verifying Results ===")
	if err := verifyScenario(scenarioName, repoURL, ticketMap); err != nil {
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
	// Insert token into URL
	// repo is like https://github.com/org/repo
	authRepo := strings.Replace(repo, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)

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
	_ = runCommand("kubectl", "logs", "-l", selector, "-n", ns, "--all-containers=true", "--tail=50")
	fmt.Println("--- LOGS END ---")
}