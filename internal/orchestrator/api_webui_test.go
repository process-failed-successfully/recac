package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPI_WebUI_Actions(t *testing.T) {
	orch := New(&mockPoller{}, &MockSpawner{}, 1*time.Minute)

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, slog.Default(), context.Background())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	bodyBytes, err := io.ReadAll(rr.Body)
	assert.NoError(t, err)

	html := string(bodyBytes)

	// Verify JS action function exists
	assert.Contains(t, html, "function doJobAction(action, id)")
	assert.Contains(t, html, "fetch(url, { method: method })")

	// Verify buttons render logic exists in fetchJobs
	assert.Contains(t, html, "doJobAction(\\'approve\\'")
	assert.Contains(t, html, "doJobAction(\\'retry\\'")
	assert.Contains(t, html, "doJobAction(\\'cancel\\'")
	assert.Contains(t, html, "doJobAction(\\'purge\\'")

	// Verify clone job JS function exists
	assert.Contains(t, html, "function cloneJob(")

	// Verify Submit Job Modal HTML exists
	assert.Contains(t, html, "id=\"submitModal\"")
	assert.Contains(t, html, "Submit Ad-hoc Job")
	assert.Contains(t, html, "id=\"job-summary\"")
	assert.Contains(t, html, "id=\"job-repo\"")
	assert.Contains(t, html, "onclick=\"submitAdHocJob()\"")

	// Verify JS submit function exists
	assert.Contains(t, html, "async function submitAdHocJob()")
	assert.Contains(t, html, "fetch('/jobs', {")
}
