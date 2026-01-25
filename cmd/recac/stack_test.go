package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectStack_Go(t *testing.T) {
	tmpDir := t.TempDir()

	goModContent := `module example.com/my/project

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	gorm.io/gorm v1.25.5
	github.com/spf13/cobra v1.8.0
	github.com/lib/pq v1.10.9
)
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)

	info, err := detectStack(tmpDir)
	require.NoError(t, err)

	assert.Len(t, info.Languages, 1)
	assert.Equal(t, "Go", info.Languages[0].Name)
	assert.Equal(t, "1.21", info.Languages[0].Version)

	assert.Len(t, info.Frameworks, 3) // Gin, Cobra, GORM

	// Check content loosely
	frameworkNames := []string{}
	for _, f := range info.Frameworks {
		frameworkNames = append(frameworkNames, f.Name)
	}
	assert.Contains(t, frameworkNames, "Gin")
	assert.Contains(t, frameworkNames, "GORM")
	assert.Contains(t, frameworkNames, "Cobra")

	assert.Len(t, info.Databases, 1)
	assert.Equal(t, "PostgreSQL", info.Databases[0].Name)
}

func TestDetectStack_Node(t *testing.T) {
	tmpDir := t.TempDir()

	packageJsonContent := `{
  "name": "my-app",
  "version": "1.0.0",
  "engines": {
    "node": ">=18.0.0"
  },
  "dependencies": {
    "react": "^18.2.0",
    "next": "14.0.0",
    "tailwindcss": "^3.0.0"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJsonContent), 0644)
	require.NoError(t, err)

	info, err := detectStack(tmpDir)
	require.NoError(t, err)

	assert.Len(t, info.Languages, 1)
	assert.Equal(t, "Node.js", info.Languages[0].Name)
	assert.Equal(t, ">=18.0.0", info.Languages[0].Version)

	frameworkNames := []string{}
	for _, f := range info.Frameworks {
		frameworkNames = append(frameworkNames, f.Name)
	}
	assert.Contains(t, frameworkNames, "React")
	assert.Contains(t, frameworkNames, "Next.js")
	assert.Contains(t, frameworkNames, "Tailwind CSS")

	assert.Len(t, info.Tools, 1)
	assert.Equal(t, "TypeScript", info.Tools[0].Name)
}

func TestDetectStack_Python(t *testing.T) {
	tmpDir := t.TempDir()

	reqContent := `
Django==4.2.0
djangorestframework
pandas
`
	err := os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(reqContent), 0644)
	require.NoError(t, err)

	info, err := detectStack(tmpDir)
	require.NoError(t, err)

	assert.Len(t, info.Languages, 1)
	assert.Equal(t, "Python", info.Languages[0].Name)

	frameworkNames := []string{}
	for _, f := range info.Frameworks {
		frameworkNames = append(frameworkNames, f.Name)
	}
	assert.Contains(t, frameworkNames, "Django")
	assert.Contains(t, frameworkNames, "Pandas")
}

func TestDetectStack_Infra(t *testing.T) {
	tmpDir := t.TempDir()

	os.Create(filepath.Join(tmpDir, "Dockerfile"))
	os.Create(filepath.Join(tmpDir, "docker-compose.yml"))
	os.Create(filepath.Join(tmpDir, "main.tf"))

	// K8s file
	k8sContent := `
apiVersion: v1
kind: Pod
metadata:
  name: test
`
	os.WriteFile(filepath.Join(tmpDir, "deployment.yaml"), []byte(k8sContent), 0644)

	info, err := detectStack(tmpDir)
	require.NoError(t, err)

	assert.Contains(t, info.Infrastructure, "Docker")
	assert.Contains(t, info.Infrastructure, "Docker Compose")
	assert.Contains(t, info.Infrastructure, "Terraform")
	assert.Contains(t, info.Infrastructure, "Kubernetes")
}

func TestRunStackCmd_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module foo\ngo 1.20"), 0644)

	// Save original flags
	origJSON := stackJSON
	stackJSON = true
	defer func() { stackJSON = origJSON }()

	// Capture output
	// Since runStack writes to cmd.OutOrStdout(), we can't easily capture it here
	// without full Cobra setup or mocking cmd. But we can just run the logic.

	info, err := detectStack(tmpDir)
	require.NoError(t, err)

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded StackInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "Go", decoded.Languages[0].Name)
}
