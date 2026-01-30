package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestTopologyCmd(t *testing.T) {
	// Create a temporary docker-compose file
	content := `
version: '3.8'
services:
  web:
    image: nginx
    depends_on:
      - api
      - db
  api:
    image: my-api
    depends_on:
      db:
        condition: service_healthy
  db:
    image: postgres
  cache:
    image: redis
    links:
      - db:database
`
	tmpfile, err := os.CreateTemp("", "docker-compose.*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Setup command
	cmd := &cobra.Command{Use: "test"}
	// We need to set the global flag variable or mock it?
	// topologyFile is a global variable in topology.go
	// This is not thread safe but fine for this test if run sequentially
	originalFile := topologyFile
	defer func() { topologyFile = originalFile }()
	topologyFile = tmpfile.Name()

	// Capture output
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Run logic directly
	err = runTopology(cmd, []string{})
	assert.NoError(t, err)

	output := out.String()

	// Assertions
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "web --> api")
	assert.Contains(t, output, "web --> db")
	assert.Contains(t, output, "api --> db")
	assert.Contains(t, output, "cache --> db") // From links

	// Check sanitization if needed
	// web is web
}
