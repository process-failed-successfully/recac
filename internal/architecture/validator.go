package architecture

import (
	"fmt"
	"os"
)

// FileSystem interface to allow mocking in tests
type FileSystem interface {
	Stat(name string) (os.FileInfo, error)
}

// RealFileSystem implements FileSystem using os package
type RealFileSystem struct{}

func (fs RealFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// Validator validates a SystemArchitecture
type Validator struct {
	FS FileSystem
}

// NewValidator creates a new Validator with the given FileSystem
func NewValidator(fs FileSystem) *Validator {
	if fs == nil {
		fs = RealFileSystem{}
	}
	return &Validator{FS: fs}
}

// Validate checks the architecture for consistency and file existence
func (v *Validator) Validate(arch *SystemArchitecture) error {
	if arch.Version == "" {
		return fmt.Errorf("version is required")
	}
	if arch.SystemName == "" {
		return fmt.Errorf("system_name is required")
	}
	if len(arch.Components) == 0 {
		return fmt.Errorf("no components defined")
	}

	// 1. Collect all component IDs and Outputs for graph validation
	componentIDs := make(map[string]bool)
	// outputs := make(map[string]Output) // Key: ComponentID:OutputType (or just OutputType if global)

	for _, c := range arch.Components {
		if componentIDs[c.ID] {
			return fmt.Errorf("duplicate component ID: %s", c.ID)
		}
		componentIDs[c.ID] = true

		for _, o := range c.Produces {
			typeName := o.Type
			if typeName == "" {
				typeName = o.Event
			}
			if typeName == "" {
				return fmt.Errorf("component %s output missing type/event", c.ID)
			}
			// Store as ProducerID:Type to allow multiple components to produce same type if needed, 
			// but for strict matching we might want to check uniqueness or allow it.
			// For this validation, let's just track that SOMEONE produces it.
			// key := fmt.Sprintf("%s:%s", c.ID, typeName)
			// outputs[key] = o
		}
	}

	// 2. Validate Components
	for _, c := range arch.Components {
		if err := v.validateComponent(c, componentIDs); err != nil {
			return fmt.Errorf("component %s error: %w", c.ID, err)
		}
	}

	return nil
}

func (v *Validator) validateComponent(c Component, allIDs map[string]bool) error {
	if c.ID == "" {
		return fmt.Errorf("missing ID")
	}
	if c.Type == "" {
		return fmt.Errorf("missing type")
	}

	// Validate Contracts
	for _, contract := range c.Contracts {
		if contract.Path != "" {
			if _, err := v.FS.Stat(contract.Path); err != nil {
				return fmt.Errorf("contract file not found: %s", contract.Path)
			}
		}
	}

	// Validate Inputs
	for _, input := range c.Consumes {
		if input.Source != "" && !allIDs[input.Source] {
			return fmt.Errorf("input source '%s' does not exist", input.Source)
		}
		if input.Schema != "" {
			if _, err := v.FS.Stat(input.Schema); err != nil {
				return fmt.Errorf("input schema file not found: %s", input.Schema)
			}
		}
	}

	// Validate Outputs
	for _, output := range c.Produces {
		if output.Target != "" && !allIDs[output.Target] {
			return fmt.Errorf("output target '%s' does not exist", output.Target)
		}
		if output.Schema != "" {
			if _, err := v.FS.Stat(output.Schema); err != nil {
				return fmt.Errorf("output schema file not found: %s", output.Schema)
			}
		}
	}

	return nil
}
