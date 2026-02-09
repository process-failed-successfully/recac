package agent

import (
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

// Persona represents a specific system prompt configuration
type Persona struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	SystemPrompt string `yaml:"system_prompt"`
}

// PersonaManager manages loading and accessing personas
type PersonaManager struct {
	mu       sync.RWMutex
	personas map[string]Persona
}

// NewPersonaManager initializes the manager with default personas
func NewPersonaManager() *PersonaManager {
	pm := &PersonaManager{
		personas: make(map[string]Persona),
	}

	// Default Personas - Required for tests
	pm.personas["default"] = Persona{
		Name:         "Default",
		Description:  "Standard helpful assistant",
		SystemPrompt: "You are a helpful AI assistant.",
	}
	pm.personas["junior"] = Persona{
		Name:         "Junior Developer",
		Description:  "Eager to learn junior developer",
		SystemPrompt: "You are a junior software engineer. You are eager to learn but might need help with complex topics.",
	}
	pm.personas["teacher"] = Persona{
		Name:         "Teacher",
		Description:  "Expert Computer Science Teacher",
		SystemPrompt: "You are an expert Computer Science Teacher. Explain concepts clearly and patiently.",
	}

	return pm
}

// LoadPersonas attempts to load additional personas from configuration
func (pm *PersonaManager) LoadPersonas() error {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to relative path if Home fails (e.g. in some containers)
		home = "."
	}

	configPath := filepath.Join(home, ".recac", "personas.yaml")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // No custom personas, ignore
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var loaded map[string]Persona
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		return err
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for k, v := range loaded {
		pm.personas[k] = v
	}

	return nil
}

// AddPersona allows adding a persona programmatically (useful for tests)
func (pm *PersonaManager) AddPersona(id string, p Persona) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.personas[id] = p
}

// GetPersona retrieves a persona by ID
func (pm *PersonaManager) GetPersona(id string) (Persona, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.personas[id]
	return p, ok
}

// ListSorted returns a sorted list of persona IDs
func (pm *PersonaManager) ListSorted() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	keys := make([]string, 0, len(pm.personas))
	for k := range pm.personas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
