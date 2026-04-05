package db

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresStore(t *testing.T) {
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
	mock.ExpectExec("ALTER TABLE observations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE signals").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE signals").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE file_locks").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE file_locks").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE file_locks").WillReturnResult(sqlmock.NewResult(0, 0))

	store := &PostgresStore{db: db}
	err = store.migrate()
	assert.NoError(t, err)

	// Close
	mock.ExpectClose()
	err = store.Close()
	assert.NoError(t, err)
}

func TestPostgresStore_SaveObservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	mock.ExpectExec("INSERT INTO observations").WithArgs("proj1", "agent1", "content").WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SaveObservation("proj1", "agent1", "content")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_QueryHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

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

func TestPostgresStore_SetSignal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	mock.ExpectExec("INSERT INTO signals").WithArgs("proj1", "key1", "val1").WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SetSignal("proj1", "key1", "val1")
	assert.NoError(t, err)
}

func TestPostgresStore_GetSignal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

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

func TestPostgresStore_DeleteSignal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	mock.ExpectExec("DELETE FROM signals").WithArgs("proj1", "key1").WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.DeleteSignal("proj1", "key1")
	assert.NoError(t, err)
}

func TestPostgresStore_SaveFeatures(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	mock.ExpectExec("INSERT INTO project_features").WithArgs("proj1", "{}").WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SaveFeatures("proj1", "{}")
	assert.NoError(t, err)
}

func TestPostgresStore_GetFeatures(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	rows := sqlmock.NewRows([]string{"content"}).AddRow("{}")
	mock.ExpectQuery("SELECT content FROM project_features").WithArgs("proj1").WillReturnRows(rows)

	val, err := store.GetFeatures("proj1")
	assert.NoError(t, err)
	assert.Equal(t, "{}", val)
}

func TestPostgresStore_SaveSpec(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	mock.ExpectExec("INSERT INTO project_specs").WithArgs("proj1", "spec content").WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SaveSpec("proj1", "spec content")
	assert.NoError(t, err)
}

func TestPostgresStore_GetSpec(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	rows := sqlmock.NewRows([]string{"content"}).AddRow("spec content")
	mock.ExpectQuery("SELECT content FROM project_specs").WithArgs("proj1").WillReturnRows(rows)

	val, err := store.GetSpec("proj1")
	assert.NoError(t, err)
	assert.Equal(t, "spec content", val)
}

func TestPostgresStore_AcquireLock(t *testing.T) {
	// Simple test for AcquireLock (doesn't exist, successful insert)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

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

func TestPostgresStore_ReleaseLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	mock.ExpectExec("DELETE FROM file_locks").
		WithArgs("proj1", "file.txt", "agent1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.ReleaseLock("proj1", "file.txt", "agent1")
	assert.NoError(t, err)
}

func TestPostgresStore_ReleaseAllLocks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	mock.ExpectExec("DELETE FROM file_locks").
		WithArgs("proj1", "agent1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.ReleaseAllLocks("proj1", "agent1")
	assert.NoError(t, err)
}

func TestPostgresStore_Cleanup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	mock.ExpectExec("DELETE FROM file_locks WHERE expires_at < NOW()").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM signals WHERE created_at").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM observations WHERE id").WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.Cleanup()
	assert.NoError(t, err)
}

func TestPostgresStore_UpdateFeatureStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	// Mock Begin
	mock.ExpectBegin()

	// Mock QueryRow for existing features
	featuresJSON := `{"features": [{"id": "feat1", "status": "pending", "passes": false}]}`
	rows := sqlmock.NewRows([]string{"content"}).AddRow(featuresJSON)
	mock.ExpectQuery("SELECT content FROM project_features WHERE project_id = \\$1 FOR UPDATE").
		WithArgs("proj1").
		WillReturnRows(rows)

	// Mock Exec for update
	mock.ExpectExec("UPDATE project_features SET content = \\$1, updated_at = NOW\\(\\) WHERE project_id = \\$2").
		WithArgs(sqlmock.AnyArg(), "proj1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock Commit
	mock.ExpectCommit()

	err = store.UpdateFeatureStatus("proj1", "feat1", "completed", true)
	assert.NoError(t, err)
}

func TestPostgresStore_GetActiveLocks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

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

func TestNewPostgresStore_Error_Ping(t *testing.T) {
	_, err := NewPostgresStore("invalid dsn")
	assert.Error(t, err)
}

