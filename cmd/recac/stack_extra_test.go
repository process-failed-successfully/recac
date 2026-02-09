package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStackCommand_RequirementsTxt(t *testing.T) {
	tmpDir := t.TempDir()

	content := `
Django==4.0
flask>=2.0
numpy
pandas
pytorch
tensorflow
`
	os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(content), 0644)

	info, err := analyzeStack(tmpDir)
	if err != nil {
		t.Fatalf("analyzeStack failed: %v", err)
	}

	expected := []string{"Django", "Flask", "NumPy", "Pandas", "PyTorch", "TensorFlow"}
	for _, exp := range expected {
		found := false
		for _, f := range info.Frameworks {
			if f == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected framework %s, not found in %v", exp, info.Frameworks)
		}
	}
}
