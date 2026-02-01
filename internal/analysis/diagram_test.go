package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeStructs(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "analysis_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a sample go file
	code := `package sample

type User struct {
	ID   int
	Name string
	Profile *Profile
}

type Profile struct {
	Bio string
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "sample.go"), []byte(code), 0644)
	assert.NoError(t, err)

	// Analyze
	classes, relationships, err := AnalyzeStructs(tmpDir)
	assert.NoError(t, err)

	// Verify Classes
	assert.Contains(t, classes, "sample.User")
	assert.Contains(t, classes, "sample.Profile")

	user := classes["sample.User"]
	assert.Equal(t, "User", user.Name)
	assert.Equal(t, "sample", user.Package)

	// Verify Relationships
	found := false
	for _, rel := range relationships {
		if rel.From == "sample.User" && rel.To == "sample.Profile" && rel.Type == "has" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected User has Profile relationship")
}

func TestGenerateMermaidClassDiagram(t *testing.T) {
	classes := map[string]*ClassDef{
		"sample.A": {Name: "A", Package: "sample", Fields: []string{"int X"}},
		"sample.B": {Name: "B", Package: "sample"},
	}
	rels := []Relationship{
		{From: "sample.A", To: "sample.B", Type: "has"},
	}

	mermaid := GenerateMermaidClassDiagram(classes, rels, nil, true)

	assert.Contains(t, mermaid, "classDiagram")
	assert.Contains(t, mermaid, "class sample_A {")
	assert.Contains(t, mermaid, "int X")
	assert.Contains(t, mermaid, "sample_A --> sample_B")
}
