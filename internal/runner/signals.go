package runner

import (
	"os"
	"path/filepath"
)

// checkCompletion checks if the project is marked as completed.
func (s *Session) checkCompletion() bool {
	return s.hasSignal("COMPLETED")
}

// hasSignal checks if a signal exists in the DB or filesystem (legacy/test).
func (s *Session) hasSignal(name string) bool {
	// 1. Check DB (Modern Source)
	if s.DBStore != nil {
		val, err := s.DBStore.GetSignal(s.Project, name)
		if err == nil && val == "true" {
			return true
		}
	}

	// 2. Fallback: Check Filesystem (Legacy/Test Source)
	// We check this if DB is nil (Test) OR if signal missing in DB (Migration/Legacy)
	path := filepath.Join(s.Workspace, "."+name) // Hidden file preferred
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try non-hidden (Legacy)
		path = filepath.Join(s.Workspace, name)
	}

	if _, err := os.Stat(path); err == nil {
		// Found file-based signal.

		// Security Check: Only migrate non-privileged signals from filesystem
		// UNLESS we are in a mode without DB (Testing)
		if s.DBStore != nil {
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

			// Migration Logic
			s.Logger.Info("migrating signal from filesystem to DB", "signal", name)
			if err := s.DBStore.SetSignal(s.Project, name, "true"); err != nil {
				s.Logger.Error("failed to migrate signal to DB", "signal", name, "error", err)
				return true // File exists, so logically signal is true
			}
			// Cleanup the file after migration
			os.Remove(path)
			return true
		} else {
			// No DB (Test/Local mode): Trust filesystem
			return true
		}
	}

	return false
}

// clearSignal removes a signal from the DB and filesystem.
func (s *Session) clearSignal(name string) {
	if s.DBStore != nil {
		s.DBStore.DeleteSignal(s.Project, name)
	}
	// Also ensure file is removed (redundancy/legacy)
	os.Remove(filepath.Join(s.Workspace, "."+name))
	os.Remove(filepath.Join(s.Workspace, name))
}

// createSignal creates a signal in the DB (or filesystem if DB unavailable).
func (s *Session) createSignal(name string) error {
	if s.DBStore != nil {
		if err := s.DBStore.SetSignal(s.Project, name, "true"); err != nil {
			return err
		}
	} else {
		// Fallback: Create file (hidden)
		if err := os.WriteFile(filepath.Join(s.Workspace, "."+name), []byte("true"), 0644); err != nil {
			return err
		}
	}
	s.Logger.Info("created signal", "signal", name)
	return nil
}
