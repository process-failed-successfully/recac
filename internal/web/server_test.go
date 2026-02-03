package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"recac/internal/db"
)

func TestNewServer(t *testing.T) {
	store := &MockStore{}
	s := NewServer(store, 8080, "test-project")
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.projectID != "test-project" {
		t.Errorf("Expected projectID 'test-project', got %s", s.projectID)
	}
}

func TestHandleFeatures_Found(t *testing.T) {
	features := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{ID: "F1", Description: "Feature 1"},
		},
	}
	data, _ := json.Marshal(features)

	store := &MockStore{
		GetFeaturesFunc: func(projectID string) (string, error) {
			if projectID == "test-project" {
				return string(data), nil
			}
			return "", errors.New("not found")
		},
	}

	s := NewServer(store, 8080, "test-project")

	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()

	s.handleFeatures(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var got []db.Feature
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(got) != 1 || got[0].ID != "F1" {
		t.Errorf("Unexpected features response: %v", got)
	}
}

func TestHandleFeatures_Fallback(t *testing.T) {
	features := db.FeatureList{
		ProjectName: "Default Project",
		Features: []db.Feature{
			{ID: "F2", Description: "Feature 2"},
		},
	}
	data, _ := json.Marshal(features)

	store := &MockStore{
		GetFeaturesFunc: func(projectID string) (string, error) {
			if projectID == "default" {
				return string(data), nil
			}
			return "", errors.New("not found")
		},
	}

	// Request for "other-project" which doesn't exist, should fallback to "default"
	s := NewServer(store, 8080, "other-project")

	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()

	s.handleFeatures(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var got []db.Feature
	json.NewDecoder(resp.Body).Decode(&got)
	if len(got) != 1 || got[0].ID != "F2" {
		t.Errorf("Unexpected fallback features: %v", got)
	}
}

func TestHandleFeatures_NotFound(t *testing.T) {
	store := &MockStore{
		GetFeaturesFunc: func(projectID string) (string, error) {
			return "", errors.New("not found")
		},
	}
	s := NewServer(store, 8080, "unknown")

	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()

	s.handleFeatures(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if body != "[]" {
		t.Errorf("Expected [], got %s", body)
	}
}

func TestHandleGraph(t *testing.T) {
	features := db.FeatureList{
		ProjectName: "Graph Project",
		Features: []db.Feature{
			{ID: "A", Description: "Feature A", Status: "done"},
			{ID: "B", Description: "Feature B", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"A"}}},
		},
	}
	data, _ := json.Marshal(features)

	store := &MockStore{
		GetFeaturesFunc: func(projectID string) (string, error) {
			if projectID == "test-graph" {
				return string(data), nil
			}
			return "", errors.New("not found")
		},
	}

	s := NewServer(store, 8080, "test-graph")

	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()

	s.handleGraph(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body := w.Body.String()
	if !strings.Contains(body, "graph TD") {
		t.Error("Response should contain Mermaid graph definition")
	}
	if !strings.Contains(body, "A --> B") {
		t.Error("Graph should show dependency A --> B")
	}
}

func TestHandleGraph_Error(t *testing.T) {
	store := &MockStore{
		GetFeaturesFunc: func(projectID string) (string, error) {
			return "", errors.New("not found")
		},
	}
	s := NewServer(store, 8080, "empty")

	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()

	s.handleGraph(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Error[No Data Found]") {
		t.Errorf("Expected error graph, got: %s", body)
	}
}

func TestSanitizeMermaidID(t *testing.T) {
	input := "My.ID With Spaces"
	want := "My_ID_With_Spaces"
	got := sanitizeMermaidID(input)
	if got != want {
		t.Errorf("sanitizeMermaidID(%q) = %q, want %q", input, got, want)
	}
}
