package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cheatHTTPClient = &http.Client{Timeout: 10 * time.Second}
	cheatShURL      = "https://cht.sh"
)

// Static cheatsheets for offline use or quick access
var staticCheatsheets = map[string]string{
	"tar": `
# Create a gzip compressed archive
tar -czvf archive.tar.gz /path/to/directory

# Extract a gzip compressed archive
tar -xzvf archive.tar.gz

# List contents
tar -tvf archive.tar.gz
`,
	"git": `
# Undo last commit but keep changes
git reset --soft HEAD~1

# Discard local changes to a file
git checkout -- <file>

# Show log with graph
git log --oneline --graph --all --decorate
`,
	"docker": `
# Remove stopped containers
docker container prune

# Remove unused images
docker image prune -a

# Build image
docker build -t name:tag .
`,
}

var cheatCmd = &cobra.Command{
	Use:   "cheat [topic]",
	Short: "Get a cheatsheet for a command or language",
	Long: `Retrieves cheatsheets from static list, cht.sh, or generates one using AI.
If no topic is provided, it attempts to guess the topic based on the current directory context.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCheat,
}

func init() {
	rootCmd.AddCommand(cheatCmd)
}

func runCheat(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	var topic string

	if len(args) > 0 {
		topic = args[0]
	} else {
		// Auto-detect context
		detected, err := detectContext()
		if err != nil {
			return fmt.Errorf("failed to detect context: %w", err)
		}
		if detected == "" {
			return fmt.Errorf("no topic provided and could not detect context from files")
		}
		topic = detected
		fmt.Fprintf(cmd.OutOrStdout(), "🔍 Detected context: %s\n", topic)
	}

	// 1. Check static cheatsheets
	if content, ok := staticCheatsheets[topic]; ok {
		fmt.Fprintln(cmd.OutOrStdout(), content)
		return nil
	}

	// 2. Check cht.sh
	fmt.Fprintf(cmd.OutOrStdout(), "🌐 Fetching from cht.sh/%s...\n", topic)
	content, err := fetchCheatSheet(topic)
	if err == nil {
		fmt.Fprintln(cmd.OutOrStdout(), content)
		return nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  cht.sh failed: %v\n", err)

	// 3. Fallback to AI
	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Asking AI for a cheatsheet...")
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-cheat")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf("Generate a concise cheatsheet for '%s' with common commands and examples. Use code blocks.", topic)
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), resp)
	return nil
}

func detectContext() (string, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return "", err
	}

	for _, f := range files {
		name := f.Name()
		switch name {
		case "go.mod":
			return "go", nil
		case "package.json", "package-lock.json", "yarn.lock":
			return "javascript", nil
		case "requirements.txt", "setup.py", "Pipfile":
			return "python", nil
		case "Cargo.toml":
			return "rust", nil
		case "Dockerfile":
			return "docker", nil
		case "Makefile":
			return "make", nil
		case "pom.xml":
			return "maven", nil
		case "build.gradle":
			return "gradle", nil
		}
		if strings.HasSuffix(name, ".tf") {
			return "terraform", nil
		}
	}
	return "", nil
}

func fetchCheatSheet(topic string) (string, error) {
	resp, err := cheatHTTPClient.Get(fmt.Sprintf("%s/%s?T", cheatShURL, topic))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", fmt.Errorf("topic not found")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
