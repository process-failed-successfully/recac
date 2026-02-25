package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanForDependencyFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tech_radar_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some files
	files := []string{
		"go.mod",
		"package.json",
		"other.txt",
		"Dockerfile",
		"src/main.go", // Nested, should not be picked up unless in specific paths?
		".github/workflows/ci.yml",
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	found, err := ScanForDependencyFiles(tmpDir)
	if err != nil {
		t.Fatalf("ScanForDependencyFiles failed: %v", err)
	}

	// Check if expected files are found
	expected := map[string]bool{
		"go.mod":                   true,
		"package.json":             true,
		"Dockerfile":               true,
		".github/workflows/ci.yml": true,
	}

	// Convert found paths to relative paths for easier comparison
	foundMap := make(map[string]bool)
	for _, p := range found {
		rel, err := filepath.Rel(tmpDir, p)
		if err != nil {
			t.Fatal(err)
		}
		// Windows fix
		rel = filepath.ToSlash(rel)
		foundMap[rel] = true
	}

	for exp := range expected {
		if !foundMap[exp] {
			t.Errorf("Expected file %s not found", exp)
		}
	}

	if foundMap["other.txt"] {
		t.Errorf("Unexpected file other.txt found")
	}
	if foundMap["src/main.go"] {
		t.Errorf("Unexpected file src/main.go found")
	}
}

func TestGenerateRadarHTML(t *testing.T) {
	items := []RadarItem{
		{
			Name:        "Go",
			Quadrant:    "Languages & Frameworks",
			Ring:        "Adopt",
			Description: "Main language",
		},
		{
			Name:        "Docker",
			Quadrant:    "Tools",
			Ring:        "Trial",
			Description: "Containerization",
		},
	}

	html, err := GenerateRadarHTML(items)
	if err != nil {
		t.Fatalf("GenerateRadarHTML failed: %v", err)
	}

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Errorf("HTML output does not look like HTML")
	}

	// Verify JSON data is embedded
	jsonData, _ := json.Marshal(items)
	if !strings.Contains(html, string(jsonData)) {
		t.Errorf("HTML output does not contain items JSON")
	}

	// Verify items appear in the HTML (though they are rendered via JS, the data is there)
	if !strings.Contains(html, "Go") {
		t.Errorf("HTML output missing item name 'Go'")
	}
}
