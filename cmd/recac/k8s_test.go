package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
)

func TestK8sGenerate(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-k8s-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp dir to verify Dockerfile detection
	cwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	// Create dummy Dockerfile
	dockerfileContent := "FROM alpine\nEXPOSE 8080\nCMD [\"app\"]"
	if err := os.WriteFile("Dockerfile", []byte(dockerfileContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Mock Agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := agent.NewMockAgent()

	// Prepare mock response
	mockResponse := `
Here are the manifests:
<file path="deployment.yaml">
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 2
</file>
<file path="service.yaml">
apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  ports:
  - port: 80
</file>
`
	mockAgent.SetResponse(mockResponse)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 3. Setup Command and Flags
	// Reset global flags
	k8sOutputDir = "k8s-out"
	k8sPort = ""
	k8sReplicas = 2
	k8sHelm = false
	k8sNamespace = "default"

	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())

	// 4. Run Command
	err = runK8s(cmd, []string{})
	if err != nil {
		t.Fatalf("runK8s failed: %v", err)
	}

	// 5. Verify Output
	output := buf.String()
	if !strings.Contains(output, "Found Dockerfile") {
		t.Errorf("Expected output to mention Dockerfile, got:\n%s", output)
	}
	if !strings.Contains(output, "Detected port: 8080") {
		t.Errorf("Expected output to detect port 8080, got:\n%s", output)
	}
	if !strings.Contains(output, "Created k8s-out/deployment.yaml") {
		t.Errorf("Expected output to mention deployment creation, got:\n%s", output)
	}

	// 6. Verify Files Created
	depPath := filepath.Join(tmpDir, "k8s-out", "deployment.yaml")
	content, err := os.ReadFile(depPath)
	if err != nil {
		t.Errorf("Failed to read generated deployment: %v", err)
	}
	if !strings.Contains(string(content), "name: test-app") {
		t.Errorf("Generated content mismatch in deployment.yaml")
	}

	svcPath := filepath.Join(tmpDir, "k8s-out", "service.yaml")
	if _, err := os.Stat(svcPath); os.IsNotExist(err) {
		t.Errorf("service.yaml was not created")
	}
}

func TestK8sHelm(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-k8s-helm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	// 2. Mock Agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := agent.NewMockAgent()

	// Prepare mock response for Helm
	mockResponse := `
<file path="Chart.yaml">
apiVersion: v2
name: test-chart
version: 0.1.0
</file>
<file path="templates/deployment.yaml">
kind: Deployment
</file>
`
	mockAgent.SetResponse(mockResponse)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 3. Setup Command with Helm flag
	k8sOutputDir = "chart-out"
	k8sHelm = true
	k8sPort = "3000" // Manual port

	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())

	// 4. Run Command
	err = runK8s(cmd, []string{})
	if err != nil {
		t.Fatalf("runK8s (Helm) failed: %v", err)
	}

	// 5. Verify Output
	output := buf.String()
	if !strings.Contains(output, "Created chart-out/Chart.yaml") {
		t.Errorf("Expected Chart.yaml creation, got:\n%s", output)
	}

	// 6. Verify Files
	chartPath := filepath.Join(tmpDir, "chart-out", "Chart.yaml")
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		t.Errorf("Chart.yaml not created")
	}

	tplPath := filepath.Join(tmpDir, "chart-out", "templates", "deployment.yaml")
	if _, err := os.Stat(tplPath); os.IsNotExist(err) {
		t.Errorf("templates/deployment.yaml not created")
	}
}
