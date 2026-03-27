package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// FileDirPoller reads work items from individual JSON files or YAML pipelines in a directory.
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
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if entry.IsDir() || (ext != ".json" && ext != ".yaml" && ext != ".yml") {
			continue
		}

		path := filepath.Join(p.watchDir, entry.Name())
		logger.Info("[FileDirPoller] Found work file", "path", path)

		data, err := os.ReadFile(path)
		if err != nil {
			logger.Error("[FileDirPoller] Failed to read work file", "path", path, "error", err)
			continue
		}

		var fileItems []WorkItem
		if ext == ".yaml" || ext == ".yml" {
			runID := "stable"
			parsedItems, err := ParsePipelineToWorkItemsWithRunID(data, "", nil, runID, filepath.Dir(path))
			if err != nil {
				logger.Error("[FileDirPoller] Failed to parse pipeline file", "path", path, "error", err)
				continue
			}
			fileItems = parsedItems
		} else {
			var item WorkItem
			if err := json.Unmarshal(data, &item); err != nil {
				logger.Error("[FileDirPoller] Failed to unmarshal work item", "path", path, "error", err)
				continue
			}
			fileItems = []WorkItem{item}
		}

		// Move the file to the processed directory to prevent re-reading
		processedPath := filepath.Join(p.processedDir, entry.Name())
		if err := os.Rename(path, processedPath); err != nil {
			logger.Error("[FileDirPoller] Failed to move processed file", "from", path, "to", processedPath, "error", err)
			continue
		}

		items = append(items, fileItems...)
	}

	return items, nil
}

func (p *FileDirPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// No-op for file poller usually, but could log
	fmt.Printf("[FileDirPoller] Item %s status updated to %s: %s\n", item.ID, status, comment)
	return nil
}

func (p *FileDirPoller) Ping(ctx context.Context) error {
	// Check if directory exists and is readable
	info, err := os.Stat(p.watchDir)
	if err != nil {
		return fmt.Errorf("watch directory access failed: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("watch path is not a directory: %s", p.watchDir)
	}
	return nil
}
