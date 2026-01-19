package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQueryHistoryPerformance(t *testing.T) {
	// Create a temporary database file
	tmpDir, err := os.MkdirTemp("", "perf_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "perf.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Seed data
	projectID := "proj_perf"
	count := 20000 // Enough to make a scan noticeable

	t.Logf("Seeding %d observations...", count)

	// Direct access to db for faster seeding (since we are in package db)
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatal(err)
	}

	stmt, err := tx.Prepare("INSERT INTO observations (project_id, agent_id, content, created_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	baseTime := time.Now().Add(-time.Duration(count) * time.Minute)
	for i := 0; i < count; i++ {
		_, err := stmt.Exec(projectID, "agent1", fmt.Sprintf("content %d", i), baseTime.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	t.Log("Seeding complete. Measuring QueryHistory...")

	// Measure
	start := time.Now()
	// Querying a small limit vs a large table is where index helps (avoid sorting all 20k rows)
	results, err := store.QueryHistory(projectID, 50)
	if err != nil {
		t.Fatal(err)
	}
	duration := time.Since(start)

	if len(results) != 50 {
		t.Errorf("Expected 50 results, got %d", len(results))
	}

	t.Logf("QueryHistory (limit 50) took: %v", duration)
}
