package db

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_SQLite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Test with explicit type and connection string
	cfg := StoreConfig{
		Type:             "sqlite",
		ConnectionString: dbPath,
	}
	store, err := NewStore(cfg)
	require.NoError(t, err)
	require.NotNil(t, store)
	_, ok := store.(*SQLiteStore)
	assert.True(t, ok, "Expected a SQLiteStore instance")
	store.Close()

	// Test with default type
	cfg = StoreConfig{
		ConnectionString: dbPath,
	}
	store, err = NewStore(cfg)
	require.NoError(t, err)
	require.NotNil(t, store)
	_, ok = store.(*SQLiteStore)
	assert.True(t, ok, "Expected a SQLiteStore instance with default type")
	store.Close()

	// Test with default connection string
	cfg = StoreConfig{
		Type: "sqlite",
	}
	store, err = NewStore(cfg)
	require.NoError(t, err)
	require.NotNil(t, store)
	// We can't easily check the path of the default, but we can check the type
	_, ok = store.(*SQLiteStore)
	assert.True(t, ok, "Expected a SQLiteStore instance with default path")
	store.Close()
}

func TestNewStore_Postgres(t *testing.T) {
	// DSN for a test database. Assumes a running postgres instance.
	// This test will be skipped if the DSN is not available.
	dsn := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"

	// Test with explicit type
	cfg := StoreConfig{
		Type:             "postgres",
		ConnectionString: dsn,
	}
	store, err := NewStore(cfg)
	require.NoError(t, err)
	require.NotNil(t, store)
	_, ok := store.(*PostgresStore)
	assert.True(t, ok, "Expected a PostgresStore instance")
	store.Close()

	// Test with "postgresql" alias
	cfg = StoreConfig{
		Type:             "postgresql",
		ConnectionString: dsn,
	}
	store, err = NewStore(cfg)
	require.NoError(t, err)
	require.NotNil(t, store)
	_, ok = store.(*PostgresStore)
	assert.True(t, ok, "Expected a PostgresStore instance with alias")
	store.Close()

}

func TestNewStore_Errors(t *testing.T) {
	// Test unsupported type
	cfg := StoreConfig{
		Type: "mongodb",
	}
	_, err := NewStore(cfg)
	assert.Error(t, err, "Expected error for unsupported store type")

	// Test missing connection string for postgres
	cfg = StoreConfig{
		Type: "postgres",
	}
	_, err = NewStore(cfg)
	assert.Error(t, err, "Expected error for missing postgres connection string")
}
