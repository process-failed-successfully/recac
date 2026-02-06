package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadChallenges_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectories
	subDir1 := filepath.Join(tmpDir, "sub1")
	subDir2 := filepath.Join(tmpDir, "sub2")
	os.Mkdir(subDir1, 0755)
	os.Mkdir(subDir2, 0755)

	// Create challenge files
	c1 := `{"name": "Challenge 1", "description": "d1", "language": "python", "test_file": "t1.py"}`
	c2 := `name: Challenge 2
description: d2
language: go
test_file: t2.go
`
	c3 := `[
  {"name": "Challenge 3a", "description": "d3a", "language": "python", "test_file": "t3a.py"},
  {"name": "Challenge 3b", "description": "d3b", "language": "python", "test_file": "t3b.py"}
]`

	os.WriteFile(filepath.Join(tmpDir, "c1.json"), []byte(c1), 0644)
	os.WriteFile(filepath.Join(subDir1, "c2.yaml"), []byte(c2), 0644)
	os.WriteFile(filepath.Join(subDir2, "c3.json"), []byte(c3), 0644)

	// Create ignored file
	os.WriteFile(filepath.Join(tmpDir, "ignored.txt"), []byte("ignored"), 0644)

	challenges, err := loadChallenges(tmpDir)
	if err != nil {
		t.Fatalf("loadChallenges failed: %v", err)
	}

	if len(challenges) != 4 {
		t.Errorf("Expected 4 challenges, got %d", len(challenges))
	}

	names := make(map[string]bool)
	for _, c := range challenges {
		names[c.Name] = true
	}

	expected := []string{"Challenge 1", "Challenge 2", "Challenge 3a", "Challenge 3b"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("Expected challenge %s not found", name)
		}
	}
}
