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
	assert.Contains(t, html, "function doJobAction(btn, action, id)")
	assert.Contains(t, html, "fetch(url, { method: method })")

	// Verify buttons render logic exists in fetchJobs
	assert.Contains(t, html, "doJobAction(this, \\'approve\\'")
	assert.Contains(t, html, "doJobAction(this, \\'skip\\'")
	assert.Contains(t, html, "doJobAction(this, \\'retry\\'")
	assert.Contains(t, html, "doJobAction(this, \\'cancel\\'")
	assert.Contains(t, html, "doJobAction(this, \\'purge\\'")

	// Verify empty state CTA exists
	assert.Contains(t, html, "+ Submit Job")

	// Verify clear history button renders
	assert.Contains(t, html, "deleteAction(this, \\'/history\\')")

	// Verify status CSS classes exist for all statuses
	assert.Contains(t, html, ".status-Completed")
	assert.Contains(t, html, ".status-Failed, .status-Error")
	assert.Contains(t, html, ".status-Running, .status-Active, .status-Spawning")
	assert.Contains(t, html, ".status-Pending, .status-Pending-Approval")
	assert.Contains(t, html, ".status-Canceled")

	// Verify dynamic status class generation handles spaces
	assert.Contains(t, html, "<td class=\"status-' + safeStatus.replace(/\\s+/g, '-') + '\">")

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
	assert.Contains(t, html, "onsubmit=\"submitPipeline(); return false;\"")

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

	// Verify Analyze Failures Modal HTML exists
	assert.Contains(t, html, "id=\"analyzeFailuresModal\"")
	assert.Contains(t, html, "Analyze Failures")
	assert.Contains(t, html, "Analyze Durations")

	// Verify Analyze Anomalies Modal HTML exists
	assert.Contains(t, html, "id=\"analyzeAnomaliesModal\"")
	assert.Contains(t, html, "Analyze Anomalies")

	// Verify View Timeline HTML exists
	assert.Contains(t, html, "timelineModal")
	assert.Contains(t, html, "View Timeline")
	assert.Contains(t, html, "onclick=\"openAnalyzeFailuresModal()\"")

	// Verify Export Trace exists
	assert.Contains(t, html, "Export Trace")
	assert.Contains(t, html, "async function exportTrace(btn)")
	assert.Contains(t, html, "fetch('/jobs/export/trace')")

	// Verify Export Pipeline exists
	assert.Contains(t, html, "Export Pipeline")
	assert.Contains(t, html, "async function exportPipeline(btn)")
	assert.Contains(t, html, "fetch('/jobs/export/pipeline?name=dashboard-export')")

	// Verify JS openAnalyzeFailuresModal function exists
	assert.Contains(t, html, "async function openAnalyzeFailuresModal()")
	assert.Contains(t, html, "fetch('/jobs?state=all&status=Failed')")

	// Verify JS openAnalyzeAnomaliesModal function exists
	assert.Contains(t, html, "async function openAnalyzeAnomaliesModal()")
	assert.Contains(t, html, "fetch('/jobs/analyze/anomalies')")

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

	// Verify Explain Job modal HTML exists
	assert.Contains(t, html, "id=\"explainModal\"")
	assert.Contains(t, html, "onclick=\"closeExplainModal()\"")
	assert.Contains(t, html, "id=\"explain-title\"")
	assert.Contains(t, html, "id=\"explain-content\"")

	// Verify Explain Job JS functions exist
	assert.Contains(t, html, "async function explainJob(id)")
	assert.Contains(t, html, "fetch('/jobs/' + encodeURIComponent(id) + '/explain')")
	assert.Contains(t, html, "function closeExplainModal()")

	// Verify Reports functionality exists
	assert.Contains(t, html, "Generate Changelog")
	assert.Contains(t, html, "Generate Postmortem")
	assert.Contains(t, html, "async function generateChangelog(btn)")
	assert.Contains(t, html, "fetch('/changelog/generate')")
	assert.Contains(t, html, "async function generatePostmortem(btn)")
	assert.Contains(t, html, "fetch('/postmortem/generate')")
	assert.Contains(t, html, "id=\"reportModal\"")
}
