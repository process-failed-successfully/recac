package runner

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/db"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadFeatures_LegacyArrayFormat(t *testing.T) {
	// Setup workspace
	tmpDir, err := os.MkdirTemp("", "recac-test-loadfeatures")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create feature_list.json as an ARRAY (MockAgent format)
	features := []db.Feature{
		{
			ID:          "req-1",
			Description: "Test Feature",
			Status:      "todo",
		},
	}
	data, _ := json.Marshal(features)
	err = os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create Session with dummy logger
	session := &Session{
		Workspace: tmpDir,
		Project:   "test-project",
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Run loadFeatures
	loaded := session.loadFeatures()

	// Assertions
	assert.NotEmpty(t, loaded, "Should have loaded features from array format")
	if len(loaded) > 0 {
		assert.Equal(t, "req-1", loaded[0].ID)
	}
}
