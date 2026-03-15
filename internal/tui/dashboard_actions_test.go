package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToggleDrain(t *testing.T) {
	tests := []struct {
		name       string
		isDraining bool
		wantAction string
		wantMethod string
		wantPath   string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "Drain success",
			isDraining: false,
			wantAction: "Draining",
			wantMethod: http.MethodPost,
			wantPath:   "/drain",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "Undrain success",
			isDraining: true,
			wantAction: "Undrained",
			wantMethod: http.MethodPost,
			wantPath:   "/undrain",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "Drain HTTP error",
			isDraining: false,
			wantMethod: http.MethodPost,
			wantPath:   "/drain",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.wantMethod, r.Method)
				assert.Equal(t, tc.wantPath, r.URL.Path)
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			cmd := toggleDrain(server.URL, tc.isDraining)
			msg := cmd()

			actionMsg, ok := msg.(actionMsg)
			assert.True(t, ok)
			if tc.wantErr {
				assert.Error(t, actionMsg.Err)
			} else {
				assert.NoError(t, actionMsg.Err)
				assert.Equal(t, tc.wantAction, actionMsg.Message)
			}
		})
	}
}

func TestUpdateDependenciesCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/jobs/JOB-1/dependencies", r.URL.Path)
		var body map[string][]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, []string{"DEP-1", "DEP-2"}, body["depends_on"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := updateDependenciesCmd(server.URL, "JOB-1", []string{"DEP-1", "DEP-2"})
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Updated dependencies for job JOB-1", actionMsg.Message)
}

func TestScaleConcurrencyCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/scale", r.URL.Path)
		var body map[string]int
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, 5, body["max_concurrent_jobs"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := scaleConcurrencyCmd(server.URL, 5)
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Scaled concurrency to 5", actionMsg.Message)
}

func TestPurgeJobCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/history/job-123", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := purgeJobCmd(server.URL, "job-123")
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Purged", actionMsg.Message)
}

func TestClearPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/pending", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"cleared": 3}`))
	}))
	defer server.Close()

	cmd := clearPending(server.URL)
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Cleared 3 pending jobs", actionMsg.Message)
}

func TestClearPending_FormatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"cleared": "invalid"}`))
	}))
	defer server.Close()

	cmd := clearPending(server.URL)
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actionMsg.Err)
	assert.Contains(t, actionMsg.Err.Error(), "invalid response format")
}
