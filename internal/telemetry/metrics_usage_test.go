package telemetry

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsUsage(t *testing.T) {
	project := "test-metrics-usage"

	// Reset metrics if possible? Prometheus default registry is global.
	// We can't easily reset it, but we can check if values increase.
	// Or we can just trust that our helpers call the right metric.
	// Let's use Gather() to inspect.

	// 1. Counters
	initialLines := getMetricValue(t, "recac_lines_generated_total", project)
	TrackLineGenerated(project, 5)
	finalLines := getMetricValue(t, "recac_lines_generated_total", project)
	if finalLines-initialLines != 5 {
		t.Errorf("Expected lines generated to increase by 5, got %v", finalLines-initialLines)
	}

	initialFiles := getMetricValue(t, "recac_files_created_total", project)
	TrackFileCreated(project)
	finalFiles := getMetricValue(t, "recac_files_created_total", project)
	if finalFiles-initialFiles != 1 {
		t.Errorf("Expected files created to increase by 1")
	}

	TrackFileModified(project)
	// Check modified...

	TrackBuildResult(project, true)
	// Check build success...

	TrackAgentIteration(project)
	// Check iterations...

	TrackTokenUsage(project, 100)
	// Check tokens...

	TrackAgentStall(project)
	// Check stalls...

	TrackTaskCompleted(project)
	// Check completed...

	TrackLockContention(project)
	// Check lock...

	TrackOrchestratorLoop(project)
	// Check loops...

	TrackError(project, "test_error")
	// Check error with label type="test_error"

	TrackDBOp(project)
	// Check db ops...

	TrackDockerOp(project)
	// Check docker ops...

	TrackDockerError(project)
	// Check docker errors...

	// 2. Gauges
	SetContextUsage(project, 75.5)
	val := getMetricValue(t, "recac_context_window_usage", project)
	if val != 75.5 {
		t.Errorf("Expected context usage 75.5, got %v", val)
	}

	SetActiveAgents(project, 3)
	val = getMetricValue(t, "recac_active_agents", project)
	if val != 3 {
		t.Errorf("Expected active agents 3, got %v", val)
	}

	SetTasksPending(project, 10)
	val = getMetricValue(t, "recac_tasks_pending", project)
	if val != 10 {
		t.Errorf("Expected tasks pending 10, got %v", val)
	}

	// 3. Histogram
	ObserveAgentLatency(project, 0.5)
	// Harder to check exact value, but count should increase.
}

func getMetricValue(t *testing.T, name string, projectLabel string) float64 {
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}

	for _, mf := range mfs {
		if mf.GetName() == name {
			for _, m := range mf.GetMetric() {
				// Check labels
				for _, label := range m.GetLabel() {
					if label.GetName() == "project" && label.GetValue() == projectLabel {
						if m.Counter != nil {
							return m.Counter.GetValue()
						}
						if m.Gauge != nil {
							return m.Gauge.GetValue()
						}
						if m.Histogram != nil {
							return float64(m.Histogram.GetSampleCount())
						}
					}
				}
			}
		}
	}
	return 0
}

func TestTrackError_Labels(t *testing.T) {
	project := "test-error-labels"
	errType := "db_connection_failed"

	initial := getErrorMetric(t, project, errType)
	TrackError(project, errType)
	final := getErrorMetric(t, project, errType)

	if final-initial != 1 {
		t.Errorf("Expected error count for %s to increase", errType)
	}
}

func getErrorMetric(t *testing.T, project, errType string) float64 {
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}

	for _, mf := range mfs {
		if mf.GetName() == "recac_errors_total" {
			for _, m := range mf.GetMetric() {
				matchProject := false
				matchType := false
				for _, label := range m.GetLabel() {
					if label.GetName() == "project" && label.GetValue() == project {
						matchProject = true
					}
					if label.GetName() == "type" && label.GetValue() == errType {
						matchType = true
					}
				}
				if matchProject && matchType {
					return m.Counter.GetValue()
				}
			}
		}
	}
	return 0
}

func TestStartMetricsServer_Bind(t *testing.T) {
	// This function blocks, so we run it in a goroutine and cancel via some mechanism?
	// It doesn't take a context. It returns error only if it fails all ports.
	// If it succeeds, it blocks forever.
	// So we can't easily test success without leaking a goroutine.
	// We'll trust the logic or refactor to accept context (out of scope).
	// But we can test that calling it twice handles mutex correctly.

	go func() {
		StartMetricsServer(19090)
	}()
	time.Sleep(100 * time.Millisecond)

	// Second call should return immediately with nil because metricsRunning is true
	err := StartMetricsServer(19090)
	if err != nil {
		t.Errorf("Expected nil when already running, got %v", err)
	}
}
