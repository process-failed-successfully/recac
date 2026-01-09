package db

import (
	"fmt"
	"strings"
)

// StoreConfig holds configuration for the storage backend
type StoreConfig struct {
	Type             string // "sqlite" or "postgres"
	ConnectionString string // File path for SQLite, DSN for Postgres
}

// NewStore creates a new Store instance based on the provided configuration
func NewStore(config StoreConfig) (Store, error) {
	switch strings.ToLower(config.Type) {
	case "postgres", "postgresql":
		if config.ConnectionString == "" {
			return nil, fmt.Errorf("postgres connection string is required")
		}
		return NewPostgresStore(config.ConnectionString)
	case "sqlite", "sqlite3":
		if config.ConnectionString == "" {
			// Default to .recac.db if not provided
			config.ConnectionString = ".recac.db"
		}
		return NewSQLiteStore(config.ConnectionString)
	case "":
		// Default to SQLite if empty
		if config.ConnectionString == "" {
			config.ConnectionString = ".recac.db"
		}
		return NewSQLiteStore(config.ConnectionString)
	default:
		return nil, fmt.Errorf("unsupported store type: %s", config.Type)
	}
}
