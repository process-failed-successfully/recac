package main

import (
	"bytes"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLanguage(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		filename string
		expected string
	}{
		{"Go", ".go", "main.go", "Go"},
		{"JavaScript", ".js", "script.js", "JavaScript"},
		{"TypeScript", ".ts", "app.ts", "TypeScript"},
		{"Python", ".py", "script.py", "Python"},
		{"Java", ".java", "Main.java", "Java"},
		{"Ruby", ".rb", "script.rb", "Ruby"},
		{"Rust", ".rs", "main.rs", "Rust"},
		{"PHP", ".php", "index.php", "PHP"},
		{"HTML", ".html", "index.html", "HTML"},
		{"CSS", ".css", "style.css", "CSS"},
		{"Shell", ".sh", "script.sh", "Shell"},
		{"SQL", ".sql", "query.sql", "SQL"},
		{"Makefile", "", "Makefile", "Make"},
		{"Unknown", ".xyz", "file.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, getLanguage(tt.ext, tt.filename))
		})
	}
}

func TestScanGoMod(t *testing.T) {
	content := `module example.com/my/app

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/spf13/cobra v1.8.0
	gorm.io/gorm v1.25.5
)
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	frameworks := scanGoMod(path)
	assert.Contains(t, frameworks, "Gin")
	assert.Contains(t, frameworks, "Cobra")
	assert.Contains(t, frameworks, "GORM")
	assert.NotContains(t, frameworks, "Fiber")
}

func TestScanPackageJson(t *testing.T) {
	content := `{
  "name": "my-app",
  "dependencies": {
    "react": "^18.2.0",
    "next": "13.5.6",
    "tailwindcss": "^3.3.0"
  }
}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "package.json")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	frameworks := scanPackageJson(path)
	assert.Contains(t, frameworks, "React")
	assert.Contains(t, frameworks, "Next.js")
	assert.Contains(t, frameworks, "Tailwind CSS")
	assert.NotContains(t, frameworks, "Vue")
}

func TestScanDockerCompose(t *testing.T) {
	content := `version: '3.8'
services:
  db:
    image: postgres:15
  cache:
    image: redis:7
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "docker-compose.yml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	dbs := scanDockerCompose(path)
	assert.Contains(t, dbs, "PostgreSQL")
	assert.Contains(t, dbs, "Redis")
	assert.NotContains(t, dbs, "MySQL")
}

func TestScanRequirementsTxt(t *testing.T) {
	content := `Django==4.2.7
numpy>=1.26.0
pandas
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "requirements.txt")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	frameworks := scanRequirementsTxt(path)
	assert.Contains(t, frameworks, "Django")
	assert.Contains(t, frameworks, "NumPy")
	assert.Contains(t, frameworks, "Pandas")
	assert.NotContains(t, frameworks, "Flask")
}

func TestAnalyzeStack(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files
	files := map[string]string{
		"main.go":                  "package main",
		"go.mod":                   "module example\nrequire github.com/gin-gonic/gin v1.9.0",
		"package.json":             `{"dependencies": {"react": "^18.0.0"}}`,
		"docker-compose.yml":       "services:\n  db:\n    image: postgres:15",
		".github/workflows/ci.yml": "name: CI",
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		err := os.MkdirAll(filepath.Dir(path), 0755)
		require.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	info, err := analyzeStack(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 1, info.Languages["Go"])
	assert.Contains(t, info.Frameworks, "Gin")
	assert.Contains(t, info.Frameworks, "React")
	assert.Contains(t, info.Infrastructure, "Docker Compose")
	assert.Contains(t, info.Databases, "PostgreSQL")
	assert.Contains(t, info.CI, "GitHub Actions")
}

func TestAppendUnique(t *testing.T) {
	slice := []string{"a", "b"}
	slice = appendUnique(slice, "b")
	assert.Equal(t, []string{"a", "b"}, slice)

	slice = appendUnique(slice, "c")
	assert.Equal(t, []string{"a", "b", "c"}, slice)
}

func TestStackContains(t *testing.T) {
	slice := []string{"a", "b"}
	assert.True(t, stackContains(slice, "a"))
	assert.False(t, stackContains(slice, "c"))
}

func TestRunStack(t *testing.T) {
	// Create a temp dir with some files
	tmpDir := t.TempDir()
	files := map[string]string{
		"main.go":                  "package main",
		"go.mod":                   "module example\nrequire github.com/gin-gonic/gin v1.9.0",
		"package.json":             `{"dependencies": {"react": "^18.0.0"}}`,
		"docker-compose.yml":       "services:\n  db:\n    image: postgres:15",
		".github/workflows/ci.yml": "name: CI",
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		err := os.MkdirAll(filepath.Dir(path), 0755)
		require.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		wantOut []string
	}{
		{
			name:    "Default Table Output",
			args:    []string{tmpDir},
			wantErr: false,
			wantOut: []string{"PROJECT STACK", "LANGUAGES:", "FRAMEWORKS & LIBRARIES:", "INFRASTRUCTURE & CI:", "DATABASES:"},
		},
		{
			name:    "JSON Output",
			args:    []string{tmpDir, "--json"},
			wantErr: false,
			wantOut: []string{"\"languages\":", "\"frameworks\":", "\"infrastructure\":", "\"databases\":", "\"ci\":"},
		},
		{
			name:    "Mermaid Output",
			args:    []string{tmpDir, "--mermaid"},
			wantErr: false,
			wantOut: []string{"graph TD", "App[", "FW0([", "Infra0[", "CI0{{", "DB0[("},
		},
		{
			name:    "Invalid Directory",
			args:    []string{"/path/to/non/existent/directory/xyz123"},
			wantErr: true,
			wantOut: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new command to reset flags
			cmd := &cobra.Command{}
			cmd.Flags().Bool("json", false, "Output as JSON")
			cmd.Flags().Bool("mermaid", false, "Output as a Mermaid component diagram")

			// Parse custom flags manually
			var cmdArgs []string
			if len(tt.args) > 0 {
				if tt.args[len(tt.args)-1] == "--json" {
					cmd.Flags().Set("json", "true")
					cmdArgs = tt.args[:len(tt.args)-1]
				} else if tt.args[len(tt.args)-1] == "--mermaid" {
					cmd.Flags().Set("mermaid", "true")
					cmdArgs = tt.args[:len(tt.args)-1]
				} else {
					cmdArgs = tt.args
				}
			}

			// Capture output
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)

			err := runStack(cmd, cmdArgs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				out := buf.String()
				for _, want := range tt.wantOut {
					assert.Contains(t, out, want)
				}
			}
		})
	}
}

