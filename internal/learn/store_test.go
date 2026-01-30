package learn

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore(t *testing.T) {
	tmp, err := os.MkdirTemp("", "learn-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	store := NewStore(tmp)

	// Load empty
	cards, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 0 {
		t.Errorf("Expected 0 cards, got %d", len(cards))
	}

	// Save
	newCards := []Flashcard{
		{ID: "1", Question: "Q1", Answer: "A1"},
	}
	if err := store.Save(newCards); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	path := filepath.Join(tmp, ".recac", "flashcards.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("File not created")
	}

	// Load again
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Errorf("Expected 1 card, got %d", len(loaded))
	}
	if loaded[0].Question != "Q1" {
		t.Errorf("Expected Q1, got %s", loaded[0].Question)
	}
}
