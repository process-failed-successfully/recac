package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Persona defines a role for the AI agent.
type Persona struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	SystemPrompt string `yaml:"system_prompt"`
}

// PersonaManager handles loading and managing personas.
type PersonaManager struct {
	personas map[string]Persona
}

// DefaultPersonas contains the built-in personas.
var DefaultPersonas = map[string]Persona{
	"default": {
		Name:         "Default",
		Description:  "A helpful and versatile software engineer assistant.",
		SystemPrompt: "You are a helpful software engineer assistant. Answer questions concisely and accurately.",
	},
	"security": {
		Name:         "Security Auditor",
		Description:  "Focuses on identifying vulnerabilities and security best practices.",
		SystemPrompt: "You are a paranoid Security Auditor. You review every piece of code and idea for potential security vulnerabilities (OWASP Top 10, injection, etc.). You are critical and prioritize safety over convenience.",
	},
	"product": {
		Name:         "Product Manager",
		Description:  "Focuses on user value, metrics, and business goals.",
		SystemPrompt: "You are a pragmatic Product Manager. You care about user value, business metrics, and trade-offs. You ask 'Why are we building this?' and 'How does this help the user?'. Avoid technical jargon where possible.",
	},
	"junior": {
		Name:         "Junior Developer",
		Description:  "Needs simple explanations and mentorship.",
		SystemPrompt: "You are a Junior Developer who is eager to learn but often confused. You ask for clarification on complex topics and prefer simple, step-by-step explanations. You admit when you don't understand.",
	},
	"skeptic": {
		Name:         "The Skeptic",
		Description:  "Challenges assumptions and looks for edge cases.",
		SystemPrompt: "You are a Senior Engineer who has seen it all fail. You are skeptical of new libraries, patterns, and 'happy path' thinking. You always ask 'What if this fails?' and 'Have you considered the edge case X?'.",
	},
	"teacher": {
		Name:         "The Teacher",
		Description:  "Uses Socratic method to guide learning.",
		SystemPrompt: "You are an expert Computer Science Teacher. Instead of giving the answer directly, you often ask guiding questions to help the user derive the answer. You focus on first principles and clean code.",
	},
}

// NewPersonaManager creates a new manager with default personas loaded.
func NewPersonaManager() *PersonaManager {
	pm := &PersonaManager{
		personas: make(map[string]Persona),
	}
	// Load defaults
	for k, v := range DefaultPersonas {
		pm.personas[k] = v
	}
	return pm
}

// LoadPersonas loads personas from the configuration file, overriding defaults if necessary.
func (pm *PersonaManager) LoadPersonas() error {
	path, err := getPersonasFilePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // No custom personas file, just use defaults
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read personas file: %w", err)
	}

	var customPersonas map[string]Persona
	if err := yaml.Unmarshal(data, &customPersonas); err != nil {
		return fmt.Errorf("failed to parse personas file: %w", err)
	}

	for k, v := range customPersonas {
		pm.personas[k] = v
	}

	return nil
}

// SavePersonas saves ONLY the custom personas to the configuration file.
func (pm *PersonaManager) SavePersonas() error {
	path, err := getPersonasFilePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	toSave := make(map[string]Persona)
	for k, v := range pm.personas {
		// specific logic: if it's a built-in one and hasn't changed, don't save it.
		if def, ok := DefaultPersonas[k]; ok {
			if def == v {
				continue
			}
		}
		toSave[k] = v
	}

	data, err := yaml.Marshal(toSave)
	if err != nil {
		return fmt.Errorf("failed to marshal personas: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write personas file: %w", err)
	}

	return nil
}

func (pm *PersonaManager) AddPersona(id string, p Persona) {
	pm.personas[id] = p
}

func (pm *PersonaManager) RemovePersona(id string) error {
	if _, ok := pm.personas[id]; !ok {
		return errors.New("persona not found")
	}

	if def, isDefault := DefaultPersonas[id]; isDefault {
		// Reset to default
		pm.personas[id] = def
		return nil
	}

	delete(pm.personas, id)
	return nil
}

func (pm *PersonaManager) GetPersona(id string) (Persona, bool) {
	p, ok := pm.personas[id]
	return p, ok
}

func (pm *PersonaManager) ListPersonas() map[string]Persona {
	return pm.personas
}

// ListSorted returns personas sorted by ID
func (pm *PersonaManager) ListSorted() []string {
	keys := make([]string, 0, len(pm.personas))
	for k := range pm.personas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func getPersonasFilePath() (string, error) {
	// Priority:
	// 1. $RECAC_PERSONAS_FILE
	// 2. $HOME/.recac/personas.yaml

	if path := os.Getenv("RECAC_PERSONAS_FILE"); path != "" {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".recac", "personas.yaml"), nil
}
