package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"recac/internal/jira"
	"recac/pkg/e2e/manager"
	"recac/pkg/e2e/scenarios"

	"github.com/joho/godotenv"
)

var (
	defaultRepo = "192.168.0.55:5000/recac-e2e"
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
	flag.StringVar(&model, "model", "meta-llama/llama-3.3-70b-instruct:free", "AI Model")
	flag.StringVar(&deployRepo, "repo", defaultRepo, "Docker repository for deployment")
	flag.StringVar(&targetRepo, "repo-url", repoURL, "Target Git repository for the agent")
	flag.StringVar(&pullPolicy, "pull-policy", "IfNotPresent", "Image pull policy (Always, IfNotPresent, Never)")
	flag.StringVar(&exactImage, "image", "", "Exact image name to use (overrides hash-based logic)")
	flag.BoolVar(&skipBuild, "skip-build", false, "Force skip docker build")
	flag.BoolVar(&skipCleanup, "skip-cleanup", false, "Skip cleanup on finish")
	local := flag.Bool("local", false, "Run orchestrator locally instead of deploying to K8s")
	flag.Parse()

	// Use targetRepo instead of hardcoded repoURL
	if targetRepo != "" {
		repoURL = targetRepo
	}

	// Validate Env
	required := []string{"JIRA_URL", "JIRA_USERNAME", "JIRA_API_TOKEN", "GITHUB_API_KEY"}
	for _, env := range required {
		if os.Getenv(env) == "" {
			return fmt.Errorf("missing required env var: %s", env)
		}
	}

	// Provider specific validation
	switch provider {
	case "openrouter":
		if os.Getenv("OPENROUTER_API_KEY") == "" {
			return fmt.Errorf("missing OPENROUTER_API_KEY for provider openrouter")
		}
	case "gemini", "gemini-cli":
		if os.Getenv("GEMINI_API_KEY") == "" {
			return fmt.Errorf("missing GEMINI_API_KEY for provider %s", provider)
		}
	case "anthropic":
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			return fmt.Errorf("missing ANTHROPIC_API_KEY for provider anthropic")
		}
	case "openai":
		if os.Getenv("OPENAI_API_KEY") == "" {
			return fmt.Errorf("missing OPENAI_API_KEY for provider openai")
		}
	case "cursor":
		if os.Getenv("CURSOR_API_KEY") == "" {
			return fmt.Errorf("missing CURSOR_API_KEY for provider cursor")
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
	var imageName string
	if exactImage != "" {
		imageName = exactImage
		log.Printf("Using exact image: %s", imageName)
	} else {
		// Compute Source Hash
		log.Println("Computing source hash...")
		hash, err := computeSourceHash()
		if err != nil {
			return fmt.Errorf("failed to compute source hash: %w", err)
		}
		// Shorten hash for tag
		shortHash := hash[:12]
		imageName = fmt.Sprintf("%s:%s", deployRepo, shortHash)
		log.Printf("Computed Image Tag: %s", imageName)

		// Check if image exists locally (unless skipBuild is explicitly set true, which assumes user knows)
		imageExists := false
		if !skipBuild {
			cmd := exec.Command("docker", "image", "inspect", imageName)
			if err := cmd.Run(); err == nil {
				imageExists = true
				log.Println("Image already exists locally. Skipping build.")
			}
		}

		if !skipBuild && !imageExists {
			log.Println("=== Building and Pushing Image ===")
			// Use hash as cache bypass to ensure we build for this version, but can cache intermediate layers
			// Actually, if we want Docker cache to work, we shouldn't bypass unless necessary.
			// But sticking to the previous pattern of passing ARGS is fine, just use the hash.
			if err := runCommand("make", "image-prod", fmt.Sprintf("DEPLOY_IMAGE=%s", imageName), fmt.Sprintf("ARGS=--build-arg CACHE_BYPASS=%s", shortHash)); err != nil {
				return fmt.Errorf("failed to build image: %w", err)
			}
			if err := runCommand("docker", "push", imageName); err != nil {
				return fmt.Errorf("failed to push image: %w", err)
			}
		} else if skipBuild {
			log.Println("=== Skipping Build (Explicit Flag) ===")
		} else {
			// Image exists
			log.Println("=== Pushing Existing Image (Ensure Registry has it) ===")
			if err := runCommand("docker", "push", imageName); err != nil {
				return fmt.Errorf("failed to push image: %w", err)
			}
		}
	}

	// 1b. Build recac CLI (Before generating scenario which might use it)
	fmt.Println("=== Building recac CLI ===")
	buildCmd := exec.Command("go", "build", "-o", "recac", "./cmd/recac")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build recac CLI: %v\nOutput: %s", err, out)
	}

	// 2. Setup Jira
	log.Println("=== Setting up Jira Scenario ===")
	if _, ok := scenarios.Registry[scenarioName]; !ok {
		return fmt.Errorf("unknown scenario: %s", scenarioName)
	}

	label, ticketMap, err := mgr.GenerateScenario(ctx, scenarioName, repoURL, provider, model)
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

	// 3. Prepare Repository
	log.Println("=== Preparing Repository (Cleaning stale branches) ===")
	if err := prepareRepo(repoURL, ticketMap); err != nil {
		log.Printf("Warning: Failed to prepare repository: %v", err)
	}

	// 4. Deploy (Helm or Local)
	if *local {
		log.Println("=== Running Orchestrator LOCALLY ===")

		// Run orchestrator
		cmd := exec.Command("go", "run", "./cmd/orchestrator",
			"--mode=local",
			"--poller=jira",
			fmt.Sprintf("--jira-label=%s", label),
			"--image=recac-build", // Use local image
			fmt.Sprintf("--agent-provider=%s", provider),
			fmt.Sprintf("--agent-model=%s", model),
			"--verbose",
			"--interval=10s",
		)
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		log.Printf("Starting orchestrator: %v", cmd.Args)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start local orchestrator: %w", err)
		}

		defer func() {
			if cmd.Process != nil {
				log.Println("Stopping local orchestrator...")
				_ = cmd.Process.Kill()
			}
		}()

		// Wait loop for local execution
		log.Println("=== Waiting for Execution (Local) ===")
		timeout := 600 * time.Second
		start := time.Now()

		for {
			if time.Since(start) > timeout {
				return fmt.Errorf("timeout waiting for local execution verification")
			}

			// Try verify
			log.Println("Attempting verification...")
			if err := verifyScenario(scenarioName, repoURL, ticketMap); err == nil {
				log.Println("Verification PASSED!")
				break
			} else {
				log.Printf("Verification pending/failed: %v", err)
			}

			// Check if orchestrator died
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				return fmt.Errorf("orchestrator process exited unexpectedly")
			}

			time.Sleep(10 * time.Second)
		}

		log.Println("=== E2E Test PASSED (Local) ===")
		return nil

	} else {
		log.Println("=== Scaling down old Orchestrator to prevent race conditions ===")
		// Ignore error if deployment doesn't exist
		_ = runCommand("kubectl", "scale", "deployment", releaseName, "--replicas=0", "-n", namespace)
		
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
			"--set", fmt.Sprintf("config.maxIterations=%s", getEnvOrDefault("MAX_ITERATIONS", "20")),
			"--set", fmt.Sprintf("config.provider=%s", provider),
			"--set", fmt.Sprintf("config.model=%s", model),
			"--set", "config.dbType=postgres",
			"--set", "postgresql.enabled=true",
			"--set", "postgresql.image.repository=bitnami/postgresql",
			"--set", "postgresql.image.tag=latest",
			"--set", fmt.Sprintf("config.jiraUrl=%s", os.Getenv("JIRA_URL")),
			"--set", fmt.Sprintf("config.jiraUsername=%s", os.Getenv("JIRA_USERNAME")),
			"--set", fmt.Sprintf("secrets.openrouterApiKey=%s", os.Getenv("OPENROUTER_API_KEY")),
			"--set", fmt.Sprintf("secrets.geminiApiKey=%s", os.Getenv("GEMINI_API_KEY")),
			"--set", fmt.Sprintf("secrets.anthropicApiKey=%s", os.Getenv("ANTHROPIC_API_KEY")),
			"--set", fmt.Sprintf("secrets.openaiApiKey=%s", os.Getenv("OPENAI_API_KEY")),
			"--set", fmt.Sprintf("secrets.cursorApiKey=%s", os.Getenv("CURSOR_API_KEY")),
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
	}

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

	// Determine expected job name from ticket map.
	// We iterate over all tickets and see if ANY match a running job.
	// This handles cases where we have multiple tickets or IDs change.
	var foundJobName string

	log.Println("Scanning for active jobs corresponding to generated tickets...")

	// Retry loop for finding ANY valid job
	timeout := 300 * time.Second
	start := time.Now()

	for time.Since(start) < timeout {
		for _, ticketID := range ticketMap {
			prefix := fmt.Sprintf("recac-agent-%s", strings.ToLower(ticketID))
			// We use a short timeout here since we are polling in the outer loop
			// But waitForJob is designed to wait.
			// Let's refactor waitForJob or just use checkJobExists.

			// We'll reuse waitForJob with a short timeout to check if this specific job exists yet.
			// If it returns a name, we found it.
			name, err := waitForJob(namespace, prefix, 2*time.Second)
			if err == nil && name != "" {
				foundJobName = name
				log.Printf("Found job for ticket %s: %s", ticketID, foundJobName)
				break
			}
		}

		if foundJobName != "" {
			break
		}

		time.Sleep(5 * time.Second)
		log.Printf("Waiting for any known job to appear... (scanned %d tickets)", len(ticketMap))
	}

	if foundJobName == "" {
		err = fmt.Errorf("timeout waiting for any agent job to start (checked %d tickets)", len(ticketMap))
	}

	jobName := foundJobName
	if err != nil {
		printKubeDebugInfo(namespace)
		printLogs(namespace, fmt.Sprintf("app.kubernetes.io/name=%s", "recac"))
		return fmt.Errorf("agent job failed to start: %w", err)
	}
	log.Printf("Agent job started: %s", jobName)

	// Wait for Job Completion
	log.Println("Waiting for Agent Job to complete...")
	cleanJobName := strings.TrimPrefix(jobName, "job.batch/")
	if err := waitForJobCompletion(namespace, jobName, 2400*time.Second); err != nil {
		printKubeDebugInfo(namespace)
		// Use specific job-name selector to avoid confusion with other pods
		printLogs(namespace, fmt.Sprintf("job-name=%s", cleanJobName))
		return fmt.Errorf("agent job failed to complete: %w", err)
	}

	// Print logs for debugging (especially for git push issues)
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

