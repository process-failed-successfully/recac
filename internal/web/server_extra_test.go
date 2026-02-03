package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewServer_Defaults(t *testing.T) {
	mockStore := &MockStore{}

	// Case 1: Empty Project ID -> "default"
	serverDefault := NewServer(mockStore, 8080, "")

	// We verify by calling handleFeatures and checking which projectID it requests from store
	var requestedProjectID string
	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		requestedProjectID = projectID
		// Return some content so it doesn't fallback to default for my-project
		if projectID == "my-project" {
			return "{}", nil
		}
		return "", nil
	}

	req, _ := http.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()
	// We need to access the handler. Server struct doesn't expose mux, but it exposes methods.
	// Methods are bound to the receiver instance which has the projectID.
	handler := http.HandlerFunc(serverDefault.handleFeatures)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "default", requestedProjectID)

	// Case 2: Specific Project ID
	serverProj := NewServer(mockStore, 8080, "my-project")

	handlerProj := http.HandlerFunc(serverProj.handleFeatures)
	handlerProj.ServeHTTP(rr, req)

	assert.Equal(t, "my-project", requestedProjectID)
}
