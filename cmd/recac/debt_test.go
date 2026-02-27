package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebtCmd(t *testing.T) {
	// 1. Setup temp dir with a file containing TODO
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	content := `package main
// TODO: Refactor this mess
func main() {}
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// 2. Mock execCommand
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "git" && len(arg) > 0 && arg[0] == "blame" {
			// Call the helper process
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "-test.run=TestDebtHelperProcess", "--", "blame")
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
			return cmd
		}
		return origExec(name, arg...)
	}

	// 3. Run command
	cmd := debtCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Reset flags
	debtJSON = false
	debtMinAge = ""
	debtAuthor = ""

	err = runDebt(cmd, []string{tempDir})

	// If git is not installed, it might fail. We should check that error or skip.
	if err != nil && err.Error() == "git is not installed or not in PATH" {
		t.Skip("git not installed")
	}
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Refactor this mess")
	assert.Contains(t, output, "John Doe")
	// Since we mock 2 years ago
	assert.Contains(t, output, "2.0y")
}

func TestDebtCmd_FilterAge(t *testing.T) {
	// 1. Setup temp dir with a file containing TODO
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	content := `package main
// TODO: Refactor this mess
func main() {}
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// 2. Mock execCommand
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "git" && len(arg) > 0 && arg[0] == "blame" {
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "-test.run=TestDebtHelperProcess", "--", "blame")
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
			return cmd
		}
		return origExec(name, arg...)
	}

	// 3. Run command with min-age 3y (should filter out our 2y old item)
	cmd := debtCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Reset flags
	debtJSON = false
	debtMinAge = "3y"
	debtAuthor = ""

	err = runDebt(cmd, []string{tempDir})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No matching technical debt found")
}

func TestDebtCmd_JSON(t *testing.T) {
	// 1. Setup temp dir with a file containing TODO
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	content := `package main
// TODO: Refactor this mess
func main() {}
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// 2. Mock execCommand
	origExec := execCommand
	defer func() { execCommand = origExec }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "git" && len(arg) > 0 && arg[0] == "blame" {
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "-test.run=TestDebtHelperProcess", "--", "blame")
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
			return cmd
		}
		return origExec(name, arg...)
	}

	// 3. Run command
	cmd := debtCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Reset flags
	debtJSON = true
	debtMinAge = ""
	debtAuthor = ""

	err = runDebt(cmd, []string{tempDir})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"author": "John Doe"`)
	assert.Contains(t, output, `"age": "2.0y"`)
}

// TestDebtHelperProcess is the mock git blame
func TestDebtHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		return
	}

	if args[0] == "blame" {
		// Output porcelain format
		// timestamp 2 years ago
		twoYearsAgo := time.Now().AddDate(-2, 0, 0).Unix()

		fmt.Printf("4e5d6f7a 1 1 1\n")
		fmt.Printf("author John Doe\n")
		fmt.Printf("author-time %d\n", twoYearsAgo)
		os.Exit(0)
	}
	os.Exit(1)
}

func TestTimeSinceHuman(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			name: "Today",
			t:    now,
			want: "today",
		},
		{
			name: "Yesterday",
			t:    now.AddDate(0, 0, -1),
			want: "1d",
		},
		{
			name: "10 days",
			t:    now.AddDate(0, 0, -10),
			want: "10d",
		},
		{
			name: "1 month",
			t:    now.AddDate(0, 0, -31),
			want: "1.0m",
		},
		{
			name: "6 months",
			t:    now.AddDate(0, 0, -180),
			want: "6.0m",
		},
		{
			name: "1 year",
			t:    now.AddDate(-1, 0, -1), // 366 days
			want: "1.0y",
		},
		{
			name: "2.5 years",
			t:    now.AddDate(0, 0, -913),
			want: "2.5y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := timeSinceHuman(tt.t)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseDurationExtended(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:  "Days",
			input: "10d",
			want:  time.Hour * 24 * 10,
		},
		{
			name:  "Weeks",
			input: "2w",
			want:  time.Hour * 24 * 7 * 2,
		},
		{
			name:  "Months",
			input: "3m",
			want:  time.Hour * 24 * 30 * 3,
		},
		{
			name:  "Years",
			input: "1y",
			want:  time.Hour * 24 * 365,
		},
		{
			name:  "Standard Go Duration",
			input: "24h",
			want:  time.Hour * 24,
		},
		{
			name:    "Invalid Suffix",
			input:   "10x",
			wantErr: true,
		},
		{
			name:    "Invalid Number with Suffix",
			input:   "abcy",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDurationExtended(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
