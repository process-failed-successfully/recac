package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultIgnoreMap(t *testing.T) {
	m := DefaultIgnoreMap()
	assert.True(t, m[".git"])
	assert.True(t, m["node_modules"])
	assert.True(t, m["TODO.md"])
	assert.False(t, m["main.go"])
}

func TestWriteLines(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_write.txt")
	lines := []string{"line1", "line2", "line3"}

	err := writeLines(filePath, lines)
	require.NoError(t, err)

	readBack, err := readLines(filePath)
	require.NoError(t, err)
	assert.Equal(t, lines, readBack)

	// Test error: create file in non-existent directory
	invalidPath := filepath.Join(tmpDir, "non_existent_dir", "file.txt")
	err = writeLines(invalidPath, lines)
	assert.Error(t, err)
}

func TestReadLines_Error(t *testing.T) {
	// Test error: read non-existent file
	_, err := readLines("non_existent_file.txt")
	assert.Error(t, err)
}

func TestIsBinaryExt(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".exe", true},
		{".jpg", true},
		{".go", false},
		{".txt", false},
		{".PDF", false}, // Case sensitive in switch?
		{".pdf", true},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, isBinaryExt(tt.ext), "Extension: %s", tt.ext)
	}
}

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{"Empty", []byte{}, false},
		{"Text", []byte("hello world"), false},
		{"Binary", []byte{0x00, 0x01, 0x02}, true},
		{"Mixed", []byte("hello\x00world"), true},
		{"LongText", []byte(strings.Repeat("a", 1000)), false},
		{"LongBinaryDetection", append([]byte(strings.Repeat("a", 500)), 0x00), true},
		{"LongBinaryBeyondLimit", append([]byte(strings.Repeat("a", 600)), 0x00), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isBinaryContent(tt.content))
		})
	}
}

func TestExtractFileContexts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy file
	fileName := "testfile.go"
	filePath := filepath.Join(tmpDir, fileName)
	content := "package main\nfunc main() {}"
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	// Change to temp dir so relative paths work
	t.Chdir(tmpDir)

	tests := []struct {
		name   string
		output string
		check  func(*testing.T, string, error)
	}{
		{
			name:   "No match",
			output: "Error in something",
			check: func(t *testing.T, s string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, s, "No specific files identified")
			},
		},
		{
			name:   "Match existing file",
			output: "Error in testfile.go:1:1",
			check: func(t *testing.T, s string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, s, "File: testfile.go")
				assert.Contains(t, s, content)
			},
		},
		{
			name:   "Match non-existent file",
			output: "Error in missing.go:1",
			check: func(t *testing.T, s string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, s, "Files referenced in output could not be read")
			},
		},
		{
			name:   "Truncated file",
			output: "Error in large.txt:1",
			check: func(t *testing.T, s string, err error) {
				// Create large file
				largeContent := strings.Repeat("a", 11*1024)
				os.WriteFile("large.txt", []byte(largeContent), 0644)

				res, err := extractFileContexts("large.txt:1")
				assert.NoError(t, err)
				assert.Contains(t, res, "... (truncated)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := extractFileContexts(tt.output)
			tt.check(t, s, err)
		})
	}
}

func TestSharedUtilsHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer func() {
		os.Stdout.Sync()
		os.Stderr.Sync()
	}()

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, cmdArgs := args[0], args[1:]

	if cmd == "git" {
		if len(cmdArgs) >= 1 && cmdArgs[0] == "diff" {
			if len(cmdArgs) == 2 && cmdArgs[1] == "HEAD" {
				if os.Getenv("MOCK_GIT_HEAD_FAIL") == "1" {
					os.Exit(1)
				}
				if os.Getenv("MOCK_GIT_NO_CHANGES") == "1" {
					os.Exit(0)
				}
				fmt.Print("diff --git a/file b/file\n...")
				os.Exit(0)
			}
			if len(cmdArgs) == 1 {
				// git diff (fallback)
				if os.Getenv("MOCK_GIT_FALLBACK_FAIL") == "1" {
					os.Exit(1)
				}
				fmt.Print("diff --git a/fallback b/fallback\n...")
				os.Exit(0)
			}
		}
	}
	os.Exit(0)
}

func TestGetGitDiff(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestSharedUtilsHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	t.Run("HEAD success", func(t *testing.T) {
		diff, err := getGitDiff()
		require.NoError(t, err)
		assert.Contains(t, diff, "diff --git a/file")
	})

	t.Run("HEAD fail, fallback success", func(t *testing.T) {
		os.Setenv("MOCK_GIT_HEAD_FAIL", "1")
		defer os.Unsetenv("MOCK_GIT_HEAD_FAIL")

		diff, err := getGitDiff()
		require.NoError(t, err)
		assert.Contains(t, diff, "diff --git a/fallback")
	})

	t.Run("No changes", func(t *testing.T) {
		os.Setenv("MOCK_GIT_NO_CHANGES", "1")
		defer os.Unsetenv("MOCK_GIT_NO_CHANGES")

		diff, err := getGitDiff()
		assert.ErrorIs(t, err, ErrNoChanges)
		assert.Empty(t, diff)
	})
}
