package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Persona defines a role for the AI agent.
type Persona struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	SystemPrompt string `yaml:"system_prompt"`
}

// LoadPersonas loads personas from a YAML file.
func LoadPersonas(path string) (map[string]Persona, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[string]Persona), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read personas file: %w", err)
	}

	var personas map[string]Persona
	if err := yaml.Unmarshal(data, &personas); err != nil {
		return nil, fmt.Errorf("failed to parse personas file: %w", err)
	}

	if personas == nil {
		personas = make(map[string]Persona)
	}

	return personas, nil
}

// SavePersonas saves personas to a YAML file.
func SavePersonas(path string, personas map[string]Persona) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := yaml.Marshal(personas)
	if err != nil {
		return fmt.Errorf("failed to marshal personas: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write personas file: %w", err)
	}

	return nil
}
