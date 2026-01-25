package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cheatNoOnline bool
	cheatSuggest  bool
)

// URL for cheat.sh, can be overridden in tests
var cheatShURL = "https://cht.sh"

// HTTP client with timeout, can be overridden/configured
var cheatHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

var cheatCmd = &cobra.Command{
	Use:   "cheat [topic]",
	Short: "Get instant coding cheat sheets and examples",
	Long: `Provides quick, copy-pasteable examples for common tools and languages.
It uses a combination of:
1. Context Analysis (what's in your directory?)
2. Embedded Cheats (common tools)
3. Online Community Cheats (cht.sh)
4. AI Generation (fallback)`,
	RunE: runCheat,
}

func init() {
	rootCmd.AddCommand(cheatCmd)
	cheatCmd.Flags().BoolVar(&cheatNoOnline, "no-online", false, "Disable online lookups (cht.sh)")
	cheatCmd.Flags().BoolVarP(&cheatSuggest, "suggest", "s", false, "Suggest commands based on current directory context")
}

// Embedded cheats for offline availability
var embeddedCheats = map[string]string{
	"git": `
# Git Cheat Sheet

git init                     # Initialize a new repository
git clone <url>              # Clone a repository
git status                   # Show the working tree status
git add .                    # Add all changes to staging
git commit -m "message"      # Commit staged changes
git push                     # Push changes to remote
git pull                     # Pull changes from remote
git branch                   # List branches
git checkout -b <branch>     # Create and switch to a new branch
git merge <branch>           # Merge a branch into the current one
git log --oneline --graph    # Show commit history
git reset --soft HEAD~1      # Undo last commit but keep changes
git stash                    # Stash changes
git stash pop                # Restore stashed changes
`,
	"docker": `
# Docker Cheat Sheet

docker build -t <name> .     # Build an image
docker run -it <image>       # Run a container interactively
docker ps                    # List running containers
docker ps -a                 # List all containers
docker stop <id>             # Stop a container
docker rm <id>               # Remove a container
docker rmi <image>           # Remove an image
docker logs <id>             # View container logs
docker exec -it <id> sh      # Shell into a container
docker system prune -a       # Clean up unused data
`,
	"kubernetes": `
# Kubernetes (kubectl) Cheat Sheet

kubectl get pods             # List pods
kubectl get services         # List services
kubectl get deployments      # List deployments
kubectl logs <pod>           # View pod logs
kubectl exec -it <pod> -- sh # Shell into a pod
kubectl apply -f <file>      # Apply a configuration
kubectl delete -f <file>     # Delete resources from file
kubectl describe pod <pod>   # Describe pod details
kubectl config current-context # Show current context
`,
	"go": `
# Go Cheat Sheet

go run .                     # Run the package in current directory
go build                     # Build the package
go test ./...                # Run all tests
go mod tidy                  # Prune/add dependencies
go get <package>             # Add a dependency
go fmt ./...                 # Format code
go vet ./...                 # Vet code
`,
	"tar": `
# Tar Cheat Sheet

tar -cvf archive.tar dir/    # Create an archive
tar -xvf archive.tar         # Extract an archive
tar -zcvf archive.tar.gz dir # Create a gzipped archive
tar -zxvf archive.tar.gz     # Extract a gzipped archive
`,
}

func runCheat(cmd *cobra.Command, args []string) error {
	// 1. Context Analysis / Suggestions
	// If no args provided, or --suggest flag is set, run context analysis
	if len(args) == 0 || cheatSuggest {
		if err := showContextSuggestions(cmd); err != nil {
			return err
		}
		// If no topic provided, we are done after suggestions
		if len(args) == 0 {
			return nil
		}
	}

	topic := strings.ToLower(args[0])

	// 2. Embedded Cheats (Offline)
	if content, ok := embeddedCheats[topic]; ok {
		fmt.Fprintf(cmd.OutOrStdout(), "\n📘 Embedded Cheat Sheet for '%s':\n%s\n", topic, content)
		return nil
	}

	// 3. Online Lookup (cht.sh)
	if !cheatNoOnline {
		fmt.Fprintf(cmd.OutOrStdout(), "🌍 Searching cht.sh for '%s'...\n", topic)
		content, err := fetchCheatSh(topic)
		if err == nil && content != "" {
			// cht.sh sometimes returns "Unknown topic" or similar text but 200 OK.
			// A valid response usually starts with comments or commands.
			// Simple heuristic: if it contains "Unknown topic", treat as fail.
			if !strings.Contains(content, "Unknown topic") && !strings.Contains(content, "404 Not Found") {
				fmt.Fprintln(cmd.OutOrStdout(), content)
				return nil
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "   -> Not found or unreachable.\n")
		}
	}

	// 4. AI Fallback
	return generateAICheatSheet(cmd, topic)
}

func showContextSuggestions(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Context Suggestions:")
	found := false

	// Check for Makefile
	if _, err := os.Stat(filepath.Join(cwd, "Makefile")); err == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "\n  Found Makefile:")
		fmt.Fprintln(cmd.OutOrStdout(), "    make build")
		fmt.Fprintln(cmd.OutOrStdout(), "    make test")
		found = true
	}

	// Check for Dockerfile
	if _, err := os.Stat(filepath.Join(cwd, "Dockerfile")); err == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "\n  Found Dockerfile:")
		fmt.Fprintln(cmd.OutOrStdout(), "    docker build -t myapp .")
		fmt.Fprintln(cmd.OutOrStdout(), "    docker run -it myapp")
		found = true
	}

	// Check for Go
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "\n  Found Go Project:")
		fmt.Fprintln(cmd.OutOrStdout(), "    go run .")
		fmt.Fprintln(cmd.OutOrStdout(), "    go test ./...")
		fmt.Fprintln(cmd.OutOrStdout(), "    go mod tidy")
		found = true
	}

	// Check for Node/NPM
	if _, err := os.Stat(filepath.Join(cwd, "package.json")); err == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "\n  Found Node/NPM Project:")
		fmt.Fprintln(cmd.OutOrStdout(), "    npm install")
		fmt.Fprintln(cmd.OutOrStdout(), "    npm start")
		fmt.Fprintln(cmd.OutOrStdout(), "    npm test")
		found = true
	}

	// Check for Python
	if _, err := os.Stat(filepath.Join(cwd, "requirements.txt")); err == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "\n  Found Python Project:")
		fmt.Fprintln(cmd.OutOrStdout(), "    pip install -r requirements.txt")
		found = true
	}

	if !found {
		fmt.Fprintln(cmd.OutOrStdout(), "  (No specific project markers found)")
	}
	fmt.Fprintln(cmd.OutOrStdout(), "") // Newline
	return nil
}

func fetchCheatSh(topic string) (string, error) {
	// cht.sh supports ?T for raw text
	url := fmt.Sprintf("%s/%s?T", cheatShURL, topic)
	resp, err := cheatHTTPClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func generateAICheatSheet(cmd *cobra.Command, topic string) error {
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-cheat")
	if err != nil {
		return fmt.Errorf("failed to create agent for fallback: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🧠 Asking AI to generate a cheat sheet for '%s'...\n", topic)

	prompt := fmt.Sprintf(`Generate a concise cheat sheet for '%s'.
List the most common commands and short explanations.
Format as Markdown.
Be brief and practical.`, topic)

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Fprintln(cmd.OutOrStdout(), "")

	return err
}
