package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"log/slog"
)

func TestMainRun_ListJobs(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.list_jobs", true)
	viper.Set("orchestrator.host", "http://localhost:8080")
	viper.Set("orchestrator.list_jobs_format", "json")

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	err := run(ctx, logger)
	assert.NoError(t, err)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_PrintTimeline(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.timeline", true)
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_PrintStatus(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.status", true)
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_PrintTree(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.tree", true)
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_PrintDependents(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.dependents", true)
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_PrintBlockers(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.blockers", true)
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

// TailJob omitted because it sleeps and timeouts.

func TestMainRun_GenerateChangelog(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.generate_changelog", true)
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_GeneratePostmortem(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.generate_postmortem", "JOB-1")
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_ExportGraph(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.export_graph", "graph.dot")
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_InspectJob(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.inspect_job", "JOB-1")
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_WatchPipeline(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.watch_pipeline", "JOB-1")
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_PrintAnalytics(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.analytics", true)
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_AnalyzeFailures(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.analyze_failures", true)
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}

func TestMainRun_AnalyzeDurations(t *testing.T) {
	viper.Reset()
	viper.Set("orchestrator.analyze_durations", true)
	viper.Set("orchestrator.host", "http://localhost:8080")

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = run(ctx, logger)
	assert.Equal(t, 1, exitCode)
}
