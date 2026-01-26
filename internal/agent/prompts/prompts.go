package prompts

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed templates/*.md
var templateFS embed.FS

// List of available prompt templates
const (
	Planner       = "planner"
	ManagerReview = "manager_review"
	CodingAgent   = "coding_agent"
	Initializer   = "initializer"
	QAAgent       = "qa_agent"
	TPMAgent      = "tpm_agent"
	ArchitectAgent = "architect_agent"
)

// GetPrompt loads a template and injects variables.
// It checks RECAC_PROMPTS_DIR first for overrides.
func GetPrompt(name string, vars map[string]string) (string, error) {
	var content []byte
	var err error

	// 1. Check override directory
	overrideDir := os.Getenv("RECAC_PROMPTS_DIR")
	if overrideDir != "" {
		localPath := filepath.Join(overrideDir, name+".md")
		if c, e := os.ReadFile(localPath); e == nil {
			content = c
		}
	}

	// 2. Fallback to embedded
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

// ListPrompts returns the names of all available embedded prompt templates.
func ListPrompts() ([]string, error) {
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Strings(names)
	return names, nil
}