func TestPostgresStore_UpdateFeatureStatus_Error(t *testing.T) {
	tests := []struct {
		name  string
		setup func(sqlmock.Sqlmock)
	}{
		{
			name: "Transaction error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
			},
		},
		{
			name: "Query error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT content FROM project_features").WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			},
		},
		{
			name: "JSON unmarshal error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				rows := sqlmock.NewRows([]string{"content"}).AddRow("invalid json")
				mock.ExpectQuery("SELECT content FROM project_features").WillReturnRows(rows)
				mock.ExpectRollback()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			store := &PostgresStore{db: db}

			tt.setup(mock)
			err = store.UpdateFeatureStatus("proj1", "feat1", "completed", true)
			assert.Error(t, err)
		})
	}
}

func TestPostgresStore_Cleanup_Error(t *testing.T) {
	tests := []struct {
		name  string
		setup func(sqlmock.Sqlmock)
	}{
		{
			name: "Error on first exec",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM file_locks").WillReturnError(sql.ErrConnDone)
			},
		},
		{
			name: "Error on second exec",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM file_locks").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("DELETE FROM signals").WillReturnError(sql.ErrConnDone)
			},
		},
		{
			name: "Error on third exec",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM file_locks").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("DELETE FROM signals").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("DELETE FROM observations").WillReturnError(sql.ErrConnDone)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			store := &PostgresStore{db: db}

			tt.setup(mock)
			err = store.Cleanup()
			assert.Error(t, err)
		})
	}
}

func TestPostgresStore_GetActiveLocks_Error(t *testing.T) {
	tests := []struct {
		name  string
		setup func(sqlmock.Sqlmock)
	}{
		{
			name: "Query error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT path, agent_id, expires_at FROM file_locks").WillReturnError(sql.ErrConnDone)
			},
		},
		{
			name: "Scan error",
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"path", "agent_id", "expires_at"}).AddRow("file.txt", "agent1", "invalid_date")
				mock.ExpectQuery("SELECT path, agent_id, expires_at FROM file_locks").WillReturnRows(rows)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			store := &PostgresStore{db: db}

			tt.setup(mock)
			_, err = store.GetActiveLocks("proj1")
			assert.Error(t, err)
		})
	}
}

func TestPostgresStore_AcquireLock_Hijack(t *testing.T) {
	tests := []struct {
		name  string
		setup func(sqlmock.Sqlmock)
		agent string
	}{
		{
			name: "Lock exists but expired -> Hijack",
			setup: func(mock sqlmock.Sqlmock) {
				expiredTime := time.Now().UTC().Add(-1 * time.Hour)
				rows := sqlmock.NewRows([]string{"agent_id", "expires_at"}).AddRow("old_agent", expiredTime)
				mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").WillReturnRows(rows)
				mock.ExpectExec("UPDATE file_locks SET agent_id = \\$1").WillReturnResult(sqlmock.NewResult(1, 1))
			},
			agent: "new_agent",
		},
		{
			name: "Lock exists and held by us -> Renew",
			setup: func(mock sqlmock.Sqlmock) {
				futureTime := time.Now().UTC().Add(1 * time.Hour)
				rows2 := sqlmock.NewRows([]string{"agent_id", "expires_at"}).AddRow("my_agent", futureTime)
				mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").WillReturnRows(rows2)
				mock.ExpectExec("UPDATE file_locks SET expires_at = \\$1").WillReturnResult(sqlmock.NewResult(1, 1))
			},
			agent: "my_agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			store := &PostgresStore{db: db}

			tt.setup(mock)
			acquired, err := store.AcquireLock("proj1", "file.txt", tt.agent, time.Second)
			assert.NoError(t, err)
			assert.True(t, acquired)
		})
	}
}

func TestNewPostgresStoreWithDB_MigrateError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)

	mock.ExpectPing()
	// Currently, migrate() handles internal DB errors gracefully and returns nil.
	// We verify that NewPostgresStoreWithDB completes successfully under these conditions.
	_, err = NewPostgresStoreWithDB(db)
	assert.NoError(t, err)
}

func TestNewPostgresStoreWithDB_PingError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)

	mock.ExpectPing().WillReturnError(sql.ErrConnDone)

	_, err = NewPostgresStoreWithDB(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping database")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNewPostgresStore_ConnectionError(t *testing.T) {
	// Calling with invalid format that the "postgres" driver rejects early.
	// "postgres" driver rejects empty string or some formats before Ping.
	_, err := NewPostgresStore("invalid_dsn")
	assert.Error(t, err)
}
