package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger_FileOnly(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"
	logger := NewLogger(true, tmpFile, true) // Debug=true, File=tmpFile, SilenceStdout=true

	logger.Info("info message")
	logger.Debug("debug message")

	// Verify file content
	content, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "info message")
	assert.Contains(t, string(content), "debug message")
	assert.Contains(t, string(content), "INFO")
	assert.Contains(t, string(content), "DEBUG")
}

func TestNewLogger_StdoutAndFile(t *testing.T) {
	tmpFile := t.TempDir() + "/test_multi.log"

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
	}()

	logger := NewLogger(false, tmpFile, false) // Debug=false, File=tmpFile, SilenceStdout=false
	logger.Info("multi message")

	w.Close()

	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(r)

	// Verify stdout
	assert.Contains(t, stdoutBuf.String(), "multi message")

	// Verify file
	content, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "multi message")
}

func TestNewLogger_NoHandlers(t *testing.T) {
	logger := NewLogger(false, "", true) // SilenceStdout=true, No file
	assert.NotNil(t, logger)

	// Should not panic
	logger.Info("nothing")
}

func TestMultiHandler_Direct(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewJSONHandler(&buf1, nil)
	h2 := slog.NewJSONHandler(&buf2, nil)

	mh := &multiHandler{handlers: []slog.Handler{h1, h2}}

	ctx := context.Background()

	// Enabled
	assert.True(t, mh.Enabled(ctx, slog.LevelInfo))

	// Handle
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	err := mh.Handle(ctx, record)
	require.NoError(t, err)

	assert.Contains(t, buf1.String(), "test")
	assert.Contains(t, buf2.String(), "test")

	// WithAttrs
	mh2 := mh.WithAttrs([]slog.Attr{slog.String("key", "val")})
	record2 := slog.NewRecord(time.Now(), slog.LevelInfo, "attr", 0)
	mh2.Handle(ctx, record2)

	assert.Contains(t, buf1.String(), `"key":"val"`)
	assert.Contains(t, buf2.String(), `"key":"val"`)

	// WithGroup
	mh3 := mh.WithGroup("grp").WithAttrs([]slog.Attr{slog.String("inner", "val")})
	record3 := slog.NewRecord(time.Now(), slog.LevelInfo, "group", 0)
	mh3.Handle(ctx, record3)

	assert.Contains(t, buf1.String(), `"grp":{"inner":"val"}`)
	assert.Contains(t, buf2.String(), `"grp":{"inner":"val"}`)
}

func TestInitLogger(t *testing.T) {
	// Just ensure it doesn't panic
	InitLogger(false, "", true)
	assert.NotNil(t, slog.Default())
}

func TestHelperFunctions(t *testing.T) {
	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	InitLogger(true, "", false) // Enable debug, output to stdout

	LogInfo("info msg")
	LogDebug("debug msg")
	LogError("error msg", assert.AnError)
	LogInfof("formatted %s", "msg")

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "info msg")
	assert.Contains(t, output, "debug msg")
	assert.Contains(t, output, "error msg")
	assert.Contains(t, output, "formatted msg")
	assert.Contains(t, output, "assert.AnError general error for testing")
}
