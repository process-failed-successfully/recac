package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetTagsAPI(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 1*time.Second)

	orch.activeJobs = map[string]JobInfo{
		"job1": {
			ID:     "job1",
			Status: "Active",
			WorkItem: WorkItem{
				Tags: []string{"backend", "urgent"},
			},
		},
	}
	orch.pendingJobs = map[string]JobInfo{
		"job2": {
			ID:     "job2",
			Status: "Pending",
			WorkItem: WorkItem{
				Tags: []string{"frontend", "urgent"},
			},
		},
	}
	orch.completedJobs = []JobInfo{
		{
			ID:     "job3",
			Status: "Completed",
			WorkItem: WorkItem{
				Tags: []string{"backend"},
			},
		},
		{
			ID:     "job4",
			Status: "Failed",
			WorkItem: WorkItem{
				Tags: []string{"frontend", "bug"},
			},
		},
	}

	logger := slog.Default()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	req, err := http.NewRequest("GET", "/tags", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	type TagInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	var tags []TagInfo
	err = json.NewDecoder(rr.Body).Decode(&tags)
	assert.NoError(t, err)

	expectedTags := []TagInfo{
		{Name: "backend", Count: 2},
		{Name: "frontend", Count: 2},
		{Name: "urgent", Count: 2},
		{Name: "bug", Count: 1},
	}

	assert.Equal(t, expectedTags, tags)
}

func TestGetTagsAPI_Empty(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := slog.Default()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	req, err := http.NewRequest("GET", "/tags", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	type TagInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	var tags []TagInfo
	err = json.NewDecoder(rr.Body).Decode(&tags)
	assert.NoError(t, err)

	assert.Empty(t, tags)
}

func TestGetTagsAPI_EncodeError(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	orch := New(mockPoller, mockSpawner, 1*time.Second)
	logger := slog.Default()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, context.Background())

	req, err := http.NewRequest("GET", "/tags", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	fw := &failingResponseWriter{rr}

	mux.ServeHTTP(fw, req)

	// Nothing to assert, just ensuring no panics
}
