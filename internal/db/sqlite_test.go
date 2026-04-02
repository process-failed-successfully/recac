package db

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSQLiteStore(t *testing.T) {
	// Use mock db instead of real connection
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	// Mock ping
	mock.ExpectPing()

	// Mock migrate
	// In migrate, we do lots of Execs. We can just mock them to return success.
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS observations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS signals").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS project_features").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS project_specs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS file_locks").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_observations_project_created ON observations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE observations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE signals").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE file_locks").WillReturnResult(sqlmock.NewResult(0, 0))

	store := &SQLiteStore{db: db}
	err = store.migrate()
	assert.NoError(t, err)

	// Close
	mock.ExpectClose()
	err = store.Close()
	assert.NoError(t, err)
}

func TestSQLiteStore_SaveObservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectExec("INSERT INTO observations").WithArgs("proj1", "agent1", "content", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SaveObservation("proj1", "agent1", "content")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLiteStore_QueryHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	rows := sqlmock.NewRows([]string{"id", "agent_id", "content", "created_at"}).
		AddRow(1, "agent1", "hello", time.Now())

	mock.ExpectQuery("SELECT id, agent_id, content, created_at FROM observations").
		WithArgs("proj1", 10).
		WillReturnRows(rows)

	obs, err := store.QueryHistory("proj1", 10)
	assert.NoError(t, err)
	assert.Len(t, obs, 1)
	assert.Equal(t, "agent1", obs[0].AgentID)
}

func TestSQLiteStore_SetSignal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectExec("INSERT OR REPLACE INTO signals").WithArgs("proj1", "key1", "val1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SetSignal("proj1", "key1", "val1")
	assert.NoError(t, err)
}

func TestSQLiteStore_GetSignal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	rows := sqlmock.NewRows([]string{"value"}).AddRow("val1")
	mock.ExpectQuery("SELECT value FROM signals").WithArgs("proj1", "key1").WillReturnRows(rows)

	val, err := store.GetSignal("proj1", "key1")
	assert.NoError(t, err)
	assert.Equal(t, "val1", val)

	mock.ExpectQuery("SELECT value FROM signals").WithArgs("proj1", "key2").WillReturnError(sql.ErrNoRows)
	val2, err2 := store.GetSignal("proj1", "key2")
	assert.NoError(t, err2)
	assert.Equal(t, "", val2)
}

func TestSQLiteStore_DeleteSignal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectExec("DELETE FROM signals").WithArgs("proj1", "key1").WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.DeleteSignal("proj1", "key1")
	assert.NoError(t, err)
}

func TestSQLiteStore_SaveFeatures(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectExec("INSERT OR REPLACE INTO project_features").WithArgs("proj1", "{}", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SaveFeatures("proj1", "{}")
	assert.NoError(t, err)
}

func TestSQLiteStore_GetFeatures(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	rows := sqlmock.NewRows([]string{"content"}).AddRow("{}")
	mock.ExpectQuery("SELECT content FROM project_features").WithArgs("proj1").WillReturnRows(rows)

	val, err := store.GetFeatures("proj1")
	assert.NoError(t, err)
	assert.Equal(t, "{}", val)
}

func TestSQLiteStore_SaveSpec(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectExec("INSERT OR REPLACE INTO project_specs").WithArgs("proj1", "spec content", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SaveSpec("proj1", "spec content")
	assert.NoError(t, err)
}

func TestSQLiteStore_GetSpec(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	rows := sqlmock.NewRows([]string{"content"}).AddRow("spec content")
	mock.ExpectQuery("SELECT content FROM project_specs").WithArgs("proj1").WillReturnRows(rows)

	val, err := store.GetSpec("proj1")
	assert.NoError(t, err)
	assert.Equal(t, "spec content", val)
}

func TestSQLiteStore_UpdateFeatureStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	// Mock GetFeatures
	featuresJSON := `{"features": [{"id": "feat1", "status": "pending", "passes": false}]}`
	rows := sqlmock.NewRows([]string{"content"}).AddRow(featuresJSON)
	mock.ExpectQuery("SELECT content FROM project_features WHERE project_id = \\?").
		WithArgs("proj1").
		WillReturnRows(rows)

	// Mock SaveFeatures
	mock.ExpectExec("INSERT OR REPLACE INTO project_features").
		WithArgs("proj1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.UpdateFeatureStatus("proj1", "feat1", "completed", true)
	assert.NoError(t, err)
}

func TestSQLiteStore_AcquireLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").
		WithArgs("proj1", "file.txt").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec("INSERT INTO file_locks").
		WithArgs("proj1", "file.txt", "agent1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	acquired, err := store.AcquireLock("proj1", "file.txt", "agent1", time.Second)
	assert.NoError(t, err)
	assert.True(t, acquired)
}

func TestSQLiteStore_ReleaseLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectExec("DELETE FROM file_locks").
		WithArgs("proj1", "file.txt", "agent1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.ReleaseLock("proj1", "file.txt", "agent1")
	assert.NoError(t, err)
}

func TestSQLiteStore_ReleaseAllLocks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectExec("DELETE FROM file_locks").
		WithArgs("proj1", "agent1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.ReleaseAllLocks("proj1", "agent1")
	assert.NoError(t, err)
}

func TestSQLiteStore_Cleanup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectExec("DELETE FROM file_locks WHERE expires_at < CURRENT_TIMESTAMP").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM signals WHERE created_at").
		WithArgs("PROJECT_SIGNED_OFF", "QA_PASSED", "COMPLETED").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM observations WHERE id").WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.Cleanup()
	assert.NoError(t, err)
}

func TestSQLiteStore_GetActiveLocks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	rows := sqlmock.NewRows([]string{"path", "agent_id", "expires_at"}).
		AddRow("file.txt", "agent1", time.Now().Add(time.Hour))
	mock.ExpectQuery("SELECT path, agent_id, expires_at FROM file_locks").
		WithArgs(sqlmock.AnyArg(), "proj1").
		WillReturnRows(rows)

	locks, err := store.GetActiveLocks("proj1")
	assert.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, "file.txt", locks[0].Path)
}
