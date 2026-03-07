package scenarios

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSQLParserScenario_Verify_2(t *testing.T) {
	s := &SQLParserScenario{}

	assert.Equal(t, "sql-parser", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("url"))

	tickets := s.Generate("id", "url")
	assert.Len(t, tickets, 1)

	dir := setupGitRepo(t)

	// We need remote origin
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	_ = exec.Command("git", "-C", dir, "branch", "agent/SQL-PARSER").Run()
	_ = exec.Command("git", "-C", dir, "push", "origin", "agent/SQL-PARSER").Run()

	// 1. Missing main file, doesn't know how to run
	err := s.Verify(dir, map[string]string{"SQL-PARSER": "SQL-PARSER"})
	assert.ErrorContains(t, err, "could not determine how to run the parser")

	// 2. Python file but invalid output
	_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hello')"), 0644)
	err = s.Verify(dir, map[string]string{"SQL-PARSER": "SQL-PARSER"})
	assert.ErrorContains(t, err, "no valid JSON object found in output")

	// 3. Python file with invalid JSON output
	_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('{\"a\": }')"), 0644)
	err = s.Verify(dir, map[string]string{"SQL-PARSER": "SQL-PARSER"})
	assert.ErrorContains(t, err, "failed to parse JSON output")

	// 4. Python file with valid JSON but wrong type
	_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('{\"type\": \"insert\"}')"), 0644)
	err = s.Verify(dir, map[string]string{"SQL-PARSER": "SQL-PARSER"})
	assert.ErrorContains(t, err, "expected type containing 'select'")

	// 5. Python file with valid type but no columns
	_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('{\"type\": \"select\"}')"), 0644)
	err = s.Verify(dir, map[string]string{"SQL-PARSER": "SQL-PARSER"})
	assert.ErrorContains(t, err, "failed to find expected columns in AST (need at least 2)")

	// 6. Python file with valid columns but no where clause
	_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('{\"type\": \"select\", \"columns\": [1, 2]}')"), 0644)
	err = s.Verify(dir, map[string]string{"SQL-PARSER": "SQL-PARSER"})
	assert.ErrorContains(t, err, "where clause missing in AST")

	// 7. Python file with valid where clause but no logical 'and'
	_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('{\"type\": \"select\", \"columns\": [1, 2], \"where\": {\"type\": \"or\"}}')"), 0644)
	err = s.Verify(dir, map[string]string{"SQL-PARSER": "SQL-PARSER"})
	assert.ErrorContains(t, err, "expected top-level 'and' in where clause")

	// 8. Python file valid output
	_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('{\"type\": \"select\", \"columns\": [1, 2], \"where\": {\"type\": \"and\"}}')"), 0644)
	err = s.Verify(dir, map[string]string{"SQL-PARSER": "SQL-PARSER"})
	assert.NoError(t, err)

	// Try without passing keys (fallback)
	err = s.Verify(dir, nil)
	assert.NoError(t, err)

	// Check helper functions
	m := map[string]interface{}{
		"k1": "v1",
		"k2": []interface{}{1, 2},
		"k3": map[string]interface{}{"a": "b"},
	}
	assert.Equal(t, "v1", getStringKey(m, "k1", "k0"))
	assert.Equal(t, "", getStringKey(m, "k0"))

	assert.NotNil(t, getArrayKey(m, "k2", "k0"))
	assert.Nil(t, getArrayKey(m, "k0"))

	assert.NotNil(t, getMapKey(m, "k3", "k0"))
	assert.Nil(t, getMapKey(m, "k0"))

	// nested hasLogicalType
	m2 := map[string]interface{}{
		"where": map[string]interface{}{
			"type": "and",
		},
	}
	assert.True(t, hasLogicalType(m2, "and"))
}
