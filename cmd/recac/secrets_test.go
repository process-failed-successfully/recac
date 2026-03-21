package main

import (
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestSecretsIntegration(t *testing.T) {
	// 1. Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-secrets-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Chdir(oldWd)
	}()

	// 2. Init Key
	// Override global variable secretsKeyFile for testing
	secretsKeyFile = ".recac.key"

	if err := runSecretsInit(nil, nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := os.Stat(".recac.key"); os.IsNotExist(err) {
		t.Error(".recac.key not created")
	}

	// 3. Create .env
	envContent := "SECRET_KEY=supersecret\nDB_PASS=12345"
	if err := os.WriteFile(".env", []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Encrypt
	if err := runSecretsEncrypt(nil, []string{".env"}); err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if _, err := os.Stat(".env.enc"); os.IsNotExist(err) {
		t.Error(".env.enc not created")
	}

	// 5. Decrypt
	// Mock command with flags
	mockCmd := &cobra.Command{}
	mockCmd.Flags().StringP("output", "o", "", "")
	mockCmd.Flags().Set("output", "decrypted.env")

	if err := runSecretsDecrypt(mockCmd, []string{".env.enc"}); err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	decryptedContent, err := os.ReadFile("decrypted.env")
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}

	if string(decryptedContent) != envContent {
		t.Errorf("Decrypted content mismatch.\nExpected:\n%s\nGot:\n%s", envContent, string(decryptedContent))
	}

	// 6. Test Run
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printerCode := `package main
import (
	"fmt"
	"os"
)
func main() {
	fmt.Print(os.Getenv("SECRET_KEY"))
}
`
	if err := os.WriteFile("printer.go", []byte(printerCode), 0644); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatal(err)
	}

	err = runSecretsRun(nil, []string{"go", "run", "printer.go"})

	// Close write end of pipe to finish writing
	w.Close()
	os.Stdout = oldStdout // Restore stdout

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	out, _ := io.ReadAll(r)
	if string(out) != "supersecret" {
		t.Errorf("Run command output mismatch. Expected 'supersecret', got '%s'", string(out))
	}
}

func TestSecretsInit_KeyExists(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "recac-secrets-test-exists")
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	secretsKeyFile = ".recac.key"
	os.WriteFile(".recac.key", []byte("dummy key"), 0644)

	err := runSecretsInit(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestSecretsInit_AddToGitignore(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "recac-secrets-test-gitignore")
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	secretsKeyFile = ".recac.key"
	os.WriteFile(".gitignore", []byte("node_modules\n"), 0644)

	err := runSecretsInit(nil, nil)
	assert.NoError(t, err)

	content, _ := os.ReadFile(".gitignore")
	assert.Contains(t, string(content), ".recac.key")
}

func TestSecretsInit_AlreadyInGitignore(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "recac-secrets-test-gitignore-exists")
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	secretsKeyFile = ".recac.key"
	os.WriteFile(".gitignore", []byte(".recac.key\n"), 0644)

	err := runSecretsInit(nil, nil)
	assert.NoError(t, err)

	// Should still just have one entry
	content, _ := os.ReadFile(".gitignore")
	assert.Equal(t, ".recac.key\n", string(content))
}
