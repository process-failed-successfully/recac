package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/db"

	"github.com/stretchr/testify/assert"
)

func TestRun_Commands(t *testing.T) {
	// Setup temp DB
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, ".recac.db")

	// Create empty DB
	// We rely on db.NewStore to create it or we create it explicitly
	// db.NewSQLiteStore creates it.

	config := db.StoreConfig{
		Type:             "sqlite",
		ConnectionString: dbPath,
	}

	tests := []struct {
		name          string
		args          []string
		stdin         string
		expectedOut   string
		expectedErr   string
		checkDB       func(t *testing.T, store db.Store)
		setupDB       func(t *testing.T, store db.Store)
		preRun        func(t *testing.T, dir string)
	}{
		{
			name:        "Blocker",
			args:        []string{"exe", "blocker", "Blocked by UI"},
			expectedOut: "Blocker signal set.\n",
			checkDB: func(t *testing.T, store db.Store) {
				val, err := store.GetSignal("default", "BLOCKER")
				assert.NoError(t, err)
				assert.Equal(t, "Blocked by UI", val)
			},
		},
		{
			name:        "QA",
			args:        []string{"exe", "qa"},
			expectedOut: "QA trigger signal set.\n",
			checkDB: func(t *testing.T, store db.Store) {
				val, err := store.GetSignal("default", "TRIGGER_QA")
				assert.NoError(t, err)
				assert.Equal(t, "true", val)
			},
		},
		{
			name:        "Manager",
			args:        []string{"exe", "manager"},
			expectedOut: "Manager trigger signal set.\n",
			checkDB: func(t *testing.T, store db.Store) {
				val, err := store.GetSignal("default", "TRIGGER_MANAGER")
				assert.NoError(t, err)
				assert.Equal(t, "true", val)
			},
		},
		{
			name:        "Signal Generic",
			args:        []string{"exe", "signal", "CUSTOM_KEY", "custom_val"},
			expectedOut: "Signal CUSTOM_KEY set to custom_val.\n",
			checkDB: func(t *testing.T, store db.Store) {
				val, err := store.GetSignal("default", "CUSTOM_KEY")
				assert.NoError(t, err)
				assert.Equal(t, "custom_val", val)
			},
		},
		{
			name:        "Signal Privileged Error",
			args:        []string{"exe", "signal", "PROJECT_SIGNED_OFF", "true"},
			expectedErr: "signal 'PROJECT_SIGNED_OFF' is privileged",
		},
		{
			name:        "Feature List Empty",
			args:        []string{"exe", "feature", "list"},
			expectedOut: `{"features":[]}`,
		},
		{
			name:        "Import Features",
			args:        []string{"exe", "import"},
			stdin:       `{"features": [{"id": "feat-1", "description": "test feature", "status": "todo"}]}`,
			expectedOut: "Successfully imported 1 features.\n",
			checkDB: func(t *testing.T, store db.Store) {
				content, err := store.GetFeatures("default")
				assert.NoError(t, err)
				assert.Contains(t, content, "feat-1")
			},
		},
		{
			name:        "Feature Set",
			args:        []string{"exe", "feature", "set", "feat-1", "--status", "done", "--passes", "true"},
			setupDB: func(t *testing.T, store db.Store) {
				// Pre-populate
				data := `{"features": [{"id": "feat-1", "description": "test feature", "status": "todo"}]}`
				store.SaveFeatures("default", data)
			},
			expectedOut: "Feature feat-1 updated: status=done, passes=true\nAll features completed and passed. Auto-signaling COMPLETED.\n",
			checkDB: func(t *testing.T, store db.Store) {
				content, err := store.GetFeatures("default")
				assert.NoError(t, err)

				var fl struct {
					Features []struct {
						ID     string `json:"id"`
						Status string `json:"status"`
						Passes bool   `json:"passes"`
					} `json:"features"`
				}
				err = json.Unmarshal([]byte(content), &fl)
				assert.NoError(t, err)

				found := false
				for _, f := range fl.Features {
					if f.ID == "feat-1" {
						assert.Equal(t, "done", f.Status)
						assert.True(t, f.Passes)
						found = true
					}
				}
				assert.True(t, found)

				// Check completion signal
				sig, err := store.GetSignal("default", "COMPLETED")
				assert.NoError(t, err)
				assert.Equal(t, "true", sig)
			},
		},
		{
			name:        "Missing Command",
			args:        []string{"exe"},
			expectedErr: "missing command",
			expectedOut: "Usage: agent-bridge <command> [arguments]",
		},
		{
			name:        "Unknown Command",
			args:        []string{"exe", "unknown"},
			expectedErr: "unknown command: unknown",
			expectedOut: "Usage: agent-bridge <command> [arguments]",
		},
		{
			name:        "Verify Missing Args",
			args:        []string{"exe", "verify", "id1"},
			expectedErr: "usage: agent-bridge verify <id> <pass/fail>",
		},
		{
			name:        "Verify File Error",
			args:        []string{"exe", "verify", "id1", "pass"},
			expectedErr: "could not read ui_verification.json",
		},
		{
			name:        "Verify Success",
			args:        []string{"exe", "verify", "feat-1", "pass"},
			preRun: func(t *testing.T, dir string) {
				wd, _ := os.Getwd()
				t.Cleanup(func() { os.Chdir(wd) })
				os.Chdir(dir)
				content := `{"requests": [{"feature_id": "feat-1", "instruction": "check it", "status": "pending"}]}`
				os.WriteFile("ui_verification.json", []byte(content), 0644)
			},
			expectedOut: "UI verification for feat-1 updated to pass.\n",
			checkDB: func(t *testing.T, store db.Store) {
				// Check file content
				data, _ := os.ReadFile("ui_verification.json")
				assert.Contains(t, string(data), `"status": "pass"`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preRun != nil {
				tt.preRun(t, tempDir)
			}

			// Redirect stdout/stderr
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			os.Stdout = wOut
			os.Stderr = wErr

			// Setup stdin if needed
			oldStdin := os.Stdin
			if tt.stdin != "" {
				rIn, wIn, _ := os.Pipe()
				wIn.Write([]byte(tt.stdin))
				wIn.Close()
				os.Stdin = rIn
			}

			// Setup DB state if needed
			if tt.setupDB != nil {
				s, _ := db.NewStore(config)
				tt.setupDB(t, s)
				s.Close()
			}

			// Capture output in goroutine
			outC := make(chan string)
			errC := make(chan string)
			go func() {
				var buf bytes.Buffer
				io.Copy(&buf, rOut)
				outC <- buf.String()
			}()
			go func() {
				var buf bytes.Buffer
				io.Copy(&buf, rErr)
				errC <- buf.String()
			}()

			err := run(tt.args, config, "default")

			wOut.Close()
			wErr.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr
			os.Stdin = oldStdin

			stdout := <-outC
			_ = <-errC

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedOut != "" {
				// Normalize newlines and trim whitespace for robust comparison if needed
				// But exact match for meaningful output is better
				assert.Contains(t, stdout, tt.expectedOut)
			}

			// Check DB
			if tt.checkDB != nil {
				s, _ := db.NewStore(config)
				defer s.Close()
				tt.checkDB(t, s)
			}
		})
	}
}
