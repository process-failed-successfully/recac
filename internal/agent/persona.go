package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Persona represents a specific agent personality/system prompt configuration.
type Persona struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	SystemPrompt string `yaml:"system_prompt"`
}

// PersonaManager handles loading and retrieving personas.
type PersonaManager struct {
	personas map[string]Persona
}

// NewPersonaManager creates a new manager with default personas.
func NewPersonaManager() *PersonaManager {
	return &PersonaManager{
		personas: make(map[string]Persona),
	}
}

// DefaultPersonas provides a fallback list of personas if no config file is found.
var DefaultPersonas = map[string]Persona{
	"default": {
		Name:         "Default Assistant",
		Description:  "Standard helpful AI assistant",
		SystemPrompt: "You are a helpful AI assistant.",
	},
	"junior": {
		Name:         "Junior Developer",
		Description:  "Enthusiastic but inexperienced developer",
		SystemPrompt: "You are a junior developer who is eager to learn but might make simple mistakes. Explain your thought process clearly.",
	},
	"senior": {
		Name:         "Senior Architect",
		Description:  "Experienced software architect with deep knowledge",
		SystemPrompt: "You are a senior software architect. Focus on design patterns, scalability, and maintainability. Be concise and authoritative.",
	},
	"teacher": {
		Name:         "Computer Science Teacher",
		Description:  "Patient teacher explaining concepts simply",
		SystemPrompt: "You are an expert Computer Science Teacher. Explain concepts simply and use analogies.",
	},
}

// LoadPersonas loads personas from the configuration file.
// It looks for RECAC_PERSONAS_FILE env var, or defaults to ~/.recac/personas.yaml.
// If the file doesn't exist, it falls back to DefaultPersonas.
func (pm *PersonaManager) LoadPersonas() error {
	// Start with defaults
	pm.personas = make(map[string]Persona)
	for k, v := range DefaultPersonas {
		pm.personas[k] = v
	}

	path := getPersonasFilePath()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// No custom file, stick with defaults (which are already loaded)
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read personas file: %w", err)
	}

	var loaded map[string]Persona
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("failed to parse personas file: %w", err)
	}

	// Merge loaded personas (overwrite defaults)
	for k, v := range loaded {
		pm.personas[k] = v
	}

	return nil
}

func getPersonasFilePath() string {
	path := os.Getenv("RECAC_PERSONAS_FILE")
	if path != "" {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		// Log error? For now just return empty string which will cause os.Stat to fail,
		// triggering default behavior. But we should probably return a path.
		return ""
	}
	return filepath.Join(home, ".recac", "personas.yaml")
}

// GetPersona retrieves a persona by ID.
func (pm *PersonaManager) GetPersona(id string) (Persona, bool) {
	p, ok := pm.personas[id]
	return p, ok
}

// ListSorted returns a sorted list of persona IDs.
func (pm *PersonaManager) ListSorted() []string {
	keys := make([]string, 0, len(pm.personas))
	for k := range pm.personas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
