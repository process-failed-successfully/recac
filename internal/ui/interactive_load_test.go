package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadModelsFromFile(t *testing.T) {
	// 1. Create Temp File
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "models.json")

	// 2. Write Valid JSON
	data := map[string]interface{}{
		"models": []map[string]string{
			{
				"name":        "test-model",
				"displayName": "Test Model",
				"description": "A test model",
			},
			{
				"name": "fallback-model",
				// missing display/desc
			},
		},
	}
	bytes, _ := json.Marshal(data)
	err := os.WriteFile(tmpFile, bytes, 0644)
	assert.NoError(t, err)

	// 3. Test Load Success
	models, err := loadModelsFromFile(tmpFile)
	assert.NoError(t, err)
	assert.Len(t, models, 2)

	// Verify fields
	assert.Equal(t, "Test Model", models[0].Name)
	assert.Equal(t, "test-model", models[0].Value)
	assert.Equal(t, "A test model", models[0].DescriptionDetails)

	// Verify fallback
	assert.Equal(t, "fallback-model", models[1].Name)
	assert.Equal(t, "fallback-model", models[1].Value)
	assert.Equal(t, "fallback-model", models[1].DescriptionDetails)

	// 4. Test Non-Existent File
	_, err = loadModelsFromFile(filepath.Join(tmpDir, "non_existent.json"))
	assert.Error(t, err)

	// 5. Test Malformed JSON
	badFile := filepath.Join(tmpDir, "bad.json")
	os.WriteFile(badFile, []byte("{ bad json "), 0644)
	_, err = loadModelsFromFile(badFile)
	assert.Error(t, err)
}