func TestPrintStackTable(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	info := &StackInfo{
		Languages:      map[string]int{"Go": 5, "Python": 2},
		Frameworks:     []string{"Gin", "React"},
		Infrastructure: []string{"Docker"},
		Databases:      []string{"PostgreSQL", "Redis"},
		CI:             []string{"GitHub Actions"},
	}

	printStackTable(cmd, info)
	out := buf.String()

	assert.Contains(t, out, "LANGUAGES:")
	assert.Contains(t, out, "- Go (5 files)")
	assert.Contains(t, out, "- Python (2 files)")
	assert.Contains(t, out, "FRAMEWORKS & LIBRARIES:")
	assert.Contains(t, out, "- Gin")
	assert.Contains(t, out, "- React")
	assert.Contains(t, out, "INFRASTRUCTURE & CI:")
	assert.Contains(t, out, "- Docker")
	assert.Contains(t, out, "- GitHub Actions")
	assert.Contains(t, out, "DATABASES:")
	assert.Contains(t, out, "- PostgreSQL")
	assert.Contains(t, out, "- Redis")

	// Test empty info
	buf.Reset()
	emptyInfo := &StackInfo{
		Languages:      map[string]int{},
		Frameworks:     []string{},
		Infrastructure: []string{},
		Databases:      []string{},
		CI:             []string{},
	}
	printStackTable(cmd, emptyInfo)
	outEmpty := buf.String()
	countNoneDetected := strings.Count(outEmpty, "None detected")
	assert.Equal(t, 4, countNoneDetected)
}

func TestPrintStackMermaid(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	info := &StackInfo{
		Languages:      map[string]int{"Go": 5, "Python": 2},
		Frameworks:     []string{"Gin", "React"},
		Infrastructure: []string{"Docker"},
		Databases:      []string{"PostgreSQL", "Redis"},
		CI:             []string{"GitHub Actions"},
	}

	printStackMermaid(cmd, info)
	out := buf.String()

	assert.Contains(t, out, "graph TD")
	assert.Contains(t, out, "App[Go App]")
	assert.Contains(t, out, "FW0([Gin]) -.-> App")
	assert.Contains(t, out, "FW1([React]) -.-> App")
	assert.Contains(t, out, "App --- Infra0[Docker]")
	assert.Contains(t, out, "CI0{{GitHub Actions}} --> App")
	assert.Contains(t, out, "App <--> DB0[(PostgreSQL)]")
	assert.Contains(t, out, "App <--> DB1[(Redis)]")

	// Test empty info
	buf.Reset()
	emptyInfo := &StackInfo{
		Languages:      map[string]int{},
		Frameworks:     []string{},
		Infrastructure: []string{},
		Databases:      []string{},
		CI:             []string{},
	}
	printStackMermaid(cmd, emptyInfo)
	outEmpty := buf.String()
	assert.Contains(t, outEmpty, "graph TD")
	assert.Contains(t, outEmpty, "App[Application]")
}
