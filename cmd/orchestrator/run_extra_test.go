package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func runTestWrapper(t *testing.T, setup func(), expected string) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	viper.Reset()
	viper.Set("orchestrator.host", "http://invalid-url-that-will-fail.test")
	setup()

	oldExit := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = oldExit }()

	oldStdout := stdout
	r, w, _ := os.Pipe()
	stdout = w
	defer func() { stdout = oldStdout }()

	run(ctx, logger)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	assert.Contains(t, buf.String(), expected)
}

func TestRun_Status(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.status", true)
	}, "Failed to connect to orchestrator")
}

func TestRun_Scale(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", 5)
	}, "Failed to connect to orchestrator")
}

func TestRun_Logs(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.logs", "JOB-123")
	}, "Failed to connect to orchestrator")
}

func TestRun_InspectJob(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.inspect_job", "JOB-123")
	}, "Failed to connect to orchestrator")
}

func TestRun_CancelJob(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.cancel_job", "JOB-123")
	}, "Failed to connect to orchestrator")
}

func TestRun_CancelAll(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.cancel_all", true)
	}, "Failed to connect to orchestrator")
}

func TestRun_RetryJob(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.retry_job", "JOB-123")
	}, "Failed to connect to orchestrator")
}

func TestRun_RetryFailed(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.retry_failed", true)
	}, "Failed to connect to orchestrator")
}

func TestRun_Pause(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.pause", true)
	}, "Failed to connect to orchestrator")
}

func TestRun_Resume(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.resume", true)
	}, "Failed to connect to orchestrator")
}

func TestRun_ApproveJob(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.approve_job", "JOB-123")
	}, "Failed to connect to orchestrator")
}

func TestRun_ForcePoll(t *testing.T) {
	runTestWrapper(t, func() {
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.force_poll", true)
	}, "Failed to connect to orchestrator")
}
