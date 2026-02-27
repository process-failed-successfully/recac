package flashcards

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStore_Operations(t *testing.T) {
	// Setup temporary directory
	tempDir, err := os.MkdirTemp("", "flashcards-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storePath := filepath.Join(tempDir, "flashcards.json")
	store := NewFileStore(storePath)

	// 1. Test Add
	card1 := NewFlashcard("Q1", "A1", "ctx1", "topic1")
	store.Add(card1)

	cards := store.List()
	assert.Len(t, cards, 1)
	// Map iteration order is random, but we only have 1 card
	assert.Equal(t, card1.ID, cards[0].ID)

	// 2. Test Save & Load
	err = store.Save()
	require.NoError(t, err)

	// Create new store instance to verify persistence
	store2 := NewFileStore(storePath)
	err = store2.Load()
	require.NoError(t, err)

	cards2 := store2.List()
	assert.Len(t, cards2, 1)
	assert.Equal(t, card1.ID, cards2[0].ID)
	assert.Equal(t, card1.Question, cards2[0].Question)

	// 3. Test Update
	card1.Answer = "Updated A1"
	store.Update(card1)

	// Verify update in memory
	cards = store.List()
	assert.Equal(t, "Updated A1", cards[0].Answer)

	// 4. Test GetDue
	// card1 is new, so it should be due
	due := store.GetDue()
	assert.Len(t, due, 1)
	assert.Equal(t, card1.ID, due[0].ID)

	// Create a card due in the future
	card2 := NewFlashcard("Q2", "A2", "", "")
	card2.State = StateReview
	card2.DueDate = time.Now().Add(24 * time.Hour)
	store.Add(card2)

	due = store.GetDue()
	assert.Len(t, due, 1) // Only card1 should be due
	assert.Equal(t, card1.ID, due[0].ID)

	// Create a card due in the past
	card3 := NewFlashcard("Q3", "A3", "", "")
	card3.State = StateReview
	card3.DueDate = time.Now().Add(-24 * time.Hour)
	store.Add(card3)

	due = store.GetDue()
	assert.Len(t, due, 2) // card1 and card3

	// Check IDs
	ids := make(map[string]bool)
	for _, c := range due {
		ids[c.ID] = true
	}
	assert.True(t, ids[card1.ID])
	assert.True(t, ids[card3.ID])
	assert.False(t, ids[card2.ID])

	// 5. Test Delete
	store.Delete(card1.ID)
	cards = store.List()
	assert.Len(t, cards, 2) // card2 and card3 remain

	store.Delete(card2.ID)
	store.Delete(card3.ID)
	cards = store.List()
	assert.Empty(t, cards)

	// Save empty state
	err = store.Save()
	require.NoError(t, err)

	store3 := NewFileStore(storePath)
	err = store3.Load()
	require.NoError(t, err)
	assert.Empty(t, store3.List())
}

func TestFileStore_Load_NoFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "flashcards-test-nofile")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storePath := filepath.Join(tempDir, "nonexistent.json")
	store := NewFileStore(storePath)

	// Should not error, just initialize empty
	err = store.Load()
	require.NoError(t, err)
	assert.Empty(t, store.List())
}

func TestDefaultStorePath(t *testing.T) {
	path, err := DefaultStorePath()
	require.NoError(t, err)
	assert.Contains(t, path, ".recac")
	assert.Contains(t, path, "flashcards.json")
}
