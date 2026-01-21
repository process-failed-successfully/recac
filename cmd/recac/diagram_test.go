package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiagramCommand(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "recac-diagram-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create sample Go files
	files := map[string]string{
		"main.go": `package main

type User struct {
	ID       int
	Name     string
	Address  *Address
	Orders   []Order
	Settings map[string]string
}

type Address struct {
	Street string
	City   string
}

type Order struct {
	ID    int
	Total float64
}

// Embedded struct test
type Employee struct {
	User
	Role string
}
`,
		"other.go": `package main
type Product struct {
	ID   int
	Name string
}
`,
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Helper to run diagram command
	run := func(args ...string) string {
		// Need to switch to tmpDir or pass it as argument
		// passing as argument is cleaner as per command design
		fullArgs := append([]string{"diagram", tmpDir}, args...)
		output, err := executeCommand(rootCmd, fullArgs...)
		require.NoError(t, err)
		return output
	}

	t.Run("Basic Diagram Generation", func(t *testing.T) {
		output := run()
		assert.Contains(t, output, "classDiagram")

		// Check for classes (sanitized IDs include package name)
		assert.Contains(t, output, "class main_User {")
		assert.Contains(t, output, "class main_Address {")
		assert.Contains(t, output, "class main_Order {")
		assert.Contains(t, output, "class main_Employee {")

		// Check for fields
		assert.Contains(t, output, "int ID")
		assert.Contains(t, output, "string Name")

		// Check for relationships
		// User has Address
		assert.Contains(t, output, "main_User --> main_Address")
		// User has Orders (Order)
		assert.Contains(t, output, "main_User --> main_Order")
		// Employee embeds User
		assert.Contains(t, output, "main_Employee *-- main_User")
	})

	t.Run("Focus Filtering", func(t *testing.T) {
		output := run("--focus", "User")
		assert.Contains(t, output, "class main_User {")
		assert.NotContains(t, output, "class main_Order {") // Should be filtered out

		// Relationships where target is filtered ARE shown if target exists in analysis
		// Because User depends on Address, and Address exists (just filtered from inclusion),
		// we expect the relationship to be present to show dependencies.
		assert.Contains(t, output, "main_User --> main_Address")
	})

	t.Run("Output to File", func(t *testing.T) {
		outFile := filepath.Join(tmpDir, "diagram.mmd")
		output := run("--output", outFile)

		assert.Contains(t, output, "Diagram saved to")

		content, err := os.ReadFile(outFile)
		require.NoError(t, err)
		sContent := string(content)
		assert.Contains(t, sContent, "classDiagram")
		assert.Contains(t, sContent, "class main_User {")
	})

	t.Run("No Fields", func(t *testing.T) {
		output := run("--fields=false")
		assert.Contains(t, output, "class main_User {")
		assert.NotContains(t, output, "int ID")
		assert.Contains(t, output, "main_User --> main_Address")
	})
}
