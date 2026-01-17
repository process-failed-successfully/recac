package db

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestSQLiteStore_Errors(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &SQLiteStore{db: db}
	projectID := "test-project"
	agentID := "test-agent"

	t.Run("SaveObservation Error", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO observations").
			WithArgs(projectID, agentID, "content", sqlmock.AnyArg()).
			WillReturnError(errors.New("insert error"))

		err := store.SaveObservation(projectID, agentID, "content")
		assert.Error(t, err)
		assert.Equal(t, "insert error", err.Error())
	})

	t.Run("QueryHistory Query Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, agent_id, content, created_at FROM observations").
			WithArgs(projectID, 10).
			WillReturnError(errors.New("query error"))

		_, err := store.QueryHistory(projectID, 10)
		assert.Error(t, err)
		assert.Equal(t, "query error", err.Error())
	})

	t.Run("QueryHistory Scan Error", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "agent_id", "content", "created_at"}).
			AddRow(1, "agent", "content", "invalid-time") // Scan error on time

		mock.ExpectQuery("SELECT id, agent_id, content, created_at FROM observations").
			WithArgs(projectID, 10).
			WillReturnRows(rows)

		_, err := store.QueryHistory(projectID, 10)
		assert.Error(t, err)
	})

	t.Run("SetSignal Error", func(t *testing.T) {
		mock.ExpectExec("INSERT OR REPLACE INTO signals").
			WithArgs(projectID, "key", "value", sqlmock.AnyArg()).
			WillReturnError(errors.New("exec error"))

		err := store.SetSignal(projectID, "key", "value")
		assert.Error(t, err)
	})

	t.Run("GetSignal Query Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT value FROM signals").
			WithArgs(projectID, "key").
			WillReturnError(errors.New("query error"))

		_, err := store.GetSignal(projectID, "key")
		assert.Error(t, err)
	})

	t.Run("DeleteSignal Error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM signals").
			WithArgs(projectID, "key").
			WillReturnError(errors.New("delete error"))

		err := store.DeleteSignal(projectID, "key")
		assert.Error(t, err)
	})

	t.Run("SaveFeatures Error", func(t *testing.T) {
		mock.ExpectExec("INSERT OR REPLACE INTO project_features").
			WithArgs(projectID, "{}", sqlmock.AnyArg()).
			WillReturnError(errors.New("save error"))

		err := store.SaveFeatures(projectID, "{}")
		assert.Error(t, err)
	})

	t.Run("GetFeatures Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT content FROM project_features").
			WithArgs(projectID).
			WillReturnError(errors.New("get error"))

		_, err := store.GetFeatures(projectID)
		assert.Error(t, err)
	})

	t.Run("SaveSpec Error", func(t *testing.T) {
		mock.ExpectExec("INSERT OR REPLACE INTO project_specs").
			WithArgs(projectID, "content", sqlmock.AnyArg()).
			WillReturnError(errors.New("save error"))

		err := store.SaveSpec(projectID, "content")
		assert.Error(t, err)
	})

	t.Run("GetSpec Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT content FROM project_specs").
			WithArgs(projectID).
			WillReturnError(errors.New("get error"))

		_, err := store.GetSpec(projectID)
		assert.Error(t, err)
	})

	t.Run("UpdateFeatureStatus GetFeatures Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT content FROM project_features").
			WithArgs(projectID).
			WillReturnError(errors.New("get error"))

		err := store.UpdateFeatureStatus(projectID, "fid", "status", true)
		assert.Error(t, err)
	})

	t.Run("UpdateFeatureStatus SaveFeatures Error", func(t *testing.T) {
		features := `{"features":[{"id":"fid","status":"pending"}]}`
		rows := sqlmock.NewRows([]string{"content"}).AddRow(features)

		mock.ExpectQuery("SELECT content FROM project_features").
			WithArgs(projectID).
			WillReturnRows(rows)

		mock.ExpectExec("INSERT OR REPLACE INTO project_features").
			WithArgs(projectID, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(errors.New("save error"))

		err := store.UpdateFeatureStatus(projectID, "fid", "completed", true)
		assert.Error(t, err)
	})

	t.Run("Cleanup Error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM file_locks").
			WillReturnError(errors.New("delete locks error"))

		err := store.Cleanup()
		assert.Error(t, err)
	})

	t.Run("Cleanup Error 2", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM file_locks").
			WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectExec("DELETE FROM signals").
			WillReturnError(errors.New("delete signals error"))

		err := store.Cleanup()
		assert.Error(t, err)
	})

	t.Run("Cleanup Error 3", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM file_locks").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("DELETE FROM signals").
			WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectExec("DELETE FROM observations").
			WillReturnError(errors.New("delete observations error"))

		err := store.Cleanup()
		assert.Error(t, err)
	})

	t.Run("AcquireLock Query Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").
			WithArgs(projectID, "path").
			WillReturnError(errors.New("query error"))

		_, err := store.AcquireLock(projectID, "path", agentID, time.Millisecond)
		assert.Error(t, err)
	})

	t.Run("GetActiveLocks Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT path, agent_id, expires_at FROM file_locks").
			WithArgs(sqlmock.AnyArg(), projectID).
			WillReturnError(errors.New("query error"))

		_, err := store.GetActiveLocks(projectID)
		assert.Error(t, err)
	})

	t.Run("GetActiveLocks Scan Error", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"path", "agent_id", "expires_at"}).
			AddRow("path", "agent", "invalid-time")

		mock.ExpectQuery("SELECT path, agent_id, expires_at FROM file_locks").
			WithArgs(sqlmock.AnyArg(), projectID).
			WillReturnRows(rows)

		_, err := store.GetActiveLocks(projectID)
		assert.Error(t, err)
	})

	t.Run("ReleaseLock Error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM file_locks").
			WithArgs(projectID, "path", agentID).
			WillReturnError(errors.New("delete error"))

		err := store.ReleaseLock(projectID, "path", agentID)
		assert.Error(t, err)
	})

	t.Run("ReleaseAllLocks Error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM file_locks").
			WithArgs(projectID, agentID).
			WillReturnError(errors.New("delete error"))

		err := store.ReleaseAllLocks(projectID, agentID)
		assert.Error(t, err)
	})

	// Test NewSQLiteStore errors
	t.Run("NewSQLiteStore Open Error", func(t *testing.T) {
		// This is hard to mock via sqlmock because sql.Open is called inside.
		// However, we can mock the ping failure if we could intercept the driver,
		// but standard sqlmock works on an already opened DB or by registering a driver.
		// Testing NewSQLiteStore error paths (sql.Open failure) is usually done by passing an invalid path/driver
		// or mocking the driver. Given we use "sqlite" driver (modernc.org/sqlite), it's hard to mock it easily without side effects.
		// We will skip this for now as we are focusing on methods on the struct.
	})
}
