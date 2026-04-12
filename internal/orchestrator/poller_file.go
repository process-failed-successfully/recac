package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FilePoller reads work items from a local JSON file or a pipeline YAML file.
type FilePoller struct {
	path      string
	processed map[string]bool
	mu        sync.Mutex
}

func NewFilePoller(path string) *FilePoller {
	return &FilePoller{
		path:      path,
		processed: make(map[string]bool),
	}
}

func (p *FilePoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := os.Stat(p.path); os.IsNotExist(err) {
		logger.Warn("[FilePoller] Work file not found", "path", p.path)
		return nil, nil // No work file found yet
	}

	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read work file: %w", err)
	}

	var items []WorkItem
	ext := filepath.Ext(p.path)

	if strings.EqualFold(ext, ".yaml") || strings.EqualFold(ext, ".yml") {
		runID := "stable"
		parsedItems, err := ParsePipelineToWorkItemsWithRunID(data, "", nil, runID, filepath.Dir(p.path))
		if err != nil {
			return nil, fmt.Errorf("failed to parse pipeline file: %w", err)
		}
		items = parsedItems
	} else {
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("failed to unmarshal work items: %w", err)
		}
	}

	// Filter out already claimed items
	var newItems []WorkItem
	for _, item := range items {
		if !p.processed[item.ID] {
			newItems = append(newItems, item)
			p.processed[item.ID] = true
		}
	}

	return newItems, nil
}

func (p *FilePoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// No-op for file poller usually, but could log
	fmt.Printf("[FilePoller] Item %s status updated to %s: %s\n", item.ID, status, comment)
	return nil
}

func (p *FilePoller) Ping(ctx context.Context) error {
	// Check if file exists or directory exists
	info, err := os.Stat(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			// If file doesn't exist, check parent dir
			// Actually, just returning error is fine, the file SHOULD exist or be creatable.
			return fmt.Errorf("work file not found: %w", err)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("watch path is a directory: %s", p.path)
	}
	return nil
}
