//go:build !windows

package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveSession_Error(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Skipping permission test as root")
	}

	sm, cleanup := setupSessionManager(t)
	defer cleanup()

	sessionName := "protected-session"
	session := &SessionState{Name: sessionName, Status: "completed", LogFile: filepath.Join(sm.sessionsDir, sessionName+".log")}
	err := sm.SaveSession(session)
	require.NoError(t, err)

	// Make directory read-only to prevent deletion of files inside
	// Note: Removing a file requires write permission on the PARENT directory.
	err = os.Chmod(sm.sessionsDir, 0500) // Read-execute only
	require.NoError(t, err)
	defer os.Chmod(sm.sessionsDir, 0700) // Restore for cleanup

	err = sm.RemoveSession(sessionName, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove session state file")
}
