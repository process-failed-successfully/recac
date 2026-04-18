package undo

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	IndexFile = "index.json"
	UndoDir   = ".recac/undo"
)

// OperationType defines the type of change recorded
type OperationType string

const (
	OpModify OperationType = "modify"
	OpCreate OperationType = "create"
)

// FileChange represents a change to a single file
type FileChange struct {
	Path       string        `json:"path"`
	Type       OperationType `json:"type"`
	BackupPath string        `json:"backup_path,omitempty"` // Empty if OpCreate
}

// Operation represents a group of file changes (atomic undo unit)
type Operation struct {
	ID        string       `json:"id"`
	Timestamp time.Time    `json:"timestamp"`
	Changes   []FileChange `json:"changes"`
}

// Manager handles undo operations
type Manager struct {
	RootDir string
}

// NewManager creates a new Undo Manager rooted at the project directory
func NewManager(rootDir string) *Manager {
	return &Manager{
		RootDir: rootDir,
	}
}

// Capture backs up the given files and returns an Operation ID.
// It detects if files exist (modify) or not (create).
func (m *Manager) Capture(paths ...string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}

	undoBase := filepath.Join(m.RootDir, UndoDir)
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	backupDir := filepath.Join(undoBase, id)

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	var changes []FileChange

	for _, p := range paths {
		absPath := p
		if !filepath.IsAbs(p) {
			absPath = filepath.Join(m.RootDir, p)
		}

		info, err := os.Stat(absPath)
		if os.IsNotExist(err) {
			// File doesn't exist yet -> Create operation
			// No backup needed, just record that we created it so restore can delete it.
			changes = append(changes, FileChange{
				Path: p, // Store relative or provided path
				Type: OpCreate,
			})
			continue
		}
		if err != nil {
			return "", fmt.Errorf("failed to stat file %s: %w", p, err)
		}
		if info.IsDir() {
			continue // Skip directories for now
		}

		// File exists -> Modify operation -> Backup needed
		relPath, err := filepath.Rel(m.RootDir, absPath)
		if err != nil {
			relPath = filepath.Base(absPath) // Fallback
		}
		// Safe filename for backup
		backupName := filepath.Base(relPath)
		if !filepath.IsLocal(backupName) || backupName == "." || backupName == ".." {
			return "", fmt.Errorf("invalid path for backup: path traversal detected in %s", p)
		}

		backupPath := filepath.Join(backupDir, backupName)

		if err := copyFile(absPath, backupPath); err != nil {
			return "", fmt.Errorf("failed to backup file %s: %w", p, err)
		}

		changes = append(changes, FileChange{
			Path:       p,
			Type:       OpModify,
			BackupPath: filepath.Join(id, backupName), // Store relative to UndoDir
		})
	}

	op := Operation{
		ID:        id,
		Timestamp: time.Now(),
		Changes:   changes,
	}

	if err := m.appendHistory(op); err != nil {
		return "", fmt.Errorf("failed to save history: %w", err)
	}

	return id, nil
}

// Restore reverts the changes associated with the given Operation ID.
func (m *Manager) Restore(opID string) error {
	ops, err := m.List()
	if err != nil {
		return err
	}

	var op *Operation
	for _, o := range ops {
		if o.ID == opID {
			op = &o
			break
		}
	}
	if op == nil {
		return fmt.Errorf("operation %s not found", opID)
	}

	// Reverse changes to restore
	// For Modify: Copy backup back to original path
	// For Create: Delete the file
	undoBase := filepath.Join(m.RootDir, UndoDir)

	for _, change := range op.Changes {
		absPath := change.Path
		if !filepath.IsAbs(change.Path) {
			absPath = filepath.Join(m.RootDir, change.Path)
		}

		switch change.Type {
		case OpModify:
			backupFullPath := filepath.Join(undoBase, change.BackupPath)
			if err := copyFile(backupFullPath, absPath); err != nil {
				return fmt.Errorf("failed to restore file %s: %w", change.Path, err)
			}
		case OpCreate:
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete created file %s: %w", change.Path, err)
			}
		}
	}

	// Remove operation from history (optional, or mark as undone?)
	// For simple undo stack, we remove it.
	return m.removeHistory(opID)
}

// List returns the history of operations, sorted by most recent first.
func (m *Manager) List() ([]Operation, error) {
	indexPath := filepath.Join(m.RootDir, UndoDir, IndexFile)
	data, err := os.ReadFile(indexPath)
	if os.IsNotExist(err) {
		return []Operation{}, nil
	}
	if err != nil {
		return nil, err
	}

	var ops []Operation
	if err := json.Unmarshal(data, &ops); err != nil {
		return nil, err
	}

	// Sort recent first
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].Timestamp.After(ops[j].Timestamp)
	})

	return ops, nil
}

func (m *Manager) appendHistory(op Operation) error {
	// We read existing history from disk to ensure we don't overwrite if not cached,
	// but currently List() reads from disk.
	ops, err := m.List()
	if err != nil {
		return err
	}

	// List returns sorted recent-first. We just append the new one.
	// But JSON order doesn't strictly matter if we sort on read.
	// However, usually appendHistory implies adding to a list.
	ops = append(ops, op)
	return m.saveHistory(ops)
}

func (m *Manager) removeHistory(opID string) error {
	ops, err := m.List()
	if err != nil {
		return err
	}
	var newOps []Operation
	for _, o := range ops {
		if o.ID != opID {
			newOps = append(newOps, o)
		}
	}
	return m.saveHistory(newOps)
}

func (m *Manager) saveHistory(ops []Operation) error {
	undoDir := filepath.Join(m.RootDir, UndoDir)
	if err := os.MkdirAll(undoDir, 0755); err != nil {
		return err
	}

	indexPath := filepath.Join(undoDir, IndexFile)
	data, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, data, 0644)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
