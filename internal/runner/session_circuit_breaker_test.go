package runner

import (
	"recac/internal/notify"
	"recac/internal/telemetry"
	"strings"
	"testing"
)

func TestSession_CheckNoOpBreaker(t *testing.T) {
	s := &Session{
		Notifier: notify.NewManager(func(string, ...interface{}) {}),
		Logger:   telemetry.NewLogger(true, "", false),
	}

	// 1. One No-Op
	err := s.checkNoOpBreaker("")
	if err != nil {
		t.Errorf("Expected nil error for 1st no-op, got %v", err)
	}
	if s.NoOpCount != 1 {
		t.Errorf("Expected NoOpCount 1, got %d", s.NoOpCount)
	}

	// 2. Two No-Ops
	err = s.checkNoOpBreaker("")
	if err != nil {
		t.Errorf("Expected nil error for 2nd no-op, got %v", err)
	}
	if s.NoOpCount != 2 {
		t.Errorf("Expected NoOpCount 2, got %d", s.NoOpCount)
	}

	// 3. Valid Op resets
	err = s.checkNoOpBreaker("some output")
	if err != nil {
		t.Errorf("Expected nil error for valid op, got %v", err)
	}
	if s.NoOpCount != 0 {
		t.Errorf("Expected NoOpCount 0 after reset, got %d", s.NoOpCount)
	}

	// 4. Three consecutive No-Ops
	s.checkNoOpBreaker("")
	s.checkNoOpBreaker("")
	err = s.checkNoOpBreaker("")
	if err == nil {
		t.Error("Expected error for 3rd consecutive no-op")
	}
	if s.NoOpCount != 3 {
		t.Errorf("Expected NoOpCount 3, got %d", s.NoOpCount)
	}
}

func TestSession_CheckStalledBreaker(t *testing.T) {
	workspace := t.TempDir()

	// Create Mock DB Store to check signals
	// Since we don't want to spin up SQLite for a unit test unless needed,
	// checking `createSignal` might require a mock DB.
	// However, `checkStalledBreaker` calls `s.createSignal`.
	s := &Session{
		Workspace:        workspace,
		ManagerFrequency: 5,
		LastFeatureCount: 0,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	// 1. Initial Progress
	err := s.checkStalledBreaker("Agent", 5)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if s.StalledCount != 0 {
		t.Errorf("Expected StalledCount 0 after progress, got %d", s.StalledCount)
	}
	if s.LastFeatureCount != 5 {
		t.Errorf("Expected LastFeatureCount 5, got %d", s.LastFeatureCount)
	}

	// 2. Stall for 5 iterations -> Trigger Manager
	for i := 0; i < 5; i++ {
		s.checkStalledBreaker("Agent", 5)
	}
	if s.StalledCount != 5 {
		t.Errorf("Expected StalledCount 5, got %d", s.StalledCount)
	}

	// 3. Fast forward to 15
	s.StalledCount = 14
	err = s.checkStalledBreaker("Agent", 5)
	if err != nil {
		t.Fatalf("Unexpected error at 15: %v", err)
	}

	// 4. Trip at 16 (since 16 >= 3 * 5)
	err = s.checkStalledBreaker("Agent", 5)
	if err == nil || !strings.Contains(err.Error(), "CIRCUIT BREAKER TRIPPED") {
		t.Errorf("Expected circuit breaker trip at 16, got %v", err)
	}
}

func TestSession_CheckStalledBreaker_ManagerReset(t *testing.T) {
	s := &Session{
		ManagerFrequency: 5,
		LastFeatureCount: 0,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	// 1. Stall for 4 iterations
	for i := 0; i < 4; i++ {
		s.checkStalledBreaker("Agent", 0)
	}
	if s.StalledCount != 4 {
		t.Errorf("Expected StalledCount 4, got %d", s.StalledCount)
	}

	// 2. Manager iteration
	err := s.checkStalledBreaker("Manager", 0)
	if err != nil {
		t.Fatalf("Unexpected error on Manager call: %v", err)
	}

	if s.StalledCount != 0 {
		t.Errorf("Expected StalledCount to be reset to 0 by Manager, got %d", s.StalledCount)
	}
}
