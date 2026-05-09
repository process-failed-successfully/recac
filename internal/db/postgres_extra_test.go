package db

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresStore_ErrorPaths(t *testing.T) {
	_, err := NewPostgresStore("postgres://invalid:invalid@localhost:5432/invalid?sslmode=disable")
	if err == nil {
		t.Error("Expected error connecting to invalid postgres")
	}
}

func TestPostgresStore_CleanupErrorPaths(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &PostgresStore{db: db}

	mock.ExpectExec("DELETE FROM file_locks WHERE expires_at < NOW\\(\\)").WillReturnError(sql.ErrConnDone)

	err = store.Cleanup()
	assert.Error(t, err)

	mock.ExpectExec("DELETE FROM file_locks WHERE expires_at < NOW\\(\\)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM signals").WillReturnError(sql.ErrConnDone)

	err = store.Cleanup()
	assert.Error(t, err)

	mock.ExpectExec("DELETE FROM file_locks WHERE expires_at < NOW\\(\\)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM signals").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM observations").WillReturnError(sql.ErrConnDone)

	err = store.Cleanup()
	assert.Error(t, err)
}

func TestPostgresStore_Close(t *testing.T) {
	var store *PostgresStore
	err := store.Close()
	assert.NoError(t, err)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	store = &PostgresStore{db: db}
	mock.ExpectClose()

	err = store.Close()
	assert.NoError(t, err)
}
