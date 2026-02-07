package prompts

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*.md
var templateFS embed.FS

// List of available prompt templates
const (
	Planner        = "planner"
	ManagerReview  = "manager_review"
	CodingAgent    = "coding_agent"
	Initializer    = "initializer"
	QAAgent        = "qa_agent"
	TPMAgent       = "tpm_agent"
	ArchitectAgent = "architect_agent"
)

// ListPrompts returns a list of available embedded prompts.
func ListPrompts() ([]string, error) {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
		}
	}
	return names, nil
}

// GetPrompt loads a template and injects variables.
// It checks in this order:
// 1. RECAC_PROMPTS_DIR (Env)
// 2. .recac/prompts (Local)
// 3. ~/.recac/prompts (Global)
// 4. Embedded (Fallback)
func GetPrompt(name string, vars map[string]string) (string, error) {
	var content []byte
	var err error

	// 1. Check override directory (Env)
	if overrideDir := os.Getenv("RECAC_PROMPTS_DIR"); overrideDir != "" {
		localPath := filepath.Join(overrideDir, name+".md")
		if c, e := os.ReadFile(localPath); e == nil {
			content = c
		}
	}

	// 2. Check Local .recac/prompts
	if len(content) == 0 {
		cwd, err := os.Getwd()
		if err == nil {
			localPath := filepath.Join(cwd, ".recac", "prompts", name+".md")
			if c, e := os.ReadFile(localPath); e == nil {
				content = c
			}
		}
	}

	// 3. Check Global ~/.recac/prompts
	if len(content) == 0 {
		home, err := os.UserHomeDir()
		if err == nil {
			globalPath := filepath.Join(home, ".recac", "prompts", name+".md")
			if c, e := os.ReadFile(globalPath); e == nil {
				content = c
			}
		}
	}

	// 4. Fallback to embedded
	if len(content) == 0 {
		templatePath := filepath.Join("templates", name+".md")
		content, err = templateFS.ReadFile(templatePath)
		if err != nil {
			return "", fmt.Errorf("failed to read prompt template %s: %w", name, err)
		}
	}

	prompt := string(content)
	for k, v := range vars {
		placeholder := fmt.Sprintf("{%s}", k)
		prompt = strings.ReplaceAll(prompt, placeholder, v)
	}

	return prompt, nil
}
