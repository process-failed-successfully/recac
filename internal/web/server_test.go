package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"recac/internal/db"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_HandleFeatures(t *testing.T) {
	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test-proj")

	// Prepare mock data
	featureList := db.FeatureList{
		ProjectName: "test-proj",
		Features: []db.Feature{
			{ID: "f1", Description: "feature 1"},
		},
	}
	data, _ := json.Marshal(featureList)

	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		if projectID == "test-proj" {
			return string(data), nil
		}
		return "", nil
	}

	req, _ := http.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.handleFeatures)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resFeatures []db.Feature
	err := json.Unmarshal(rr.Body.Bytes(), &resFeatures)
	assert.NoError(t, err)
	assert.Len(t, resFeatures, 1)
	assert.Equal(t, "f1", resFeatures[0].ID)
}

func TestServer_HandleFeatures_Empty(t *testing.T) {
	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test-proj")

	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		return "", nil
	}

	req, _ := http.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.handleFeatures)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "[]", rr.Body.String())
}

func TestServer_HandleGraph(t *testing.T) {
	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test-proj")

	featureList := db.FeatureList{
		ProjectName: "test-proj",
		Features: []db.Feature{
			{ID: "f1", Description: "done", Status: "done"},
			{
				ID: "f2",
				Description: "pending",
				Status: "pending",
				Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"f1"}},
			},
		},
	}

	data, _ := json.Marshal(featureList)

	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		if projectID == "test-proj" {
			return string(data), nil
		}
		return "", nil
	}

	req, _ := http.NewRequest("GET", "/api/graph", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.handleGraph)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "graph TD")
	assert.Contains(t, body, "f1")
	assert.Contains(t, body, "f2")
	assert.Contains(t, body, "f1 --> f2")
	assert.Contains(t, body, ":::done")
}

func TestGenerateMermaid(t *testing.T) {
	g := runner.NewTaskGraph()
	g.AddNode("node1", "Node 1", nil)
	g.AddNode("node2", "Node 2", []string{"node1"})

	// Set status
	g.Nodes["node1"].Status = runner.TaskDone

	out := generateMermaid(g)
	assert.Contains(t, out, "graph TD")
	assert.Contains(t, out, "node1 --> node2")
	assert.Contains(t, out, ":::done")
}

func TestSanitizeMermaidID(t *testing.T) {
	id := "foo bar.baz-qux"
	sanitized := sanitizeMermaidID(id)
	assert.Equal(t, "foo_bar_baz_qux", sanitized)
}
