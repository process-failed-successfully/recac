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

func TestHealJobCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/jobs/JOB-1/heal", r.URL.Path)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"healed_job_id": "JOB-1-healed"}`))
	}))
	defer server.Close()

	cmd := healJobCmd(server.URL, "JOB-1")
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Healed job JOB-1-healed", actionMsg.Message)
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

func TestCleanAllCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)

		if r.URL.Path == "/jobs" {
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/pending" {
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/history" {
			w.WriteHeader(http.StatusOK)
		} else {
			t.Errorf("Unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cmd := cleanAllCmd(server.URL)
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actionMsg.Err)
	assert.Equal(t, "Clean All: OK", actionMsg.Message)
}

func TestCleanAllCmd_CancelJobsFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	cmd := cleanAllCmd(server.URL)
	msg := cmd()

	actionMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actionMsg.Err)
	assert.Contains(t, actionMsg.Err.Error(), "cancel all failed")
}

func TestUpdateDependenciesCmd_Error(t *testing.T) {
	cmd := updateDependenciesCmd("http://localhost:0", "test-id", []string{"dep1"}) // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateDependenciesCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := updateDependenciesCmd(ts.URL, "test-id", []string{"dep1"})
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestUpdateDependenciesCmd_InvalidURL(t *testing.T) {
	cmd := updateDependenciesCmd("::invalid-url", "test-id", []string{"dep1"}) // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateEnvCmd_Error(t *testing.T) {
	cmd := updateEnvCmd("http://localhost:0", "test-id", map[string]string{"K": "V"}) // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateEnvCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := updateEnvCmd(ts.URL, "test-id", map[string]string{"K": "V"})
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestUpdateEnvCmd_InvalidURL(t *testing.T) {
	cmd := updateEnvCmd("::invalid-url", "test-id", map[string]string{"K": "V"}) // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateRenameCmd_Error(t *testing.T) {
	cmd := updateRenameCmd("http://localhost:0", "test-id", "new-id") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateRenameCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := updateRenameCmd(ts.URL, "test-id", "new-id")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestUpdateRenameCmd_InvalidURL(t *testing.T) {
	cmd := updateRenameCmd("::invalid-url", "test-id", "new-id") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateTagsCmd_Error(t *testing.T) {
	cmd := updateTagsCmd("http://localhost:0", "test-id", []string{"tag1"}) // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateTagsCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := updateTagsCmd(ts.URL, "test-id", []string{"tag1"})
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestUpdateTagsCmd_InvalidURL(t *testing.T) {
	cmd := updateTagsCmd("::invalid-url", "test-id", []string{"tag1"}) // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateMaxRetriesCmd_Error(t *testing.T) {
	cmd := updateMaxRetriesCmd("http://localhost:0", "test-id", 3) // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateMaxRetriesCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := updateMaxRetriesCmd(ts.URL, "test-id", 3)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestUpdateMaxRetriesCmd_InvalidURL(t *testing.T) {
	cmd := updateMaxRetriesCmd("::invalid-url", "test-id", 3) // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateAgentCmd_Error(t *testing.T) {
	cmd := updateAgentCmd("http://localhost:0", "test-id", "provider", "model") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdateAgentCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := updateAgentCmd(ts.URL, "test-id", "provider", "model")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestUpdateAgentCmd_InvalidURL(t *testing.T) {
	cmd := updateAgentCmd("::invalid-url", "test-id", "provider", "model") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestSubmitJobCmd_Error(t *testing.T) {
	cmd := submitJobCmd("http://localhost:0", "sum", "repo", "desc", nil, nil, "group", false, "prov", "mod", nil) // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestSubmitJobCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := submitJobCmd(ts.URL, "sum", "repo", "desc", nil, nil, "group", false, "prov", "mod", nil)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestSubmitJobCmd_InvalidURL(t *testing.T) {
	cmd := submitJobCmd("::invalid-url", "sum", "repo", "desc", nil, nil, "group", false, "prov", "mod", nil) // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestCleanAllCmd_Error(t *testing.T) {
	cmd := cleanAllCmd("http://localhost:0") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestCleanAllCmd_CancelFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := cleanAllCmd(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "cancel all failed: status 500")
}

func TestCleanAllCmd_InvalidURL(t *testing.T) {
	cmd := cleanAllCmd("::invalid-url") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestCleanAllCmd_ClearPendingFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := cleanAllCmd(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "clear pending failed: status 500")
}

func TestCleanAllCmd_ClearHistoryFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs" || r.URL.Path == "/pending" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := cleanAllCmd(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "clear history failed: status 500")
}

func TestForcePoll_Error(t *testing.T) {
	cmd := forcePoll("http://localhost:0") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestForcePoll_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := forcePoll(ts.URL)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
}

func TestForcePoll_InvalidURL(t *testing.T) {
	cmd := forcePoll("::invalid-url") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestPurgeJobCmd_Error(t *testing.T) {
	cmd := purgeJobCmd("http://localhost:0", "test-id") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestPurgeJobCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := purgeJobCmd(ts.URL, "test-id")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
}

func TestPurgeJobCmd_InvalidURL(t *testing.T) {
	cmd := purgeJobCmd("::invalid-url", "test-id") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestResumeGroupCmd_Error(t *testing.T) {
	cmd := resumeGroupCmd("http://localhost:0", "test-group") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestResumeGroupCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := resumeGroupCmd(ts.URL, "test-group")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
}

func TestResumeGroupCmd_InvalidURL(t *testing.T) {
	cmd := resumeGroupCmd("::invalid-url", "test-group") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdatePriorityCmd_Error(t *testing.T) {
	cmd := updatePriorityCmd("http://localhost:0", "test-id", 5) // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestUpdatePriorityCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := updatePriorityCmd(ts.URL, "test-id", 5)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestUpdatePriorityCmd_InvalidURL(t *testing.T) {
	cmd := updatePriorityCmd("::invalid-url", "test-id", 5) // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestScaleConcurrencyCmd_Error(t *testing.T) {
	cmd := scaleConcurrencyCmd("http://localhost:0", 5) // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestScaleConcurrencyCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := scaleConcurrencyCmd(ts.URL, 5)
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
}

func TestScaleConcurrencyCmd_InvalidURL(t *testing.T) {
	cmd := scaleConcurrencyCmd("::invalid-url", 5) // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestHealJobCmd_Error(t *testing.T) {
	cmd := healJobCmd("http://localhost:0", "test-id") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestHealJobCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer ts.Close()

	cmd := healJobCmd(ts.URL, "test-id")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
	assert.Contains(t, actMsg.Err.Error(), "server error")
}

func TestHealJobCmd_InvalidURL(t *testing.T) {
	cmd := healJobCmd("::invalid-url", "test-id") // Causes NewRequest to fail
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestArchiveJobCmd_Error(t *testing.T) {
	cmd := archiveJobCmd("http://localhost:0", "test-id") // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestArchiveJobCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := archiveJobCmd(ts.URL, "test-id")
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
}

func TestArchiveBulkJobsCmd_Error(t *testing.T) {
	cmd := archiveBulkJobsCmd("http://localhost:0", map[string]bool{"job1": true}) // Invalid port
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
}

func TestArchiveBulkJobsCmd_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := archiveBulkJobsCmd(ts.URL, map[string]bool{"job1": true})
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.Error(t, actMsg.Err)
	assert.Contains(t, actMsg.Err.Error(), "status 500")
}

func TestArchiveBulkJobsCmd_NoJobs(t *testing.T) {
	cmd := archiveBulkJobsCmd("http://localhost:0", map[string]bool{})
	msg := cmd()

	actMsg, ok := msg.(actionMsg)
	assert.True(t, ok)
	assert.NoError(t, actMsg.Err)
	assert.Equal(t, "No jobs selected", actMsg.Message)
}
