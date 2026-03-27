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
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// CronPoller implements the Poller interface and triggers tasks based on a cron schedule.
type CronPoller struct {
	mu           sync.Mutex
	schedule     cron.Schedule
	templatePath string
	templateData []byte
	isPipeline   bool
	nextRun      time.Time
}

// NewCronPoller creates a new CronPoller.
// scheduleStr follows standard cron syntax (e.g., "0 0 * * *").
// templatePath is the path to a JSON file containing the WorkItem template or a YAML pipeline.
func NewCronPoller(scheduleStr string, templatePath string) (*CronPoller, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(scheduleStr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron schedule '%s': %w", scheduleStr, err)
	}

	ext := strings.ToLower(filepath.Ext(templatePath))
	isPipeline := ext == ".yaml" || ext == ".yml"

	poller := &CronPoller{
		schedule:     schedule,
		templatePath: templatePath,
		isPipeline:   isPipeline,
		nextRun:      schedule.Next(time.Now()),
	}

	// Try to load the template immediately to validate it
	if err := poller.loadTemplate(); err != nil {
		return nil, fmt.Errorf("failed to load cron template from %s: %w", templatePath, err)
	}

	return poller, nil
}

func (p *CronPoller) loadTemplate() error {
	data, err := os.ReadFile(p.templatePath)
	if err != nil {
		return err
	}

	if !p.isPipeline {
		var item WorkItem
		if err := json.Unmarshal(data, &item); err != nil {
			return fmt.Errorf("invalid JSON in template file: %w", err)
		}
	} else {
		// Validate pipeline YAML
		_, err := ParsePipelineToWorkItemsWithRunID(data, "", nil, "validate", filepath.Dir(p.templatePath))
		if err != nil {
			return fmt.Errorf("invalid pipeline YAML in template file: %w", err)
		}
	}

	p.templateData = data
	return nil
}

// Poll checks if the scheduled time has been reached.
func (p *CronPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	// Not time yet
	if now.Before(p.nextRun) {
		return []WorkItem{}, nil
	}

	// Time to trigger!
	logger.Info("Cron schedule triggered", "scheduled_time", p.nextRun, "actual_time", now)

	// Reload template to pick up any changes
	if err := p.loadTemplate(); err != nil {
		logger.Error("Failed to load cron template, skipping this run", "error", err)
		// Update next run anyway so we don't get stuck in a tight error loop
		p.nextRun = p.schedule.Next(now)
		return []WorkItem{}, nil
	}

	var items []WorkItem
	runID := now.Format("20060102-150405")

	if p.isPipeline {
		parsedItems, err := ParsePipelineToWorkItemsWithRunID(p.templateData, "", nil, runID, filepath.Dir(p.templatePath))
		if err != nil {
			logger.Error("Failed to parse pipeline cron template", "error", err)
			p.nextRun = p.schedule.Next(now)
			return []WorkItem{}, nil
		}
		items = parsedItems
	} else {
		var item WorkItem
		if err := json.Unmarshal(p.templateData, &item); err != nil {
			logger.Error("Failed to unmarshal JSON cron template", "error", err)
			p.nextRun = p.schedule.Next(now)
			return []WorkItem{}, nil
		}

		// Assign a unique ID if one wasn't provided, or append a timestamp
		if item.ID == "" {
			item.ID = fmt.Sprintf("cron-%s", uuid.New().String()[:8])
		} else {
			item.ID = fmt.Sprintf("%s-%s", item.ID, runID)
		}
		items = []WorkItem{item}
	}

	// Compute next run time
	p.nextRun = p.schedule.Next(now)
	logger.Info("Next cron run scheduled", "next_run", p.nextRun)

	return items, nil
}

// UpdateStatus is a no-op for CronPoller, as tasks are fire-and-forget based on time.
func (p *CronPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	return nil // No upstream system to update
}

// Ping verifies that the template file is still accessible.
func (p *CronPoller) Ping(ctx context.Context) error {
	return p.loadTemplate()
}
