package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Store defines the interface for storing benchmark runs.
type Store interface {
	Save(run Run) error
	LoadLatest() (*Run, error)
	LoadAll() ([]Run, error)
}

// FileStore implements Store using a JSON file.
type FileStore struct {
	path string
}

func NewFileStore(path string) (*FileStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return &FileStore{path: path}, nil
}

func (s *FileStore) Save(run Run) error {
	runs, err := s.LoadAll()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	runs = append(runs, run)

	// Write back
	data, err := json.MarshalIndent(runs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal runs: %w", err)
	}

	return os.WriteFile(s.path, data, 0644)
}

func (s *FileStore) LoadAll() ([]Run, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Run{}, nil
		}
		return nil, err
	}

	var runs []Run
	if len(data) == 0 {
		return []Run{}, nil
	}

	if err := json.Unmarshal(data, &runs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal runs: %w", err)
	}

	// Sort by timestamp just in case
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Timestamp.Before(runs[j].Timestamp)
	})

	return runs, nil
}

func (s *FileStore) LoadLatest() (*Run, error) {
	runs, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[len(runs)-1], nil
}
