package snapshot

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/git"
	"sort"
	"time"
)

type Snapshot struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	CommitSHA   string    `json:"commit_sha"`
}

type Manager struct {
	Workspace string
	Git       git.IClient
}

func NewManager(workspace string, gitClient git.IClient) *Manager {
	return &Manager{
		Workspace: workspace,
		Git:       gitClient,
	}
}

func (m *Manager) snapshotsDir() string {
	return filepath.Join(m.Workspace, ".recac", "snapshots")
}

func (m *Manager) Save(name, description string) error {
	// 1. Check if snapshot already exists
	snapDir := filepath.Join(m.snapshotsDir(), name)
	if _, err := os.Stat(snapDir); err == nil {
		return fmt.Errorf("snapshot '%s' already exists", name)
	}

	// 2. Get current commit
	sha, err := m.Git.CurrentCommitSHA(m.Workspace)
	if err != nil {
		return fmt.Errorf("failed to get current commit: %w", err)
	}

	// 3. Create git tag
	tagName := "snapshot/" + name
	if err := m.Git.Tag(m.Workspace, tagName); err != nil {
		// If tag exists but dir doesn't (inconsistent state), we fail.
		// User must resolve this (e.g. delete tag manually).
		return fmt.Errorf("failed to create git tag (maybe it exists?): %w", err)
	}

	// 4. Create snapshot dir
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// 5. Backup state files
	filesToBackup := []string{".agent_state.json", ".recac.db"}
	for _, f := range filesToBackup {
		src := filepath.Join(m.Workspace, f)
		dst := filepath.Join(snapDir, f)
		if err := copyFile(src, dst); err != nil {
			// Some files might not exist (e.g. fresh session)
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to backup %s: %w", f, err)
			}
		}
	}

	// 6. Save metadata
	meta := Snapshot{
		Name:        name,
		Description: description,
		Timestamp:   time.Now(),
		CommitSHA:   sha,
	}
	metaPath := filepath.Join(snapDir, "meta.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

func (m *Manager) Restore(name string) error {
	snapDir := filepath.Join(m.snapshotsDir(), name)
	if _, err := os.Stat(snapDir); os.IsNotExist(err) {
		return fmt.Errorf("snapshot '%s' not found", name)
	}

	// 1. Checkout tag
	tagName := "snapshot/" + name
	if err := m.Git.Checkout(m.Workspace, tagName); err != nil {
		return fmt.Errorf("failed to checkout tag %s: %w", tagName, err)
	}

	// 2. Restore state files
	filesToRestore := []string{".agent_state.json", ".recac.db"}
	for _, f := range filesToRestore {
		src := filepath.Join(snapDir, f)
		dst := filepath.Join(m.Workspace, f)
		if err := copyFile(src, dst); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to restore %s: %w", f, err)
			}
			// If backup didn't have it, remove it from workspace to match state
			os.Remove(dst)
		}
	}

	return nil
}

func (m *Manager) List() ([]Snapshot, error) {
	dir := m.snapshotsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Snapshot{}, nil
		}
		return nil, err
	}

	var snapshots []Snapshot
	for _, entry := range entries {
		if entry.IsDir() {
			metaPath := filepath.Join(dir, entry.Name(), "meta.json")
			data, err := os.ReadFile(metaPath)
			if err != nil {
				continue // Skip invalid snapshots
			}
			var s Snapshot
			if err := json.Unmarshal(data, &s); err == nil {
				snapshots = append(snapshots, s)
			}
		}
	}

	// Sort by timestamp desc
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	return snapshots, nil
}

func (m *Manager) Delete(name string) error {
	snapDir := filepath.Join(m.snapshotsDir(), name)
	if _, err := os.Stat(snapDir); os.IsNotExist(err) {
		return fmt.Errorf("snapshot '%s' not found", name)
	}

	// Remove dir
	if err := os.RemoveAll(snapDir); err != nil {
		return fmt.Errorf("failed to remove snapshot directory: %w", err)
	}

	// Remove tag
	tagName := "snapshot/" + name
	// We use direct exec because IClient doesn't support DeleteTag and we want to avoid interface churn.
	cmd := exec.Command("git", "-C", m.Workspace, "tag", "-d", tagName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete git tag: %v (output: %s)", err, out)
	}

	return nil
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
