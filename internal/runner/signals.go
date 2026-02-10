package runner

import (
	"fmt"
	"os"
	"path/filepath"
)

// checkCompletion checks if the project is marked as completed.
func (s *Session) checkCompletion() bool {
	return s.hasSignal("COMPLETED")
}

// hasSignal checks if a signal exists in the DB or filesystem (legacy).
func (s *Session) hasSignal(name string) bool {
	if s.DBStore == nil {
		return false
	}

	// 1. Check DB (Modern Source)
	val, err := s.DBStore.GetSignal(s.Project, name)
	if err == nil && val == "true" {
		return true
	}

	// 2. Migration: Check Filesystem (Legacy Source)
	path := filepath.Join(s.Workspace, name)
	if _, err := os.Stat(path); err == nil {
		// Found file-based signal.
		// Security Check: Only migrate non-privileged signals from filesystem
		privilegedSignals := map[string]bool{
			"PROJECT_SIGNED_OFF": true,
			"QA_PASSED":          true,
			"COMPLETED":          true,
			"TRIGGER_QA":         true,
			"TRIGGER_MANAGER":    true,
		}

		if privilegedSignals[name] {
			s.Logger.Warn("ignoring filesystem-based privileged signal (must come from DB)", "signal", name)
			return false
		}

		s.Logger.Info("migrating signal from filesystem to DB", "signal", name)
		if err := s.DBStore.SetSignal(s.Project, name, "true"); err != nil {
			s.Logger.Error("failed to migrate signal to DB", "signal", name, "error", err)
			return true // File exists, so logically signal is true even if migration failed
		}
		// Cleanup the file after migration
		os.Remove(path)
		return true
	}

	return false
}

// clearSignal removes a signal from the DB and filesystem.
func (s *Session) clearSignal(name string) {
	if s.DBStore != nil {
		s.DBStore.DeleteSignal(s.Project, name)
	}
	// Also ensure file is removed (redundancy)
	path := filepath.Join(s.Workspace, name)
	os.Remove(path)
}

// createSignal creates a signal in the DB.
func (s *Session) createSignal(name string) error {
	if s.DBStore == nil {
		return fmt.Errorf("db store not initialized")
	}
	if err := s.DBStore.SetSignal(s.Project, name, "true"); err != nil {
		return err
	}
	s.Logger.Info("created signal", "signal", name)
	return nil
}
