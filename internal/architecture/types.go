package architecture

// SystemArchitecture represents the root of the architecture.yaml file.
type SystemArchitecture struct {
	Version     string           `yaml:"version" json:"version"`
	SystemName  string           `yaml:"system_name" json:"system_name"`
	Types       []TypeDefinition `yaml:"types,omitempty" json:"types,omitempty"`
	Components  []Component      `yaml:"components" json:"components"`
	Constraints []string         `yaml:"flow_constraints,omitempty" json:"flow_constraints,omitempty"`
}

// TypeDefinition defines a global data type/schema that can be referenced.
type TypeDefinition struct {
	Name   string `yaml:"name" json:"name"`
	Schema string `yaml:"schema" json:"schema"` // Path to schema file or inline definition
}

// Component represents a single logical unit in the system.
type Component struct {
	ID                  string     `yaml:"id" json:"id"`
	Type                string     `yaml:"type" json:"type"` // service, worker, database, etc.
	Description         string     `yaml:"description" json:"description"`
	Contracts           []Contract `yaml:"contracts,omitempty" json:"contracts,omitempty"`
	Consumes            []Input    `yaml:"consumes,omitempty" json:"consumes,omitempty"`
	Produces            []Output   `yaml:"produces,omitempty" json:"produces,omitempty"`
	ImplementationSteps []string   `yaml:"implementation_steps,omitempty" json:"implementation_steps,omitempty"`
	Functions           []Function `yaml:"functions,omitempty" json:"functions,omitempty"`
}

// Function defines a granular unit of logic within a component.
type Function struct {
	Name         string   `yaml:"name" json:"name"`
	Args         string   `yaml:"args" json:"args"`
	Return       string   `yaml:"return" json:"return"`
	Description  string   `yaml:"description" json:"description"`
	Requirements []string `yaml:"requirements,omitempty" json:"requirements,omitempty"`
}

// Contract represents an interface definition (OpenAPI, Proto, etc.).
type Contract struct {
	Type string `yaml:"type" json:"type"` // openapi, proto, go-interface, etc.
	Path string `yaml:"path" json:"path"` // Path to the contract file
}

// Input represents data consumed by a component.
type Input struct {
	Source string `yaml:"source" json:"source"` // Component ID of the producer
	Type   string `yaml:"type" json:"type"`     // Event type or Data type name
	Schema string `yaml:"schema" json:"schema"` // Path to schema or reference
}

// Output represents data produced by a component.
type Output struct {
	Event  string `yaml:"event,omitempty" json:"event,omitempty"`   // Event name if event-driven
	Type   string `yaml:"type,omitempty" json:"type,omitempty"`     // Data type name
	Target string `yaml:"target,omitempty" json:"target,omitempty"` // Explicit target (optional)
	Schema string `yaml:"schema" json:"schema"`                     // Path to schema
}
