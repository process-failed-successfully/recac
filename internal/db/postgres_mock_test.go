package db

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func withMockStore(t *testing.T, fn func(*PostgresStore, sqlmock.Sqlmock)) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}
	fn(store, mock)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPostgresStore_Mocked(t *testing.T) {
	projectID := "test-project"
	agentID := "test-agent"

	// --- Observation Tests ---
	t.Run("SaveObservation Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("INSERT INTO observations").
				WithArgs(projectID, agentID, "content").
				WillReturnResult(sqlmock.NewResult(1, 1))

			err := store.SaveObservation(projectID, agentID, "content")
			assert.NoError(t, err)
		})
	})

	t.Run("SaveObservation Error", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("INSERT INTO observations").
				WithArgs(projectID, agentID, "content").
				WillReturnError(errors.New("insert error"))

			err := store.SaveObservation(projectID, agentID, "content")
			assert.Error(t, err)
		})
	})

	t.Run("QueryHistory Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			rows := sqlmock.NewRows([]string{"id", "agent_id", "content", "created_at"}).
				AddRow(1, agentID, "content", time.Now())

			mock.ExpectQuery("SELECT id, agent_id, content, created_at FROM observations").
				WithArgs(projectID, 10).
				WillReturnRows(rows)

			obs, err := store.QueryHistory(projectID, 10)
			assert.NoError(t, err)
			assert.Len(t, obs, 1)
			assert.Equal(t, "content", obs[0].Content)
		})
	})

	t.Run("QueryHistory Error", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectQuery("SELECT id, agent_id, content, created_at FROM observations").
				WithArgs(projectID, 10).
				WillReturnError(errors.New("query error"))

			_, err := store.QueryHistory(projectID, 10)
			assert.Error(t, err)
		})
	})

	// --- Signal Tests ---
	t.Run("SetSignal Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("INSERT INTO signals").
				WithArgs(projectID, "key", "value").
				WillReturnResult(sqlmock.NewResult(1, 1))

			err := store.SetSignal(projectID, "key", "value")
			assert.NoError(t, err)
		})
	})

	t.Run("SetSignal Error", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("INSERT INTO signals").
				WithArgs(projectID, "key", "value").
				WillReturnError(errors.New("exec error"))

			err := store.SetSignal(projectID, "key", "value")
			assert.Error(t, err)
		})
	})

	t.Run("GetSignal Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			rows := sqlmock.NewRows([]string{"value"}).AddRow("value")
			mock.ExpectQuery("SELECT value FROM signals").
				WithArgs(projectID, "key").
				WillReturnRows(rows)

			val, err := store.GetSignal(projectID, "key")
			assert.NoError(t, err)
			assert.Equal(t, "value", val)
		})
	})

	t.Run("GetSignal NotFound", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectQuery("SELECT value FROM signals").
				WithArgs(projectID, "key").
				WillReturnError(sql.ErrNoRows)

			val, err := store.GetSignal(projectID, "key")
			assert.NoError(t, err) // Should handle ErrNoRows gracefully
			assert.Equal(t, "", val)
		})
	})

	t.Run("DeleteSignal Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("DELETE FROM signals").
				WithArgs(projectID, "key").
				WillReturnResult(sqlmock.NewResult(1, 1))

			err := store.DeleteSignal(projectID, "key")
			assert.NoError(t, err)
		})
	})

	// --- Features/Spec Tests ---
	t.Run("SaveFeatures Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("INSERT INTO project_features").
				WithArgs(projectID, "{}").
				WillReturnResult(sqlmock.NewResult(1, 1))

			err := store.SaveFeatures(projectID, "{}")
			assert.NoError(t, err)
		})
	})

	t.Run("GetFeatures Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			rows := sqlmock.NewRows([]string{"content"}).AddRow("{}")
			mock.ExpectQuery("SELECT content FROM project_features").
				WithArgs(projectID).
				WillReturnRows(rows)

			val, err := store.GetFeatures(projectID)
			assert.NoError(t, err)
			assert.Equal(t, "{}", val)
		})
	})

	t.Run("GetFeatures NotFound", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectQuery("SELECT content FROM project_features").
				WithArgs(projectID).
				WillReturnError(sql.ErrNoRows)

			val, err := store.GetFeatures(projectID)
			assert.NoError(t, err)
			assert.Equal(t, "", val)
		})
	})

	t.Run("SaveSpec Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("INSERT INTO project_specs").
				WithArgs(projectID, "content").
				WillReturnResult(sqlmock.NewResult(1, 1))

			err := store.SaveSpec(projectID, "content")
			assert.NoError(t, err)
		})
	})

	t.Run("GetSpec Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			rows := sqlmock.NewRows([]string{"content"}).AddRow("content")
			mock.ExpectQuery("SELECT content FROM project_specs").
				WithArgs(projectID).
				WillReturnRows(rows)

			val, err := store.GetSpec(projectID)
			assert.NoError(t, err)
			assert.Equal(t, "content", val)
		})
	})

	t.Run("UpdateFeatureStatus Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectBegin()

			features := `{"features":[{"id":"fid","status":"pending"}]}`
			rows := sqlmock.NewRows([]string{"content"}).AddRow(features)

			mock.ExpectQuery("SELECT content FROM project_features").
				WithArgs(projectID).
				WillReturnRows(rows)

			mock.ExpectExec("UPDATE project_features").
				WithArgs(sqlmock.AnyArg(), projectID).
				WillReturnResult(sqlmock.NewResult(1, 1))

			mock.ExpectCommit()

			err := store.UpdateFeatureStatus(projectID, "fid", "completed", true)
			assert.NoError(t, err)
		})
	})

	t.Run("UpdateFeatureStatus Rollback on Read Error", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT content FROM project_features").
				WithArgs(projectID).
				WillReturnError(errors.New("read error"))
			mock.ExpectRollback()

			err := store.UpdateFeatureStatus(projectID, "fid", "completed", true)
			assert.Error(t, err)
		})
	})

	// --- Locking Tests ---
	t.Run("AcquireLock Success (New)", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			// 1. Check if lock exists (No Rows)
			mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").
				WithArgs(projectID, "path").
				WillReturnError(sql.ErrNoRows)

			// 2. Insert new lock
			mock.ExpectExec("INSERT INTO file_locks").
				WithArgs(projectID, "path", agentID, sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(1, 1))

			acquired, err := store.AcquireLock(projectID, "path", agentID, time.Second)
			assert.NoError(t, err)
			assert.True(t, acquired)
		})
	})

	t.Run("AcquireLock Success (Renew)", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			// 1. Check if lock exists (Exists, owned by us, valid)
			rows := sqlmock.NewRows([]string{"agent_id", "expires_at"}).
				AddRow(agentID, time.Now().Add(time.Minute))

			mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").
				WithArgs(projectID, "path").
				WillReturnRows(rows)

			// 2. Update existing lock
			mock.ExpectExec("UPDATE file_locks").
				WithArgs(sqlmock.AnyArg(), projectID, "path").
				WillReturnResult(sqlmock.NewResult(1, 1))

			acquired, err := store.AcquireLock(projectID, "path", agentID, time.Second)
			assert.NoError(t, err)
			assert.True(t, acquired)
		})
	})

	t.Run("AcquireLock Success (Hijack Expired)", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			// 1. Check if lock exists (Exists, expired)
			rows := sqlmock.NewRows([]string{"agent_id", "expires_at"}).
				AddRow("other", time.Now().Add(-time.Minute))

			mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").
				WithArgs(projectID, "path").
				WillReturnRows(rows)

			// 2. Update (hijack)
			mock.ExpectExec("UPDATE file_locks").
				WithArgs(agentID, sqlmock.AnyArg(), projectID, "path").
				WillReturnResult(sqlmock.NewResult(1, 1))

			acquired, err := store.AcquireLock(projectID, "path", agentID, time.Second)
			assert.NoError(t, err)
			assert.True(t, acquired)
		})
	})

	t.Run("AcquireLock Fail (Locked by other)", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			// 1. Check if lock exists (Exists, owned by other, valid)
			// We expect this to happen repeatedly until timeout.
			// Since AcquireLock has a polling loop, we need to match multiple calls.
			// However, sqlmock doesn't support "Any number of times" easily without distinct setup.
			// We'll use a short timeout and expect at least one query.

			rows := sqlmock.NewRows([]string{"agent_id", "expires_at"}).
				AddRow("other", time.Now().Add(time.Minute))

			// Match first query
			mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").
				WithArgs(projectID, "path").
				WillReturnRows(rows)

			// Because of the loop and sleep, it might query again.
			// To avoid flakiness, we use 0 timeout. The code checks timeout after first query.
			// Since time.Since(start) >= 0 is always true, it should return immediately after first check.

			acquired, err := store.AcquireLock(projectID, "path", agentID, 0)
			assert.NoError(t, err)
			assert.False(t, acquired)
		})
	})

	t.Run("ReleaseLock Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("DELETE FROM file_locks").
				WithArgs(projectID, "path", agentID).
				WillReturnResult(sqlmock.NewResult(1, 1))

			err := store.ReleaseLock(projectID, "path", agentID)
			assert.NoError(t, err)
		})
	})

	t.Run("ReleaseLock Manager", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("DELETE FROM file_locks").
				WithArgs(projectID, "path").
				WillReturnResult(sqlmock.NewResult(1, 1))

			err := store.ReleaseLock(projectID, "path", "MANAGER")
			assert.NoError(t, err)
		})
	})

	t.Run("ReleaseAllLocks Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("DELETE FROM file_locks").
				WithArgs(projectID, agentID).
				WillReturnResult(sqlmock.NewResult(1, 1))

			err := store.ReleaseAllLocks(projectID, agentID)
			assert.NoError(t, err)
		})
	})

	t.Run("GetActiveLocks Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			rows := sqlmock.NewRows([]string{"path", "agent_id", "expires_at"}).
				AddRow("path1", agentID, time.Now().Add(time.Minute))

			mock.ExpectQuery("SELECT path, agent_id, expires_at FROM file_locks").
				WithArgs(sqlmock.AnyArg(), projectID).
				WillReturnRows(rows)

			locks, err := store.GetActiveLocks(projectID)
			assert.NoError(t, err)
			assert.Len(t, locks, 1)
		})
	})

	// --- Cleanup Tests ---
	t.Run("Cleanup Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			mock.ExpectExec("DELETE FROM file_locks").
				WillReturnResult(sqlmock.NewResult(10, 10))

			mock.ExpectExec("DELETE FROM signals").
				WillReturnResult(sqlmock.NewResult(5, 5))

			mock.ExpectExec("DELETE FROM observations").
				WillReturnResult(sqlmock.NewResult(100, 100))

			err := store.Cleanup()
			assert.NoError(t, err)
		})
	})

	// --- Migration Test ---
	t.Run("Migrate Success", func(t *testing.T) {
		withMockStore(t, func(store *PostgresStore, mock sqlmock.Sqlmock) {
			// Expect initial creation queries
			for i := 0; i < 6; i++ {
				mock.ExpectExec("CREATE (TABLE|INDEX)").WillReturnResult(sqlmock.NewResult(0, 0))
			}

			// Expect fine-grained fixes (ALTER TABLE etc)
			// There are 12 execs in step 2 of migrate
			for i := 0; i < 12; i++ {
				mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
			}

			err := store.migrate()
			assert.NoError(t, err)
		})
	})
}
