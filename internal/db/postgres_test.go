package db

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestPostgresStore_SaveObservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}
	projectID := "proj1"
	agentID := "agent1"
	content := "obs content"

	mock.ExpectExec("INSERT INTO observations").
		WithArgs(projectID, agentID, content).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SaveObservation(projectID, agentID, content)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_QueryHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}
	projectID := "proj1"
	limit := 10

	rows := sqlmock.NewRows([]string{"id", "agent_id", "content", "created_at"}).
		AddRow(1, "agent1", "content1", time.Now()).
		AddRow(2, "agent2", "content2", time.Now())

	mock.ExpectQuery("SELECT id, agent_id, content, created_at FROM observations").
		WithArgs(projectID, limit).
		WillReturnRows(rows)

	obs, err := store.QueryHistory(projectID, limit)
	assert.NoError(t, err)
	assert.Len(t, obs, 2)
	assert.Equal(t, "content1", obs[0].Content)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_Signals(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}
	projectID := "proj1"
	key := "key1"
	value := "val1"

	// SetSignal
	mock.ExpectExec("INSERT INTO signals").
		WithArgs(projectID, key, value).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SetSignal(projectID, key, value)
	assert.NoError(t, err)

	// GetSignal
	rows := sqlmock.NewRows([]string{"value"}).AddRow(value)
	mock.ExpectQuery("SELECT value FROM signals").
		WithArgs(projectID, key).
		WillReturnRows(rows)

	val, err := store.GetSignal(projectID, key)
	assert.NoError(t, err)
	assert.Equal(t, value, val)

	// DeleteSignal
	mock.ExpectExec("DELETE FROM signals").
		WithArgs(projectID, key).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.DeleteSignal(projectID, key)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_FeaturesAndSpec(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}
	projectID := "proj1"
	features := `{"features":[]}`
	spec := "spec content"

	// SaveFeatures
	mock.ExpectExec("INSERT INTO project_features").
		WithArgs(projectID, features).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SaveFeatures(projectID, features)
	assert.NoError(t, err)

	// GetFeatures
	mock.ExpectQuery("SELECT content FROM project_features").
		WithArgs(projectID).
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow(features))

	val, err := store.GetFeatures(projectID)
	assert.NoError(t, err)
	assert.Equal(t, features, val)

	// SaveSpec
	mock.ExpectExec("INSERT INTO project_specs").
		WithArgs(projectID, spec).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SaveSpec(projectID, spec)
	assert.NoError(t, err)

	// GetSpec
	mock.ExpectQuery("SELECT content FROM project_specs").
		WithArgs(projectID).
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow(spec))

	val, err = store.GetSpec(projectID)
	assert.NoError(t, err)
	assert.Equal(t, spec, val)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_UpdateFeatureStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}
	projectID := "proj1"
	featureID := "F1"

	// Prepare initial content
	fl := FeatureList{
		Features: []Feature{
			{ID: "F1", Status: "pending"},
		},
	}
	initialJSON, _ := json.Marshal(fl)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT content FROM project_features").
		WithArgs(projectID).
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow(string(initialJSON)))

	// After update, we expect an UPDATE query with new JSON
	fl.Features[0].Status = "completed"
	fl.Features[0].Passes = true
	updatedJSON, _ := json.Marshal(fl)

	mock.ExpectExec("UPDATE project_features").
		WithArgs(string(updatedJSON), projectID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	err = store.UpdateFeatureStatus(projectID, featureID, "completed", true)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_AcquireLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}
	projectID := "proj1"
	path := "/path"
	agentID := "agent1"

	// Case 1: Lock available (Insert succeeds)
	// Correct way to simulate ErrNoRows with sqlmock on QueryRow:
	mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").
		WithArgs(projectID, path).
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "expires_at"})) // Empty result set

	mock.ExpectExec("INSERT INTO file_locks").
		WithArgs(projectID, path, agentID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	acquired, err := store.AcquireLock(projectID, path, agentID, time.Second)
	assert.NoError(t, err)
	assert.True(t, acquired)
}

func TestPostgresStore_AcquireLock_Expired(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}
	projectID := "proj1"
	path := "/path"
	agentID := "agent1"
	oldAgent := "oldAgent"

	// Case 2: Lock exists but expired
	expiredTime := time.Now().Add(-1 * time.Hour)
	mock.ExpectQuery("SELECT agent_id, expires_at FROM file_locks").
		WithArgs(projectID, path).
		WillReturnRows(sqlmock.NewRows([]string{"agent_id", "expires_at"}).AddRow(oldAgent, expiredTime))

	mock.ExpectExec("UPDATE file_locks").
		WithArgs(agentID, sqlmock.AnyArg(), projectID, path).
		WillReturnResult(sqlmock.NewResult(1, 1))

	acquired, err := store.AcquireLock(projectID, path, agentID, time.Second)
	assert.NoError(t, err)
	assert.True(t, acquired)
}

