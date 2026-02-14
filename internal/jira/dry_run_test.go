package jira

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestDryRunClient_CreateTicket(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	client := NewDryRunClient()
	key, err := client.CreateTicket(context.Background(), "PROJ", "Summary", "Description", "Story", []string{"label"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !strings.HasPrefix(key, "DRY-") {
		t.Errorf("Expected key to start with DRY-, got %s", key)
	}
	if !strings.Contains(output, "[DRY-RUN] Creating Story in project PROJ") {
		t.Errorf("Expected output to contain creation message, got: %s", output)
	}
	if !strings.Contains(output, "Summary: Summary") {
		t.Errorf("Expected output to contain summary, got: %s", output)
	}
}

func TestDryRunClient_CreateChildTicket(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	client := NewDryRunClient()
	key, err := client.CreateChildTicket(context.Background(), "PROJ", "Child", "Description", "Subtask", "PROJ-1", []string{"label"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !strings.HasPrefix(key, "DRY-") {
		t.Errorf("Expected key to start with DRY-, got %s", key)
	}
	if !strings.Contains(output, "[DRY-RUN] Creating Child Subtask under PROJ-1") {
		t.Errorf("Expected output to contain child creation message, got: %s", output)
	}
}

func TestDryRunClient_AddIssueLink(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	client := NewDryRunClient()
	err := client.AddIssueLink(context.Background(), "PROJ-1", "PROJ-2", "Blocks")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !strings.Contains(output, "[DRY-RUN] Linking PROJ-1 -> PROJ-2 (Blocks)") {
		t.Errorf("Expected output to contain linking message, got: %s", output)
	}
}
