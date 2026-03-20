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

func TestUpdateEnvCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/jobs/JOB-1/env", r.URL.Path)
		var body map[string]map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, map[string]string{"KEY": "VAL", "OTHER": "YES"}, body["env_vars"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := updateEnvCmd(server.URL, "JOB-1", map[string]string{"KEY": "VAL", "OTHER": "YES"})
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Updated environment variables for job JOB-1", actionMsg.Message)
}

func TestUpdateTagsCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/jobs/JOB-1/tags", r.URL.Path)
		var body map[string][]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, []string{"bug", "fix"}, body["tags"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cmd := updateTagsCmd(server.URL, "JOB-1", []string{"bug", "fix"})
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Updated tags for job JOB-1", actionMsg.Message)
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

func TestArchiveJobCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/jobs/JOB-1/archive", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock archive data"))
	}))
	defer server.Close()

	cmd := archiveJobCmd(server.URL, "JOB-1")
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Archived to JOB-1.tar.gz", actionMsg.Message)
}

func TestArchiveBulkJobsCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/jobs/archive/bulk", r.URL.Path)
		match := r.URL.Query().Get("match")
		// The order of map iteration is random, so check that both IDs are in the regex
		assert.Contains(t, match, "JOB-1")
		assert.Contains(t, match, "JOB-2")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock bulk archive data"))
	}))
	defer server.Close()

	selectedJobs := map[string]bool{"JOB-1": true, "JOB-2": true}
	cmd := archiveBulkJobsCmd(server.URL, selectedJobs)
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Archived to bulk_archive.tar.gz", actionMsg.Message)
}
