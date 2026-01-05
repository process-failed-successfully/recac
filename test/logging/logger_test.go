package logging_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/process-failed-successfully/recac/internal/logging"
	"github.com/stretchr/testify/assert"
)

func TestConsoleLogger(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Create a logger that writes to the buffer
	logger := logging.NewConsoleLogger(logging.Debug)
	logger.(*logging.ConsoleLogger).Output = &buf

	ctx := context.Background()

	// Test each log level
	logger.Debug(ctx, "Debug message", map[string]interface{}{"key": "value"})
	logger.Info(ctx, "Info message", nil)
	logger.Warn(ctx, "Warning message", map[string]interface{}{"count": 42})
	logger.Error(ctx, "Error message", nil)
	logger.Critical(ctx, "Critical message", map[string]interface{}{"error": "fatal"})

	// Verify output contains expected messages
	output := buf.String()
	assert.Contains(t, output, "Debug message")
	assert.Contains(t, output, "Info message")
	assert.Contains(t, output, "Warning message")
	assert.Contains(t, output, "Error message")
	assert.Contains(t, output, "Critical message")
}

func TestJobLogger(t *testing.T) {
	var buf bytes.Buffer
	baseLogger := logging.NewConsoleLogger(logging.Debug)
	baseLogger.(*logging.ConsoleLogger).Output = &buf

	jobLogger := logging.NewJobLogger(baseLogger, "test-job-123")

	ctx := context.Background()
	jobLogger.Info(ctx, "Job operation", map[string]interface{}{"step": "processing"})

	output := buf.String()
	assert.Contains(t, output, "test-job-123")
	assert.Contains(t, output, "Job operation")
	assert.Contains(t, output, "step=processing")
}

func TestMetricsCollector(t *testing.T) {
	collector := logging.NewInMemoryMetricsCollector()
	jobID := "test-job-456"

	// Test job started
	collector.IncrementJobStarted(jobID)
	metrics := collector.GetJobMetrics(jobID)
	assert.Equal(t, 1, metrics.StartCount)

	// Test job completed
	collector.IncrementJobCompleted(jobID, true)
	metrics = collector.GetJobMetrics(jobID)
	assert.Equal(t, 1, metrics.SuccessCount)

	// Test job retry
	collector.IncrementJobRetry(jobID, 1)
	metrics = collector.GetJobMetrics(jobID)
	assert.Equal(t, 1, metrics.RetryCount)

	// Test job duration
	duration := 2 * time.Second
	collector.RecordJobDuration(jobID, duration)
	metrics = collector.GetJobMetrics(jobID)
	assert.Equal(t, duration, metrics.TotalDuration)

	// Test orphaned job
	collector.RecordOrphanedJob(jobID)
	metrics = collector.GetJobMetrics(jobID)
	assert.True(t, metrics.IsOrphaned)
}

func TestAlertManager(t *testing.T) {
	alertManager := logging.NewInMemoryAlertManager()
	ctx := context.Background()

	// Test triggering an alert
	alertManager.TriggerAlert(ctx, logging.High, "Test alert", "test-job-789", nil)
	alerts := alertManager.GetActiveAlerts()
	assert.Len(t, alerts, 1)
	assert.Equal(t, "Test alert", alerts[0].Message)
	assert.Equal(t, "test-job-789", alerts[0].JobID)

	// Test clearing an alert
	alertID := alerts[0].ID
	alertManager.ClearAlert(alertID)
	alerts = alertManager.GetActiveAlerts()
	assert.Len(t, alerts, 0)
}

func TestMonitor(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewConsoleLogger(logging.Debug)
	logger.(*logging.ConsoleLogger).Output = &buf

	metrics := logging.NewInMemoryMetricsCollector()
	alertManager := logging.NewInMemoryAlertManager()

	monitor := logging.NewMonitor(logger, metrics, alertManager)
	monitor.Start()
	defer monitor.Stop()

	ctx := context.Background()

	// Test job started event
	monitor.LogEvent(logging.MonitorEvent{
		EventType: "job_started",
		JobID:     "test-job-111",
		Data:      "test-job-111",
	})

	// Test job retry event
	monitor.LogEvent(logging.MonitorEvent{
		EventType: "job_retry",
		JobID:     "test-job-111",
		Data: map[string]interface{}{
			"job_id":  "test-job-111",
			"attempt": 1,
		},
	})

	// Test orphaned job event
	monitor.LogEvent(logging.MonitorEvent{
		EventType: "orphaned_job",
		JobID:     "test-job-111",
		Data:      "test-job-111",
	})

	// Give the monitor time to process events
	time.Sleep(100 * time.Millisecond)

	// Verify metrics were collected
	jobMetrics := metrics.GetJobMetrics("test-job-111")
	assert.Equal(t, 1, jobMetrics.StartCount)
	assert.Equal(t, 1, jobMetrics.RetryCount)
	assert.True(t, jobMetrics.IsOrphaned)

	// Verify alerts were triggered
	alerts := alertManager.GetActiveAlerts()
	assert.Len(t, alerts, 1)
	assert.Equal(t, "Orphaned job detected", alerts[0].Message)

	// Verify logs contain expected messages
	output := buf.String()
	assert.Contains(t, output, "Job started")
	assert.Contains(t, output, "Job retry")
	assert.Contains(t, output, "Orphaned job detected")
}
