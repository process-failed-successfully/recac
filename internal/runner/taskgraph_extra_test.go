package runner

import (
	"recac/internal/db"
	"testing"
)

func TestLoadFromFeatures(t *testing.T) {
	tg := NewTaskGraph()

	// Create dummy features
	features := []db.Feature{
		{
			ID:          "feat-1",
			Category:    "core",
			Description: "Core Feature",
			Status:      "pending",
			Dependencies: db.FeatureDependencies{
				DependsOnIDs:        []string{},
				ExclusiveWritePaths: []string{"core/"},
			},
		},
		{
			ID:          "feat-2",
			Category:    "ui",
			Description: "UI Feature",
			Status:      "pending",
			Dependencies: db.FeatureDependencies{
				DependsOnIDs:        []string{"feat-1"},
				ExclusiveWritePaths: []string{"ui/"},
			},
		},
	}

	// Load
	err := tg.LoadFromFeatures(features)
	if err != nil {
		t.Fatalf("LoadFromFeatures failed: %v", err)
	}

	// Verify
	taskA, err := tg.GetTask("feat-1")
	if err != nil {
		t.Error("feat-1 not found")
	} else if taskA.Name != "Core Feature" {
		t.Errorf("Expected Core Feature, got %s", taskA.Name)
	}

	taskB, err := tg.GetTask("feat-2")
	if err != nil {
		t.Error("feat-2 not found")
	} else if len(taskB.Dependencies) != 1 || taskB.Dependencies[0] != "feat-1" {
		t.Errorf("Expected dependency on feat-1, got %v", taskB.Dependencies)
	}
}

func TestLoadFromFeatures_DowngradePrevention(t *testing.T) {
	tg := NewTaskGraph()

	// Initial features
	f1 := db.Feature{ID: "F1", Status: "pending"}
	tg.LoadFromFeatures([]db.Feature{f1})

	// Mark as InProgress manually
	tg.MarkTaskStatus("F1", TaskInProgress, nil)

	// Update with pending status from DB (should be ignored)
	f1.Status = "pending"
	tg.LoadFromFeatures([]db.Feature{f1})

	task, _ := tg.GetTask("F1")
	if task.Status != TaskInProgress {
		t.Errorf("Expected status InProgress to persist, got %s", task.Status)
	}

	// Update with failed status (should update)
	f1.Status = "failed"
	tg.LoadFromFeatures([]db.Feature{f1})

	task, _ = tg.GetTask("F1")
	if task.Status != TaskFailed {
		t.Errorf("Expected status Failed, got %s", task.Status)
	}

	// Update from Failed to Pending (should update, as failure might be retried?)
	// Code says:
	// if (existing.Status == TaskInProgress || existing.Status == TaskDone) && (newStatus == TaskPending || newStatus == TaskReady) { ... }
	// Failed is not InProgress or Done. So it should update.
	f1.Status = "pending"
	tg.LoadFromFeatures([]db.Feature{f1})
	task, _ = tg.GetTask("F1")
	if task.Status != TaskPending {
		t.Errorf("Expected status Pending, got %s", task.Status)
	}
}
