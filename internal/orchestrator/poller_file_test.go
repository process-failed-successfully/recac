package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFilePoller_NewFilePoller(t *testing.T) {
	poller := NewFilePoller("test.json")
	if poller == nil {
		t.Fatal("NewFilePoller returned nil")
	}
	if poller.path != "test.json" {
		t.Errorf("expected path 'test.json', got '%s'", poller.path)
	}
}

func TestFilePoller_Poll(t *testing.T) {
	ctx := context.Background()

	t.Run("file not found", func(t *testing.T) {
		poller := NewFilePoller("nonexistent.json")
		items, err := poller.Poll(ctx)
		if err != nil {
			t.Fatalf("Poll() returned an unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("read error", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "unreadable.json")
		if err := os.WriteFile(filePath, []byte("content"), 0000); err != nil {
			t.Fatalf("Failed to write unreadable file: %v", err)
		}

		poller := NewFilePoller(filePath)
		_, err := poller.Poll(ctx)
		if err == nil {
			t.Error("Poll() expected an error for unreadable file, but got none")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "invalid.json")
		if err := os.WriteFile(filePath, []byte("{"), 0644); err != nil {
			t.Fatalf("Failed to write invalid json file: %v", err)
		}

		poller := NewFilePoller(filePath)
		_, err := poller.Poll(ctx)
		if err == nil {
			t.Error("Poll() expected an error for invalid JSON, but got none")
		}
	})

	t.Run("successful poll and filter", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "work.json")
		content := `[{"id": "ITEM-1"}, {"id": "ITEM-2"}]`
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write work file: %v", err)
		}

		poller := NewFilePoller(filePath)

		// First poll, should get 2 items
		items, err := poller.Poll(ctx)
		if err != nil {
			t.Fatalf("Poll() returned an unexpected error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}

		// Claim one item
		if err := poller.Claim(ctx, items[0]); err != nil {
			t.Fatalf("Claim() failed: %v", err)
		}

		// Second poll, should get 1 item
		items, err = poller.Poll(ctx)
		if err != nil {
			t.Fatalf("Poll() returned an unexpected error: %v", err)
		}
		if len(items) != 1 {
			t.Errorf("expected 1 item after claim, got %d", len(items))
		}
		if items[0].ID != "ITEM-2" {
			t.Errorf("expected item with ID 'ITEM-2', got '%s'", items[0].ID)
		}
	})
}

func TestFilePoller_UpdateStatus(t *testing.T) {
	poller := NewFilePoller("")
	err := poller.UpdateStatus(context.Background(), WorkItem{ID: "TEST"}, "testing", "comment")
	if err != nil {
		t.Errorf("UpdateStatus() returned an unexpected error: %v", err)
	}
}
