package flashcards

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store defines the interface for persisting flashcards.
type Store interface {
	Load() error
	Save() error
	Add(card Flashcard)
	List() []Flashcard
	GetDue() []Flashcard
	Update(card Flashcard)
	Delete(id string)
}

// FileStore implements Store using a JSON file.
type FileStore struct {
	path  string
	cards map[string]Flashcard
	mu    sync.RWMutex
}

// NewFileStore creates a new FileStore at the given path.
func NewFileStore(path string) *FileStore {
	return &FileStore{
		path:  path,
		cards: make(map[string]Flashcard),
	}
}

// DefaultStorePath returns the default path for flashcards storage.
func DefaultStorePath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, ".recac", "flashcards.json"), nil
}

// Load reads the flashcards from the file.
func (s *FileStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.cards = make(map[string]Flashcard)
			return nil
		}
		return fmt.Errorf("failed to read flashcards file: %w", err)
	}

	var cardList []Flashcard
	if err := json.Unmarshal(data, &cardList); err != nil {
		return fmt.Errorf("failed to parse flashcards JSON: %w", err)
	}

	s.cards = make(map[string]Flashcard)
	for _, c := range cardList {
		s.cards[c.ID] = c
	}

	return nil
}

// Save writes the flashcards to the file.
func (s *FileStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Convert map to slice
	var cardList []Flashcard
	for _, c := range s.cards {
		cardList = append(cardList, c)
	}

	// Sort for deterministic output
	sort.Slice(cardList, func(i, j int) bool {
		return cardList[i].ID < cardList[j].ID
	})

	data, err := json.MarshalIndent(cardList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal flashcards: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write flashcards file: %w", err)
	}

	return nil
}

// Add inserts a new card.
func (s *FileStore) Add(card Flashcard) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cards[card.ID] = card
}

// List returns all cards.
func (s *FileStore) List() []Flashcard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []Flashcard
	for _, c := range s.cards {
		list = append(list, c)
	}
	return list
}

// GetDue returns cards that are due for review.
func (s *FileStore) GetDue() []Flashcard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var due []Flashcard
	now := time.Now()
	for _, c := range s.cards {
		if c.DueDate.Before(now) || c.State == StateNew || c.State == StateRelearning {
			due = append(due, c)
		}
	}
	return due
}

// Update modifies an existing card.
func (s *FileStore) Update(card Flashcard) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cards[card.ID] = card
}

// Delete removes a card.
func (s *FileStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cards, id)
}