func computeSourceHash() (string, error) {
	hasher := sha256.New()
	dirs := []string{"cmd", "internal", "pkg"}
	files := []string{"go.mod", "go.sum", "Dockerfile"}

	// Gather all files
	var allFiles []string

	// Add root files
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			allFiles = append(allFiles, f)
		}
	}

	// Add dirs recursively
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			continue
		}
		err := filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				allFiles = append(allFiles, path)
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}

	// Sort to ensure determinism
	sort.Strings(allFiles)

	for _, file := range allFiles {
		f, err := os.Open(file)
		if err != nil {
			return "", err
		}

		// Hash the file path first to detect renames/moves
		hasher.Write([]byte(file))

		if _, err := io.Copy(hasher, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
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
		// Filter for jobs that are not Failed. We look for jobs where Succeeded >= 0 or Active >= 0
		// But simpler to just get the job and check its status json.
		// However, kubectl get jobs -o name returns all.
		// Let's filter by checking if the job exists first.
		cmd := exec.Command("kubectl", "get", "jobs", "-n", ns, "-o", "jsonpath={.items[*].metadata.name}")
		out, err := cmd.Output()
		if err == nil {
			jobNames := strings.Fields(string(out))
			for _, name := range jobNames {
				if strings.Contains(name, namePrefix) {
					// Check if job is not failed (failed jobs might be from previous runs if cleanup failed, or immediate failures)
					// Actually, in this CI run, we see BackOff, which means Pod failed but Job is still active/retrying.
					// So finding the job name is correct. The timeout happens because it never Completes.
					// But wait, the error is "timeout waiting for job recac-agent-mflp-7887"
					// This error comes from waitForJob!
					// Meaning it couldn't FIND the job name in the list within 300s?
					// Or did it find it but waitForJobCompletion failed?
					// The logs say: "E2E Test Failed: agent job failed to start: timeout waiting for job recac-agent-mflp-7887"
					// This message comes from main() calling waitForJob.
					// So waitForJob returned error.
					// This means `kubectl get jobs` did NOT return the job name.
					// Why?
					// The logs show: "Spawning K8s Job ... item=MFLP-7888"
					// But the error says "timeout waiting for job recac-agent-mflp-7887"
					// 7888 vs 7887.
					// Mismatch in ID!
					// The ticketMap might be returning an old or different ID than what was spawned?
					// In the logs: "Spawning agent for item id=MFLP-7888" multiple times.
					// Then "Deleted MFLP-7889", "Deleted MFLP-7888".
					// Then "E2E Test Failed ... timeout waiting for job recac-agent-mflp-7887".
					// It seems we are looking for 7887 but spawned 7888?
					return name, nil
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
	repoURL = strings.TrimSuffix(repoURL, "/")
	authRepo := repoURL
	if token != "" && !strings.Contains(repoURL, "@") {
		authRepo = strings.Replace(repoURL, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
	}

	// 1. Get all remote branches using ls-remote (fast, no clone needed)
	log.Printf("Checking remote branches for %s...", repoURL)
	cmd := exec.Command("git", "ls-remote", "--heads", authRepo)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to ls-remote: %w\nOutput: %s", err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	var branchesToDelete []string
	for _, line := range lines {
		// line format: <sha>\trefs/heads/<branch>
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ref := parts[1]
		if strings.HasPrefix(ref, "refs/heads/agent/") {
			branch := strings.TrimPrefix(ref, "refs/heads/")
			branchesToDelete = append(branchesToDelete, branch)
		}
	}

	if len(branchesToDelete) == 0 {
		return nil
	}

	// 2. Delete the branches
	// Since we don't have a local repo yet, we can use a dummy one or just 'git push <url> --delete <branch>'
	log.Printf("Found %d stale agent branches to delete", len(branchesToDelete))
	for _, branch := range branchesToDelete {
		log.Printf("Deleting remote branch: %s", branch)
		// We can run git push without a local repo by specifying the URL
		delCmd := exec.Command("git", "push", authRepo, "--delete", branch)
		if out, err := delCmd.CombinedOutput(); err != nil {
			log.Printf("Warning: Failed to delete branch %s: %v\nOutput: %s", branch, err, string(out))
		}
	}

	return nil
}
func getEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
