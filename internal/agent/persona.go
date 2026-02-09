package agent

import (
	"sort"
)

type Persona struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	SystemPrompt string `yaml:"system_prompt"`
}

type PersonaManager struct {
	personas map[string]Persona
}

func NewPersonaManager() *PersonaManager {
	return &PersonaManager{
		personas: make(map[string]Persona),
	}
}

func (pm *PersonaManager) LoadPersonas() error {
    // Load defaults first
    pm.personas["default"] = Persona{
        Name: "Default",
        Description: "Standard Recac Assistant",
        SystemPrompt: "You are Recac, a helpful AI assistant.",
    }

    pm.personas["junior"] = Persona{
        Name: "Junior Developer",
        Description: "Learning the ropes",
        SystemPrompt: "You are a junior developer.",
    }

    pm.personas["teacher"] = Persona{
        Name: "CS Teacher",
        Description: "Computer Science Teacher",
        SystemPrompt: "You are an expert Computer Science Teacher.",
    }

    return nil
}

func (pm *PersonaManager) GetPersona(id string) (Persona, bool) {
	p, ok := pm.personas[id]
	return p, ok
}

func (pm *PersonaManager) ListSorted() []string {
	keys := make([]string, 0, len(pm.personas))
	for k := range pm.personas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// AddPersona is needed for testing?
func (pm *PersonaManager) AddPersona(id string, p Persona) {
    pm.personas[id] = p
}
