package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Persona represents an AI persona with a specific system prompt.
type Persona struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	SystemPrompt string `yaml:"system_prompt"`
}

// PersonaManager manages the loading, saving, and selection of personas.
type PersonaManager struct {
	customPersonas map[string]Persona
	mu             sync.RWMutex
}

var defaultPersonas = map[string]Persona{
	"Default": {
		Name:         "Default",
		Description:  "Standard AI behavior",
		SystemPrompt: "You are a helpful AI assistant.",
	},
	"Senior Go Developer": {
		Name:         "Senior Go Developer",
		Description:  "Expert in Go, concurrency, and clean code",
		SystemPrompt: "You are a Senior Go Developer. You write idiomatic, performant, and thread-safe Go code. You prefer the standard library over external dependencies where possible. You strictly follow Effective Go guidelines.",
	},
	"Python Data Scientist": {
		Name:         "Python Data Scientist",
		Description:  "Expert in Pandas, NumPy, and data analysis",
		SystemPrompt: "You are a Python Data Scientist. You are proficient with Pandas, NumPy, Scikit-learn, and data visualization tools. You write concise, vectorized code.",
	},
	"Security Auditor": {
		Name:         "Security Auditor",
		Description:  "Focuses on vulnerabilities and secure coding",
		SystemPrompt: "You are a Security Auditor. You analyze code for potential security vulnerabilities (OWASP Top 10, CWE). You suggest defensive programming techniques and secure configuration.",
	},
	"Technical Writer": {
		Name:         "Technical Writer",
		Description:  "Expert in documentation and clear explanation",
		SystemPrompt: "You are a Technical Writer. You explain complex technical concepts in simple, clear language. You focus on documentation, comments, and READMEs.",
	},
}

// NewPersonaManager creates a new PersonaManager and loads custom personas.
func NewPersonaManager() *PersonaManager {
	pm := &PersonaManager{
		customPersonas: make(map[string]Persona),
	}
	pm.loadCustomPersonas()
	return pm
}

func (pm *PersonaManager) loadCustomPersonas() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// Load from ~/.recac/personas.yaml
	path := filepath.Join(home, ".recac", "personas.yaml")
	// Using environment variable for testing isolation
	if envPath := os.Getenv("RECAC_PERSONAS_FILE"); envPath != "" {
		path = envPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var personas []Persona
	if err := yaml.Unmarshal(data, &personas); err == nil {
		pm.mu.Lock()
		defer pm.mu.Unlock()
		for _, p := range personas {
			pm.customPersonas[p.Name] = p
		}
	}
}

// SaveCustomPersona saves a new custom persona.
func (pm *PersonaManager) SaveCustomPersona(p Persona) error {
	pm.mu.Lock()
	pm.customPersonas[p.Name] = p
	pm.mu.Unlock()

	return pm.persist()
}

// DeleteCustomPersona deletes a custom persona.
func (pm *PersonaManager) DeleteCustomPersona(name string) error {
	pm.mu.Lock()
	if _, ok := pm.customPersonas[name]; !ok {
		pm.mu.Unlock()
		return fmt.Errorf("persona '%s' not found or is a default persona", name)
	}
	delete(pm.customPersonas, name)
	pm.mu.Unlock()

	return pm.persist()
}

func (pm *PersonaManager) persist() error {
	pm.mu.RLock()
	var personas []Persona
	for _, p := range pm.customPersonas {
		personas = append(personas, p)
	}
	pm.mu.RUnlock()

	data, err := yaml.Marshal(personas)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, ".recac")
	path := filepath.Join(dir, "personas.yaml")
	// Using environment variable for testing isolation
	if envPath := os.Getenv("RECAC_PERSONAS_FILE"); envPath != "" {
		path = envPath
		dir = filepath.Dir(path)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// ListPersonas returns all available personas (default + custom).
func (pm *PersonaManager) ListPersonas() []Persona {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var all []Persona
	for _, p := range defaultPersonas {
		all = append(all, p)
	}
	for _, p := range pm.customPersonas {
		// Override default if name matches?
		// For now, let's assume unique names or custom overrides default
		found := false
		for i, def := range all {
			if def.Name == p.Name {
				all[i] = p
				found = true
				break
			}
		}
		if !found {
			all = append(all, p)
		}
	}
	return all
}

// GetPersona returns a persona by name.
func (pm *PersonaManager) GetPersona(name string) (Persona, bool) {
	pm.mu.RLock()
	if p, ok := pm.customPersonas[name]; ok {
		pm.mu.RUnlock()
		return p, true
	}
	pm.mu.RUnlock()

	if p, ok := defaultPersonas[name]; ok {
		return p, true
	}

	return Persona{}, false
}

// SetActivePersona sets the active persona globally (in ~/.recac/active_persona).
func (pm *PersonaManager) SetActivePersona(name string) error {
	if _, ok := pm.GetPersona(name); !ok {
		return fmt.Errorf("persona '%s' does not exist", name)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	path := filepath.Join(home, ".recac", "active_persona")
	if envPath := os.Getenv("RECAC_ACTIVE_PERSONA_FILE"); envPath != "" {
		path = envPath
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(name), 0644)
}

// GetActivePersona returns the active persona.
// It checks local .recac/active_persona first, then global ~/.recac/active_persona.
// Defaults to "Default".
func (pm *PersonaManager) GetActivePersona() Persona {
	// 1. Check local
	cwd, _ := os.Getwd()
	if cwd != "" {
		localPath := filepath.Join(cwd, ".recac", "active_persona")
		if content, err := os.ReadFile(localPath); err == nil {
			name := strings.TrimSpace(string(content))
			if p, ok := pm.GetPersona(name); ok {
				return p
			}
		}
	}

	// 2. Check global
	home, _ := os.UserHomeDir()
	if home != "" {
		globalPath := filepath.Join(home, ".recac", "active_persona")
		if envPath := os.Getenv("RECAC_ACTIVE_PERSONA_FILE"); envPath != "" {
			globalPath = envPath
		}

		if content, err := os.ReadFile(globalPath); err == nil {
			name := strings.TrimSpace(string(content))
			if p, ok := pm.GetPersona(name); ok {
				return p
			}
		}
	}

	return defaultPersonas["Default"]
}

// PersonaAgentWrapper wraps an Agent and prepends the system prompt.
type PersonaAgentWrapper struct {
	Agent        Agent
	SystemPrompt string
}

func (w *PersonaAgentWrapper) Send(ctx context.Context, prompt string) (string, error) {
	modifiedPrompt := fmt.Sprintf("SYSTEM INSTRUCTION: %s\n\nUSER PROMPT: %s", w.SystemPrompt, prompt)
	return w.Agent.Send(ctx, modifiedPrompt)
}

func (w *PersonaAgentWrapper) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	modifiedPrompt := fmt.Sprintf("SYSTEM INSTRUCTION: %s\n\nUSER PROMPT: %s", w.SystemPrompt, prompt)
	return w.Agent.SendStream(ctx, modifiedPrompt, onChunk)
}
