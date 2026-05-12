package db

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteStore_GetSpec_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectQuery("SELECT content FROM project_specs").WithArgs("proj1").WillReturnError(sql.ErrConnDone)

	_, err = store.GetSpec("proj1")
	assert.Error(t, err)
}

func TestSQLiteStore_GetSpec_NoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &SQLiteStore{db: db}

	mock.ExpectQuery("SELECT content FROM project_specs").WithArgs("proj1").WillReturnError(sql.ErrNoRows)

	val, err := store.GetSpec("proj1")
	assert.NoError(t, err)
	assert.Equal(t, "", val)
}
