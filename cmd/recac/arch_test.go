package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestCheckViolations(t *testing.T) {
	// 1. Setup Config
	config := &ArchConfig{
		Layers: map[string]string{
			"domain": "internal/domain/.*",
			"app":    "internal/app/.*",
			"infra":  "internal/infra/.*",
		},
		Rules: []ArchRule{
			{From: "domain", Allow: []string{}}, // Strict
			{From: "app", Allow: []string{"domain"}},
			{From: "infra", Allow: []string{"domain", "app"}},
		},
	}

	// 2. Compile Regexes
	regexps := make(map[string]*regexp.Regexp)
	for k, v := range config.Layers {
		regexps[k] = regexp.MustCompile(v)
	}

	// 3. Setup Dependencies
	// domain -> ok
	// app -> domain (ok)
	// app -> infra (violation)
	// infra -> app (ok)
	// infra -> unknown (ignored)
	deps := DepMap{
		"internal/domain/user": []string{
			"fmt", // ignored (not in any layer)
		},
		"internal/app/service": []string{
			"internal/domain/user", // ok
			"internal/infra/db",    // VIOLATION: app cannot import infra
		},
		"internal/infra/db": []string{
			"internal/domain/user", // ok
			"internal/app/service", // ok
			"github.com/lib/pq",    // ignored
		},
	}

	// 4. Run Check
	violations := checkViolations(deps, config, regexps)

	// 5. Assert
	assert.Len(t, violations, 1)
	assert.Contains(t, violations[0], "internal/app/service (app) imports internal/infra/db (infra)")
}

func TestLoadArchConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Case 1: No config
	_, err := loadArchConfig("", tmpDir)
	assert.Error(t, err)

	// Case 2: Explicit config
	configContent := `
layers:
  test: "test/.*"
rules:
  - from: "test"
    allow: []
`
	configFile := filepath.Join(tmpDir, "arch.yaml")
	os.WriteFile(configFile, []byte(configContent), 0644)

	config, err := loadArchConfig("arch.yaml", tmpDir)
	assert.NoError(t, err)
	assert.Equal(t, "test/.*", config.Layers["test"])

	// Case 3: Default config generation
	err = generateDefaultArchConfig(tmpDir)
	assert.NoError(t, err)
	target := filepath.Join(tmpDir, ".recac-arch.yaml")
	assert.FileExists(t, target)

	// Read generated config
	genData, _ := os.ReadFile(target)
	var genConfig ArchConfig
	yaml.Unmarshal(genData, &genConfig)
	assert.NotEmpty(t, genConfig.Layers["domain"])
}
