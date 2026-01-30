package learn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(workspace string) *Store {
	return &Store{
		path: filepath.Join(workspace, ".recac", "flashcards.json"),
	}
}

func (s *Store) Load() ([]Flashcard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return []Flashcard{}, nil
	}

	b, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	var cards []Flashcard
	if err := json.Unmarshal(b, &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func (s *Store) Save(cards []Flashcard) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0644)
}
