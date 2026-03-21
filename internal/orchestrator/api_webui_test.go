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

	// Verify empty state CTA exists
	assert.Contains(t, html, "+ Submit Job")

	// Verify clear history button renders
	assert.Contains(t, html, "deleteAction(\\'/history\\')")

	// Verify clone job JS function exists
	assert.Contains(t, html, "function cloneJob(")

	// Verify accessibility attributes exist
	assert.Contains(t, html, "aria-label=\"Filter jobs by state\"")
	assert.Contains(t, html, "aria-label=\"Search jobs\"")

	// Verify Submit Job Modal HTML exists
	assert.Contains(t, html, "id=\"submitModal\"")
	assert.Contains(t, html, "Submit Ad-hoc Job")
	assert.Contains(t, html, "id=\"job-summary\"")
	assert.Contains(t, html, "id=\"job-repo\"")
	assert.Contains(t, html, "id=\"job-tags\"")
	assert.Contains(t, html, "id=\"job-concurrency-group\"")
	assert.Contains(t, html, "id=\"job-cancel-in-progress\"")
	assert.Contains(t, html, "onsubmit=\"submitAdHocJob(); return false;\"")

	// Verify JS submit function exists
	assert.Contains(t, html, "async function submitAdHocJob()")
	assert.Contains(t, html, "fetch('/jobs', {")
	assert.Contains(t, html, "tags: tags,")

	// Verify Submit Pipeline Modal HTML exists
	assert.Contains(t, html, "+ Submit Pipeline")
	assert.Contains(t, html, "id=\"submitPipelineModal\"")
	assert.Contains(t, html, "id=\"pipeline-yaml\"")
	assert.Contains(t, html, "onclick=\"submitPipeline()\"")

	// Verify JS submitPipeline function exists
	assert.Contains(t, html, "async function submitPipeline()")

	// Verify Search Logs HTML exists
	assert.Contains(t, html, ">Search Logs</button>")
	assert.Contains(t, html, "id=\"searchLogsModal\"")
	assert.Contains(t, html, "id=\"search-logs-query\"")
	assert.Contains(t, html, "id=\"search-logs-tag\"")
	assert.Contains(t, html, "id=\"search-logs-status\"")
	assert.Contains(t, html, "onsubmit=\"performSearchLogs(); return false;\"")

	// Verify JS performSearchLogs function exists
	assert.Contains(t, html, "async function performSearchLogs()")
	assert.Contains(t, html, "fetch('/jobs/pipeline', {")

	// Verify JS dryRunPipeline function exists
	assert.Contains(t, html, "onclick=\"dryRunPipeline()\"")
	assert.Contains(t, html, "async function dryRunPipeline()")
	assert.Contains(t, html, "fetch('/jobs/pipeline/dry-run', {")

	// Verify Set Deps Modal HTML exists
	assert.Contains(t, html, "id=\"editDepsModal\"")
	assert.Contains(t, html, "function editDependencies(encodedJobJson)")
	assert.Contains(t, html, "async function submitEditDeps()")

	// Verify Set Deps button render logic exists
	assert.Contains(t, html, "editDependencies(\\'")

	// Verify Mermaid JS script and View Graph button logic
	assert.Contains(t, html, "cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js")
	assert.Contains(t, html, "mermaid.initialize({ startOnLoad: false });")
	assert.Contains(t, html, "onclick=\"viewGraph()\"")
	assert.Contains(t, html, "id=\"graphModal\"")
	assert.Contains(t, html, "async function viewGraph()")
	assert.Contains(t, html, "fetch('/jobs/export/graph?format=mermaid')")
	assert.Contains(t, html, "mermaid.render(")
}
