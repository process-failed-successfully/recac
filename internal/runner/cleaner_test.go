package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCleanerAgent_PathTraversal(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "recac-cleaner-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	workspace := filepath.Join(tmpDir, "workspace")
	err = os.Mkdir(workspace, 0755)
	require.NoError(t, err)

	targetFile := filepath.Join(tmpDir, "target_secret.txt")
	err = os.WriteFile(targetFile, []byte("secret"), 0644)
	require.NoError(t, err)

	// Create temp_files.txt with traversal path
	// ../target_secret.txt
	traversalPath := filepath.Join("..", "target_secret.txt")
	tempFilesList := filepath.Join(workspace, "temp_files.txt")
	err = os.WriteFile(tempFilesList, []byte(traversalPath), 0644)
	require.NoError(t, err)

	// Setup Session
	logger := telemetry.NewLogger(true, "", false)
	session := &Session{
		Workspace: workspace,
		Logger:    logger,
	}

	// Execute
	// We need to call runCleanerAgent. Since it's unexported, we can only call it if we are in package runner.
	err = session.runCleanerAgent(context.Background())
	require.NoError(t, err)

	// Verify
	_, err = os.Stat(targetFile)
	if os.IsNotExist(err) {
		t.Log("Vulnerability confirmed: target file outside workspace was deleted")
	} else {
        t.Log("Target file still exists (Safe?)")
    }
    // We expect it to FAIL if the vulnerability exists, for the purpose of demonstrating it.
    // But since I need to fix it, I'll write the test to FAIL if the file IS deleted.
	assert.NoError(t, err, "Target file should still exist")
    assert.FileExists(t, targetFile, "Target file should not be deleted")
}

func TestRunCleanerAgent_AbsolutePath(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "recac-cleaner-test-abs")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	workspace := filepath.Join(tmpDir, "workspace")
	err = os.Mkdir(workspace, 0755)
	require.NoError(t, err)

	targetFile := filepath.Join(tmpDir, "target_secret_abs.txt")
	err = os.WriteFile(targetFile, []byte("secret"), 0644)
	require.NoError(t, err)

	// Create temp_files.txt with absolute path
	tempFilesList := filepath.Join(workspace, "temp_files.txt")
	err = os.WriteFile(tempFilesList, []byte(targetFile), 0644)
	require.NoError(t, err)

	// Setup Session
	logger := telemetry.NewLogger(true, "", false)
	session := &Session{
		Workspace: workspace,
		Logger:    logger,
	}

	// Execute
	err = session.runCleanerAgent(context.Background())
	require.NoError(t, err)

	// Verify
	_, err = os.Stat(targetFile)
	if os.IsNotExist(err) {
		t.Log("Vulnerability confirmed: absolute path target file was deleted")
	}
    assert.FileExists(t, targetFile, "Target file should not be deleted")
}

func TestRunCleanerAgent_SymlinkTraversal(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "recac-cleaner-test-sym")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	workspace := filepath.Join(tmpDir, "workspace")
	err = os.Mkdir(workspace, 0755)
	require.NoError(t, err)

	// Create a secret directory outside workspace
	secretDir := filepath.Join(tmpDir, "secrets")
	err = os.Mkdir(secretDir, 0755)
	require.NoError(t, err)

	targetFile := filepath.Join(secretDir, "passwd")
	err = os.WriteFile(targetFile, []byte("secret_password"), 0644)
	require.NoError(t, err)

	// Create a symlink inside workspace pointing to secrets dir
	linkDir := filepath.Join(workspace, "link_to_secrets")
	err = os.Symlink(secretDir, linkDir)
	require.NoError(t, err)

	// Create temp_files.txt requesting to delete "link_to_secrets/passwd"
	// This path is lexically inside workspace, but physically outside.
	traversalPath := filepath.Join("link_to_secrets", "passwd")
	tempFilesList := filepath.Join(workspace, "temp_files.txt")
	err = os.WriteFile(tempFilesList, []byte(traversalPath), 0644)
	require.NoError(t, err)

	// Setup Session
	logger := telemetry.NewLogger(true, "", false)
	session := &Session{
		Workspace: workspace,
		Logger:    logger,
	}

	// Execute
	err = session.runCleanerAgent(context.Background())
	require.NoError(t, err)

	// Verify
	_, err = os.Stat(targetFile)
	if os.IsNotExist(err) {
		t.Log("Vulnerability confirmed: target file accessed via symlink was deleted")
	}
	assert.FileExists(t, targetFile, "Target file accessed via symlink should not be deleted")
}
