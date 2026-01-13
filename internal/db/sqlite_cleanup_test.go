package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteStore_Cleanup(t *testing.T) {
	store := setupTestStore(t)
	projectID := "test-project"

	// 1. Create an expired lock
	_, err := store.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES (?, ?, ?, ?)`,
		projectID, "/expired/path", "agent-1", time.Now().Add(-1*time.Minute))
	require.NoError(t, err)

	// 2. Create a recent lock
	_, err = store.db.Exec(`INSERT INTO file_locks (project_id, path, agent_id, expires_at) VALUES (?, ?, ?, ?)`,
		projectID, "/recent/path", "agent-2", time.Now().Add(1*time.Minute))
	require.NoError(t, err)

	// 3. Create old and new signals
	store.SetSignal(projectID, "OLD_SIGNAL", "value")
	store.db.Exec(`UPDATE signals SET created_at = ? WHERE key = 'OLD_SIGNAL'`, time.Now().Add(-2*24*time.Hour))
	store.SetSignal(projectID, "NEW_SIGNAL", "value")
	store.SetSignal(projectID, "QA_PASSED", "true") // Critical signal

	// 4. Create observations
	for i := 0; i < 10; i++ {
		store.SaveObservation(projectID, "agent", fmt.Sprintf("obs-%d", i))
	}
	// Manually set one observation to be very old
	store.db.Exec(`UPDATE observations SET created_at = ? WHERE id = 1`, time.Now().Add(-30*24*time.Hour))


	// 5. Run Cleanup
	err = store.Cleanup()
	require.NoError(t, err)

	// 6. Verify locks
	locks, err := store.GetActiveLocks(projectID)
	require.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, "/recent/path", locks[0].Path)

	// 7. Verify signals
	val, err := store.GetSignal(projectID, "OLD_SIGNAL")
	require.NoError(t, err)
	assert.Equal(t, "", val)

	val, err = store.GetSignal(projectID, "NEW_SIGNAL")
	require.NoError(t, err)
	assert.Equal(t, "value", val)

	val, err = store.GetSignal(projectID, "QA_PASSED")
	require.NoError(t, err)
	assert.Equal(t, "true", val)


	// 8. Verify observations (this is tricky as we don't know the exact number, but it should be 10)
	history, err := store.QueryHistory(projectID, 20)
	require.NoError(t, err)
	assert.Len(t, history, 10)
}
