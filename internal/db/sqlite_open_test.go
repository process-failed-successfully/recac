package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSQLiteStore_InvalidPath(t *testing.T) {
	// A completely invalid path / URL to force an error on db.Ping() or migrate
	_, err := NewSQLiteStore("/dev/null/invalid")
	assert.Error(t, err)
}
