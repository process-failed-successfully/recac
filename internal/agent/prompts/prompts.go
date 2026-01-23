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
	Planner       = "planner"
	ManagerReview = "manager_review"
	CodingAgent   = "coding_agent"
	Initializer   = "initializer"
	QAAgent       = "qa_agent"
	TPMAgent      = "tpm_agent"
	ArchitectAgent = "architect_agent"
)

// GetPrompt loads a template and injects variables.
// It prioritizes files in the directory specified by RECAC_PROMPTS_DIR if set.
func GetPrompt(name string, vars map[string]string) (string, error) {
	var content []byte
	var err error

	// 1. Check override directory
	overrideDir := os.Getenv("RECAC_PROMPTS_DIR")
	if overrideDir != "" {
		overridePath := filepath.Join(overrideDir, name+".md")
		content, err = os.ReadFile(overridePath)
		if err == nil {
			// Found override
		} else if !os.IsNotExist(err) {
			// Error reading existing file
			return "", fmt.Errorf("failed to read prompt override %s: %w", overridePath, err)
		}
	}

	// 2. Fallback to embedded FS
	if content == nil {
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
