package prompts

import (
	"embed"
	"fmt"
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
func GetPrompt(name string, vars map[string]string) (string, error) {
	templatePath := filepath.Join("templates", name+".md")

	content, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt template %s: %w", name, err)
	}

	prompt := string(content)
	for k, v := range vars {
		placeholder := fmt.Sprintf("{%s}", k)
		prompt = strings.ReplaceAll(prompt, placeholder, v)
	}

	return prompt, nil
}
