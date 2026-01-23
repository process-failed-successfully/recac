package runner

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/db"
	"recac/internal/notify"

	"github.com/stretchr/testify/assert"
)

// CoverageMockAgent simulates agent behavior with controllable errors and responses
type CoverageMockAgent struct {
	Response    string
	SendErr     error
	Store       db.Store
	Project     string
	SignalOnRun bool   // If true, sets signal immediately when called (simulating agent action)
	SignalName  string // Name of signal to set
	SignalValue string // Value of signal to set
}

func (m *CoverageMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendErr != nil {
		return "", m.SendErr
	}
	if m.SignalOnRun && m.Store != nil {
		_ = m.Store.SetSignal(m.Project, m.SignalName, m.SignalValue)
	}
	return m.Response, nil
}

func (m *CoverageMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.SendErr != nil {
		return "", m.SendErr
	}
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestRunQAAgent_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		agentResponse string
		sendErr       error
		signalValue   string // Value to pre-set or set by agent
		setSignal     bool   // Whether agent sets signal
		wantError     bool
		errorMsg      string
	}{
		{
			name:          "Success - Agent signals true",
			agentResponse: "Looks good!",
			signalValue:   "true",
			setSignal:     true,
			wantError:     false,
		},
		{
			name:          "Failure - Agent signals false",
			agentResponse: "Issues found.",
			signalValue:   "false",
			setSignal:     true,
			wantError:     true,
			errorMsg:      "QA Agent explicitly signaled failure",
		},
		{
			name:          "Failure - Implicit (No signal set)",
			agentResponse: "I did nothing.",
			setSignal:     false,
			wantError:     true,
			errorMsg:      "QA Agent did not signal success",
		},
		{
			name:      "Error - Agent Send fails",
			sendErr:   errors.New("connection error"),
			wantError: true,
			errorMsg:  "QA Agent failed to respond",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			workspace := t.TempDir()
			store, _ := db.NewSQLiteStore(filepath.Join(workspace, ".recac.db"))
			defer store.Close()

			mockAgent := &CoverageMockAgent{
				Response:    tt.agentResponse,
				SendErr:     tt.sendErr,
				Store:       store,
				Project:     "test-project",
				SignalOnRun: tt.setSignal,
				SignalName:  "QA_PASSED",
				SignalValue: tt.signalValue,
			}

			s := &Session{
				Workspace: workspace,
				Project:   "test-project",
				DBStore:   store,
				QAAgent:   mockAgent,
				Logger:    slog.Default(),
				Notifier:  notify.NewManager(func(string, ...interface{}) {}),
			}

			// Execute
			err := s.runQAAgent(context.Background())

			// Verify
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunManagerAgent_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		agentResponse  string
		sendErr        error
		preSignal      string // Signal to exist before run
		signalValue    string // Signal set by agent
		setSignal      bool   // Whether agent sets signal
		features       string // JSON content for feature list (to test legacy ratio)
		wantError      bool
		errorMsg       string
	}{
		{
			name:          "Success - Project Signed Off via Signal",
			agentResponse: "Approved.",
			signalValue:   "true",
			setSignal:     true,
			wantError:     false,
		},
		{
			name:          "Success - Legacy Ratio Check (All passing)",
			agentResponse: "Looks good.",
			setSignal:     false,
			features:      `{"features": [{"id": "1", "description": "f1", "passes": true}]}`,
			wantError:     false,
		},
		{
			name:          "Failure - Rejection (No signal, features not passing)",
			agentResponse: "Rejected.",
			setSignal:     false,
			features:      `{"features": [{"id": "1", "description": "f1", "passes": false}]}`,
			wantError:     true,
			errorMsg:      "manager review did not result in sign-off",
		},
		{
			name:      "Error - Agent Send fails",
			sendErr:   errors.New("network error"),
			wantError: true,
			errorMsg:  "manager review request failed",
		},
		{
			name:          "Success - Already Signed Off (Pre-existing signal)",
			agentResponse: "Whatever",
			preSignal:     "true",
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			workspace := t.TempDir()
			store, _ := db.NewSQLiteStore(filepath.Join(workspace, ".recac.db"))
			defer store.Close()

			if tt.features != "" {
				err := store.SaveFeatures("test-project", tt.features)
				assert.NoError(t, err)
			}

			if tt.preSignal == "true" {
				store.SetSignal("test-project", "PROJECT_SIGNED_OFF", "true")
			}

			mockAgent := &CoverageMockAgent{
				Response:    tt.agentResponse,
				SendErr:     tt.sendErr,
				Store:       store,
				Project:     "test-project",
				SignalOnRun: tt.setSignal,
				SignalName:  "PROJECT_SIGNED_OFF",
				SignalValue: tt.signalValue,
			}

			s := &Session{
				Workspace:    workspace,
				Project:      "test-project",
				DBStore:      store,
				ManagerAgent: mockAgent,
				Logger:       slog.Default(),
				Notifier:     notify.NewManager(func(string, ...interface{}) {}),
				// Inject manager frequency to avoid 0 check issues if any
				ManagerFrequency: 5,
			}

			// Execute
			err := s.runManagerAgent(context.Background())

			// Verify
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Additional test to cover legacy completion ratio where features are loaded from file if DB is empty
func TestRunManagerAgent_LegacyFileFallback(t *testing.T) {
	workspace := t.TempDir()
	store, _ := db.NewSQLiteStore(filepath.Join(workspace, ".recac.db"))
	defer store.Close()

	// Write feature list to file
	featureContent := `{"features": [{"id": "1", "description": "file-feature", "passes": true}]}`
	err := os.WriteFile(filepath.Join(workspace, "feature_list.json"), []byte(featureContent), 0644)
	assert.NoError(t, err)

	mockAgent := &CoverageMockAgent{
		Response: "Approved",
		Store:    store,
		Project:  "test-project",
	}

	s := &Session{
		Workspace:    workspace,
		Project:      "test-project",
		DBStore:      store, // DB is empty, should fallback to file
		ManagerAgent: mockAgent,
		Logger:       slog.Default(),
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
	}

	err = s.runManagerAgent(context.Background())
	assert.NoError(t, err, "Should pass via legacy ratio check from file-loaded features")
}
