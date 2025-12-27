package telemetry

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestInitLogger_JSONOutput(t *testing.T) {
	// Capture stdout
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	LogInfo("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, `"msg":"test message"`) {
		t.Errorf("Expected output to contain message, got %s", output)
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Errorf("Expected output to contain key-value, got %s", output)
	}
	
	var logMap map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}
	
	// Verify timestamp field exists
	if _, ok := logMap["time"]; !ok {
		t.Error("JSON output missing 'time' (timestamp) field")
	}
	
	// Verify level field exists
	if _, ok := logMap["level"]; !ok {
		t.Error("JSON output missing 'level' field")
	}
	
	// Verify level is "INFO" for LogInfo
	if level, ok := logMap["level"].(string); ok {
		if level != "INFO" {
			t.Errorf("Expected level to be 'INFO', got %q", level)
		}
	}
}
