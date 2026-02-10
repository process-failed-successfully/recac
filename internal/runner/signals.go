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
	// 1. Check DB (Modern Source)
	if s.DBStore != nil {
		val, err := s.DBStore.GetSignal(s.Project, name)
		if err == nil && val == "true" {
			s.Logger.Debug("signal found in DB", "signal", name)
			return true
		} else if err != nil {
			// Verbose logging for debugging E2E failures
			// Only log error if it's not just "not found" (depending on store impl, empty string usually means not found)
			if val != "" {
				s.Logger.Debug("signal check failed in DB", "signal", name, "error", err)
			}
		}
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

		// Allow privileged signals from file ONLY if DBStore is nil (Legacy/Test Mode)
		// If DBStore is present, we enforce security to prevent agent bypass.
		if privilegedSignals[name] && s.DBStore != nil {
			s.Logger.Warn("ignoring filesystem-based privileged signal (must come from DB)", "signal", name)
			return false
		}

		if s.DBStore != nil {
			s.Logger.Info("migrating signal from filesystem to DB", "signal", name)
			if err := s.DBStore.SetSignal(s.Project, name, "true"); err != nil {
				s.Logger.Error("failed to migrate signal to DB", "signal", name, "error", err)
				return true // File exists, so logically signal is true even if migration failed
			}
			// Cleanup the file after migration
			os.Remove(path)
		} else {
			// No DBStore, accept the filesystem signal (Test/Legacy Mode)
			s.Logger.Debug("signal found in filesystem", "signal", name)
		}
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
		// If no DB, fallback to filesystem (useful for tests/legacy)
		path := filepath.Join(s.Workspace, name)
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create signal file: %w", err)
		}
		file.Close()
		s.Logger.Info("created signal (file)", "signal", name)
		return nil
	}
	if err := s.DBStore.SetSignal(s.Project, name, "true"); err != nil {
		return err
	}
	s.Logger.Info("created signal (db)", "signal", name)
	return nil
}