func TestPostgresStore_ReleaseLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}
	projectID := "proj1"
	path := "/path"
	agentID := "agent1"

	mock.ExpectExec("DELETE FROM file_locks").
		WithArgs(projectID, path, agentID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.ReleaseLock(projectID, path, agentID)
	assert.NoError(t, err)

	// Manager
	mock.ExpectExec("DELETE FROM file_locks").
		WithArgs(projectID, path).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = store.ReleaseLock(projectID, path, "MANAGER")
	assert.NoError(t, err)
}

func TestPostgresStore_Cleanup(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}

	// 1. Delete expired locks
	mock.ExpectExec("DELETE FROM file_locks WHERE expires_at < NOW()").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// 2. Delete old signals
	mock.ExpectExec("DELETE FROM signals").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// 3. Trim observations
	mock.ExpectExec("DELETE FROM observations").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.Cleanup()
	assert.NoError(t, err)
}

func TestPostgresStore_Migrate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}

	// 1. Expect initial creation queries
	// There are 5 queries. We'll verify they are executed.
	// Note: The order matters in migrate().
	// 1. observations
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS observations").WillReturnResult(sqlmock.NewResult(0, 0))
	// 2. signals
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS signals").WillReturnResult(sqlmock.NewResult(0, 0))
	// 3. project_features
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS project_features").WillReturnResult(sqlmock.NewResult(0, 0))
	// 4. project_specs
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS project_specs").WillReturnResult(sqlmock.NewResult(0, 0))
	// 5. file_locks
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS file_locks").WillReturnResult(sqlmock.NewResult(0, 0))

	// 2. Expect fine-grained fixes (step 2)
	// These are executed blindly, ignoring errors (mostly)
	mock.ExpectExec("ALTER TABLE observations ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec("ALTER TABLE signals ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE signals DROP CONSTRAINT IF EXISTS signals_pkey").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE signals ADD PRIMARY KEY").WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec("ALTER TABLE project_features ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features DROP COLUMN IF EXISTS id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features DROP CONSTRAINT IF EXISTS project_features_pkey").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features ADD PRIMARY KEY").WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec("ALTER TABLE project_specs ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs DROP COLUMN IF EXISTS id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs DROP CONSTRAINT IF EXISTS project_specs_pkey").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs ADD PRIMARY KEY").WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec("ALTER TABLE file_locks ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE file_locks DROP CONSTRAINT IF EXISTS file_locks_pkey").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE file_locks ADD PRIMARY KEY").WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.migrate()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_Migrate_PartialFailure(t *testing.T) {
	// Test that failures in step 1 are logged but don't stop execution
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	store := &PostgresStore{db: db}

	// 1. Fail first query
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS observations").WillReturnError(assert.AnError)
	// 2. Succeed others (abbreviated for brevity in test maintenance, but code executes all)
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS signals").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS project_features").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS project_specs").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS file_locks").WillReturnResult(sqlmock.NewResult(0, 0))

	// Step 2 calls... matching any to simplify test as we just want to verify it proceeds
	// We need to consume all calls. There are 15 calls in step 2.
	// sqlmock doesn't support "match any number of times".
	// But we can just expect the specific calls as before.

	mock.ExpectExec("ALTER TABLE observations ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec("ALTER TABLE signals ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE signals DROP CONSTRAINT IF EXISTS signals_pkey").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE signals ADD PRIMARY KEY").WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec("ALTER TABLE project_features ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features DROP COLUMN IF EXISTS id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features DROP CONSTRAINT IF EXISTS project_features_pkey").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_features ADD PRIMARY KEY").WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec("ALTER TABLE project_specs ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs DROP COLUMN IF EXISTS id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs DROP CONSTRAINT IF EXISTS project_specs_pkey").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE project_specs ADD PRIMARY KEY").WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec("ALTER TABLE file_locks ADD COLUMN IF NOT EXISTS project_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE file_locks DROP CONSTRAINT IF EXISTS file_locks_pkey").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE file_locks ADD PRIMARY KEY").WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.migrate()
	assert.NoError(t, err) // Should return nil even if step 1 failed
	assert.NoError(t, mock.ExpectationsWereMet())
}
