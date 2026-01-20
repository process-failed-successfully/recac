package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"recac/pkg/e2e/state"

	"github.com/joho/godotenv"
)

var (
	chartPath   = "./deploy/helm/recac"
	namespace   = "default"
	releaseName = "recac"
)

func RunDeploy(args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	var (
		stateFile  string
		deployRepo string
		pullPolicy string
		skipBuild  bool
		wait       bool
	)
	fs.StringVar(&stateFile, "state-file", "e2e_state.json", "Path to state file")
	fs.StringVar(&deployRepo, "repo", defaultRepo, "Docker repository for deployment")
	fs.StringVar(&pullPolicy, "pull-policy", "IfNotPresent", "Image pull policy")
	fs.BoolVar(&skipBuild, "skip-build", false, "Skip docker build")
	fs.BoolVar(&wait, "wait", true, "Wait for deployment to be ready")
	fs.Parse(args)

	_ = godotenv.Load()

	e2eCtx, err := state.Load(stateFile)
	if err != nil {
		return fmt.Errorf("failed to load state file: %w", err)
	}

	// 1. Build and Push
	imageName, err := buildAndPush(deployRepo, skipBuild)
	if err != nil {
		return err
	}

	// 2. Deploy Helm
	log.Println("=== Scaling down old Orchestrator to prevent race conditions ===")
	_ = runCommand("kubectl", "scale", "deployment", releaseName, "--replicas=0", "-n", namespace)
	
	log.Println("=== Cleaning up old Jobs ===")
	_ = runCommand("kubectl", "delete", "jobs", "-n", namespace, "-l", "app=recac-agent", "--cascade=foreground", "--wait=true")

	// Wipe Database if postgres is used
	// (Simplification: assuming default setup for now, copying runner logic)
	log.Println("=== Wiping PostgreSQL Database ===")
	wipeCmd := "PGPASSWORD=changeit psql -U recac -d recac -c \"TRUNCATE observations, signals, project_features, file_locks;\""
	_ = runCommand("sh", "-c", fmt.Sprintf("POD_NAME=$(kubectl get pods -l app.kubernetes.io/instance=recac,app.kubernetes.io/name=postgresql -n default -o jsonpath='{.items[0].metadata.name}' 2>/dev/null) && [ -n \"$POD_NAME\" ] && kubectl exec $POD_NAME -n default -- sh -c %s", fmt.Sprintf("'%s'", wipeCmd)))

	log.Println("=== Deploying Helm Chart ===")
	lastColon := strings.LastIndex(imageName, ":")
	repoPart := imageName[:lastColon]
	tagPart := imageName[lastColon+1:]

	// Workaround: If pushing to 192.168.0.55 (plex-desktop), pull from localhost:5000
	pullRepo := repoPart
	if strings.HasPrefix(repoPart, "192.168.0.55:5000") {
		pullRepo = strings.Replace(repoPart, "192.168.0.55:5000", "localhost:5000", 1)
		log.Printf("Detected local registry. Using %s for pull.", pullRepo)
	}

	helmLargestCmd := []string{
		"upgrade", "--install", releaseName,
		chartPath,
		"--namespace", namespace,
		"--set", fmt.Sprintf("image.repository=%s", pullRepo),
		"--set", fmt.Sprintf("image.tag=%s", tagPart),
		"--set", fmt.Sprintf("image.pullPolicy=%s", pullPolicy),
		"--set", "config.imagePullPolicy=IfNotPresent",
		"--set", "config.poller=jira",
		"--set", fmt.Sprintf("config.jira_label=%s", e2eCtx.JiraLabel),
		"--set", fmt.Sprintf("config.jira_query=labels = \"%s\" AND issuetype != Epic AND statusCategory != Done ORDER BY created ASC", e2eCtx.JiraLabel),
		"--set", "config.verbose=true",
		"--set", "config.interval=10s",
		"--set", fmt.Sprintf("config.maxIterations=%s", GetEnvOrDefault("MAX_ITERATIONS", "60")),
		"--set", fmt.Sprintf("config.provider=%s", e2eCtx.Provider),
		"--set", fmt.Sprintf("config.model=%s", e2eCtx.Model),
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

	if strings.HasPrefix(pullRepo, "localhost:5000") {
		helmLargestCmd = append(helmLargestCmd, "--set", "postgresql.global.imageRegistry=localhost:5000")
	}

	if err := runCommand("helm", helmLargestCmd...); err != nil {
		return fmt.Errorf("helm deploy failed: %w", err)
	}

	// Update State
	e2eCtx.Namespace = namespace
	e2eCtx.ReleaseName = releaseName
	if err := e2eCtx.Save(stateFile); err != nil {
		return fmt.Errorf("failed to update state file: %w", err)
	}

	if wait {
		// We could call a 'Wait' function here, but for now just basic rollout status
		return runCommand("kubectl", "rollout", "status", "deployment/"+releaseName, "-n", namespace)
	}

	return nil
}

func buildAndPush(deployRepo string, skipBuild bool) (string, error) {
	log.Println("Computing source hash...")
	hash, err := ComputeSourceHash()
	if err != nil {
		return "", fmt.Errorf("failed to compute source hash: %w", err)
	}
	shortHash := hash[:12]
	imageName := fmt.Sprintf("%s:%s", deployRepo, shortHash)
	log.Printf("Computed Image Tag: %s", imageName)

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
		if err := runCommand("make", "image-prod", fmt.Sprintf("DEPLOY_IMAGE=%s", imageName), fmt.Sprintf("ARGS=--build-arg CACHE_BYPASS=%s", shortHash)); err != nil {
			return "", fmt.Errorf("failed to build image: %w", err)
		}
		if err := runCommand("docker", "push", imageName); err != nil {
			return "", fmt.Errorf("failed to push image: %w", err)
		}
	} else if skipBuild {
		log.Println("=== Skipping Build (Explicit Flag) ===")
	} else {
		log.Println("=== Pushing Existing Image (Ensure Registry has it) ===")
		if err := runCommand("docker", "push", imageName); err != nil {
			return "", fmt.Errorf("failed to push image: %w", err)
		}
	}

	// Also build CLI
	fmt.Println("=== Building recac CLI ===")
	buildCmd := exec.Command("go", "build", "-o", "recac", "./cmd/recac")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to build recac CLI: %v\nOutput: %s", err, out)
	}

	return imageName, nil
}
