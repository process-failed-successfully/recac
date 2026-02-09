package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Persona represents a system persona with a specific prompt.
type Persona struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	SystemPrompt string `yaml:"system_prompt"`
}

// PersonaManager manages loading and retrieving personas.
type PersonaManager struct {
	personas map[string]Persona
}

// NewPersonaManager creates a new PersonaManager with default personas.
func NewPersonaManager() *PersonaManager {
	pm := &PersonaManager{
		personas: make(map[string]Persona),
	}
	// Add default personas
	pm.personas["default"] = Persona{
		Name:         "Default",
		Description:  "Standard helpful assistant",
		SystemPrompt: "", // Uses agent's default
	}
	pm.personas["developer"] = Persona{
		Name:         "Senior Developer",
		Description:  "Expert software engineer focused on clean code",
		SystemPrompt: "You are a Senior Software Engineer. You write clean, efficient, and well-documented code. You prefer modern best practices.",
	}
	return pm
}

// LoadPersonas loads personas from the configuration file.
// It merges them with defaults.
func (pm *PersonaManager) LoadPersonas() error {
	path := getPersonasFilePath()
	if path == "" {
		return nil // No file to load
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, just use defaults
		}
		// If file exists but read fails, return error
		return fmt.Errorf("failed to read personas file: %w", err)
	}

	var loadedPersonas map[string]Persona
	if err := yaml.Unmarshal(data, &loadedPersonas); err != nil {
		return fmt.Errorf("failed to parse personas file: %w", err)
	}

	for id, p := range loadedPersonas {
		pm.personas[id] = p
	}

	return nil
}

// GetPersona retrieves a persona by ID.
func (pm *PersonaManager) GetPersona(id string) (Persona, bool) {
	p, ok := pm.personas[id]
	return p, ok
}

// AddPersona adds or updates a persona.
func (pm *PersonaManager) AddPersona(id string, p Persona) {
	pm.personas[id] = p
}

// ListSorted returns a sorted list of persona IDs.
// "default" is always first.
func (pm *PersonaManager) ListSorted() []string {
	var ids []string
	for id := range pm.personas {
		if id != "default" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	// Prepend default
	if _, ok := pm.personas["default"]; ok {
		ids = append([]string{"default"}, ids...)
	}
	return ids
}

// getPersonasFilePath determines the path to the personas file.
func getPersonasFilePath() string {
	// 1. Env Var
	if envPath := os.Getenv("RECAC_PERSONAS_FILE"); envPath != "" {
		return envPath
	}

	// 2. Home Dir
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".recac", "personas.yaml")
	}

	// 3. Fallback to relative
	return ".recac/personas.yaml"
}
