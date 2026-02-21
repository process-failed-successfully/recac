package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListSessions_Coverage(t *testing.T) {
	tests := []struct {
		name         string
		setupFiles   func(sessionsDir string)
		expectFound  bool
		expectedName string
	}{
		{
			name: "WithValidAndInvalidFiles",
			setupFiles: func(sessionsDir string) {
				// Valid session
				validSession := &SessionState{Name: "valid-session", Status: "running", PID: 999999}
				data, _ := json.Marshal(validSession)
				os.WriteFile(filepath.Join(sessionsDir, "valid-session.json"), data, 0600)

				// Invalid file (not json)
				os.WriteFile(filepath.Join(sessionsDir, "invalid.txt"), []byte("not json"), 0600)

				// Invalid json file
				os.WriteFile(filepath.Join(sessionsDir, "corrupt.json"), []byte("{invalid"), 0600)
			},
			expectFound:  true,
			expectedName: "valid-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, cleanup := setupSessionManager(t)
			defer cleanup()

			tt.setupFiles(sm.sessionsDir)

			sessions, err := sm.ListSessions()
			if err != nil {
				t.Fatalf("ListSessions() error = %v", err)
			}

			found := false
			for _, s := range sessions {
				if s.Name == tt.expectedName {
					found = true
				}
			}

			if tt.expectFound != found {
				t.Errorf("ListSessions() found = %v, want %v", found, tt.expectFound)
			}
		})
	}
}

func TestListArchivedSessions_Coverage(t *testing.T) {
	tests := []struct {
		name         string
		setupFiles   func(archivedDir string)
		expectFound  bool
		expectedName string
	}{
		{
			name: "WithValidAndInvalidFiles",
			setupFiles: func(archivedDir string) {
				// Valid archived session
				validSession := &SessionState{Name: "valid-archived"}
				data, _ := json.Marshal(validSession)
				os.WriteFile(filepath.Join(archivedDir, "valid-archived.json"), data, 0600)

				// Invalid file (not json)
				os.WriteFile(filepath.Join(archivedDir, "invalid.txt"), []byte("not json"), 0600)

				// Invalid json file
				os.WriteFile(filepath.Join(archivedDir, "corrupt.json"), []byte("{invalid"), 0600)

				// Directory (should be skipped)
				os.Mkdir(filepath.Join(archivedDir, "somedir"), 0700)
			},
			expectFound:  true,
			expectedName: "valid-archived",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, cleanup := setupSessionManager(t)
			defer cleanup()

			tt.setupFiles(sm.archivedSessionsDir)

			sessions, err := sm.ListArchivedSessions()
			if err != nil {
				t.Fatalf("ListArchivedSessions() error = %v", err)
			}

			found := false
			for _, s := range sessions {
				if s.Name == tt.expectedName {
					found = true
				}
			}

			if tt.expectFound != found {
				t.Errorf("ListArchivedSessions() found = %v, want %v", found, tt.expectFound)
			}
		})
	}
}

func TestStopSession_Coverage(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(sm *SessionManager) string
		wantErrStr  string
		wantStatus  string
	}{
		{
			name: "ProcessNotRunning",
			setup: func(sm *SessionManager) string {
				sessionName := "process-died"
				session := &SessionState{
					Name:   sessionName,
					Status: "running",
					PID:    99999999, // Unlikely PID
				}
				sm.SaveSession(session)
				return sessionName
			},
			wantErrStr: "not running (process not found)",
			wantStatus: "completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, cleanup := setupSessionManager(t)
			defer cleanup()

			sessionName := tt.setup(sm)

			err := sm.StopSession(sessionName)
			if err == nil {
				t.Error("StopSession() expected error, got nil")
			} else if !strings.Contains(err.Error(), tt.wantErrStr) {
				t.Errorf("StopSession() error = %v, want substring %v", err, tt.wantErrStr)
			}

			updated, _ := sm.LoadSession(sessionName)
			if updated.Status != tt.wantStatus {
				t.Errorf("Session status = %v, want %v", updated.Status, tt.wantStatus)
			}
		})
	}
}

func TestGetSessionLogContent_Coverage(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(sm *SessionManager) string
		lines       int
		wantContent string
		wantErr     bool
		errContains string
	}{
		{
			name: "MissingLogFile",
			setup: func(sm *SessionManager) string {
				sessionName := "missing-log"
				sm.SaveSession(&SessionState{Name: sessionName, LogFile: filepath.Join(sm.sessionsDir, sessionName+".log")})
				return sessionName
			},
			lines:       10,
			wantErr:     true,
			errContains: "could not read log file",
		},
		{
			name: "ReadAllLines",
			setup: func(sm *SessionManager) string {
				sessionName := "read-all"
				content := "line1\nline2\nline3\nline4"
				logFile := filepath.Join(sm.sessionsDir, sessionName+".log")
				os.WriteFile(logFile, []byte(content), 0600)
				sm.SaveSession(&SessionState{Name: sessionName, LogFile: logFile})
				return sessionName
			},
			lines:       0,
			wantContent: "line1\nline2\nline3\nline4",
			wantErr:     false,
		},
		{
			name: "ReadLastNLines",
			setup: func(sm *SessionManager) string {
				sessionName := "read-last-n"
				content := "line1\nline2\nline3\nline4"
				logFile := filepath.Join(sm.sessionsDir, sessionName+".log")
				os.WriteFile(logFile, []byte(content), 0600)
				sm.SaveSession(&SessionState{Name: sessionName, LogFile: logFile})
				return sessionName
			},
			lines:       2,
			wantContent: "line3\nline4",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, cleanup := setupSessionManager(t)
			defer cleanup()

			sessionName := tt.setup(sm)

			logs, err := sm.GetSessionLogContent(sessionName, tt.lines)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSessionLogContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("GetSessionLogContent() error = %v, want substring %v", err, tt.errContains)
			}
			if !tt.wantErr && logs != tt.wantContent {
				t.Errorf("GetSessionLogContent() got = %q, want %q", logs, tt.wantContent)
			}
		})
	}
}
