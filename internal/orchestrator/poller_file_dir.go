package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// FileDirPoller reads work items from individual JSON files in a directory.
type FileDirPoller struct {
	watchDir     string
	processedDir string
}

func NewFileDirPoller(watchDir string) (*FileDirPoller, error) {
	processedDir := filepath.Join(watchDir, "processed")
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create processed directory: %w", err)
	}

	return &FileDirPoller{
		watchDir:     watchDir,
		processedDir: processedDir,
	}, nil
}

func (p *FileDirPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	entries, err := os.ReadDir(p.watchDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read watch directory: %w", err)
	}

	var items []WorkItem
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(p.watchDir, entry.Name())
		logger.Info("[FileDirPoller] Found work file", "path", path)

		data, err := os.ReadFile(path)
		if err != nil {
			logger.Error("[FileDirPoller] Failed to read work file", "path", path, "error", err)
			continue
		}

		var item WorkItem
		if err := json.Unmarshal(data, &item); err != nil {
			logger.Error("[FileDirPoller] Failed to unmarshal work item", "path", path, "error", err)
			continue
		}

		items = append(items, item)

		// Move the file to the processed directory to prevent re-reading
		processedPath := filepath.Join(p.processedDir, entry.Name())
		if err := os.Rename(path, processedPath); err != nil {
			logger.Error("[FileDirPoller] Failed to move processed file", "from", path, "to", processedPath, "error", err)
			// If we can't move it, we can't process it, so we'll skip it for now.
			// This could lead to retries, which is desirable.
			items = items[:len(items)-1] // Remove the item we failed to move
		}
	}

	return items, nil
}

func (p *FileDirPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// No-op for file poller usually, but could log
	fmt.Printf("[FileDirPoller] Item %s status updated to %s: %s\n", item.ID, status, comment)
	return nil
}
