package orchestrator

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAPI_GenericWebhook_Success(t *testing.T) {
	// Setup
	viper.Set("orchestrator.generic_webhook_secret", "") // No secret for this test
	defer viper.Reset()

	mockSpawner := &MockSpawner{}
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 0)
	logger := slog.Default()
	baseCtx := context.Background()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, baseCtx)

	item := WorkItem{
		ID:          "generic-123",
		Summary:     "Test Generic Webhook",
		Description: "A generic webhook test task",
		RepoURL:     "https://github.com/org/repo",
	}
	payload, _ := json.Marshal(item)

	req := httptest.NewRequest("POST", "/webhook/generic", bytes.NewReader(payload))
	rr := httptest.NewRecorder()

	// Execute
	mux.ServeHTTP(rr, req)

	// Verify
	assert.Equal(t, http.StatusAccepted, rr.Code)

	var response map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "generic-123", response["job_id"])

	// Check if job was queued
	jobs := orch.GetActiveJobs()
	if len(jobs) == 0 {
		jobs = orch.GetPendingJobs()
	}
	assert.Len(t, jobs, 1)
	assert.Equal(t, "generic-123", jobs[0].ID)
}

func TestAPI_GenericWebhook_Success_WithSecret(t *testing.T) {
	// Setup
	secret := "super-secret"
	viper.Set("orchestrator.generic_webhook_secret", secret)
	defer viper.Reset()

	mockSpawner := &MockSpawner{}
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 0)
	logger := slog.Default()
	baseCtx := context.Background()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, baseCtx)

	item := WorkItem{
		ID:          "generic-secret-123",
		Summary:     "Test Generic Webhook",
	}
	payload, _ := json.Marshal(item)

	// Calculate signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook/generic", bytes.NewReader(payload))
	req.Header.Set("X-Webhook-Signature", signature)
	rr := httptest.NewRecorder()

	// Execute
	mux.ServeHTTP(rr, req)

	// Verify
	assert.Equal(t, http.StatusAccepted, rr.Code)

	var response map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "generic-secret-123", response["job_id"])
}

func TestAPI_GenericWebhook_InvalidSecret(t *testing.T) {
	// Setup
	secret := "super-secret"
	viper.Set("orchestrator.generic_webhook_secret", secret)
	defer viper.Reset()

	mockSpawner := &MockSpawner{}
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 0)
	logger := slog.Default()
	baseCtx := context.Background()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, baseCtx)

	item := WorkItem{
		ID:          "generic-secret-123",
		Summary:     "Test Generic Webhook",
	}
	payload, _ := json.Marshal(item)

	req := httptest.NewRequest("POST", "/webhook/generic", bytes.NewReader(payload))
	req.Header.Set("X-Webhook-Signature", "sha256=invalid-signature")
	rr := httptest.NewRecorder()

	// Execute
	mux.ServeHTTP(rr, req)

	// Verify
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid X-Webhook-Signature header")
}

func TestAPI_GenericWebhook_MissingSecretHeader(t *testing.T) {
	// Setup
	secret := "super-secret"
	viper.Set("orchestrator.generic_webhook_secret", secret)
	defer viper.Reset()

	mockSpawner := &MockSpawner{}
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 0)
	logger := slog.Default()
	baseCtx := context.Background()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, baseCtx)

	item := WorkItem{
		ID:          "generic-secret-123",
		Summary:     "Test Generic Webhook",
	}
	payload, _ := json.Marshal(item)

	req := httptest.NewRequest("POST", "/webhook/generic", bytes.NewReader(payload))
	// Missing header entirely
	rr := httptest.NewRecorder()

	// Execute
	mux.ServeHTTP(rr, req)

	// Verify
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Missing X-Webhook-Signature header")
}

func TestAPI_GenericWebhook_MissingIDGeneratesOne(t *testing.T) {
	// Setup
	viper.Set("orchestrator.generic_webhook_secret", "")
	defer viper.Reset()

	mockSpawner := &MockSpawner{}
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 0)
	logger := slog.Default()
	baseCtx := context.Background()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, baseCtx)

	// No ID in WorkItem
	item := WorkItem{
		Summary:     "Test Generic Webhook",
	}
	payload, _ := json.Marshal(item)

	req := httptest.NewRequest("POST", "/webhook/generic", bytes.NewReader(payload))
	rr := httptest.NewRecorder()

	// Execute
	mux.ServeHTTP(rr, req)

	// Verify
	assert.Equal(t, http.StatusAccepted, rr.Code)

	var response map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)

	generatedID := response["job_id"]
	assert.NotEmpty(t, generatedID)
	assert.True(t, strings.HasPrefix(generatedID, "webhook-"))

	// Check if job was queued
	jobs := orch.GetActiveJobs()
	if len(jobs) == 0 {
		jobs = orch.GetPendingJobs()
	}
	assert.Len(t, jobs, 1)
	assert.Equal(t, generatedID, jobs[0].ID)
}

func TestAPI_GenericWebhook_InvalidPayload(t *testing.T) {
	// Setup
	viper.Set("orchestrator.generic_webhook_secret", "")
	defer viper.Reset()

	mockSpawner := &MockSpawner{}
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 0)
	logger := slog.Default()
	baseCtx := context.Background()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, logger, baseCtx)

	// Invalid JSON payload
	payload := []byte(`{ "id": "123", "summary": }`)

	req := httptest.NewRequest("POST", "/webhook/generic", bytes.NewReader(payload))
	rr := httptest.NewRecorder()

	// Execute
	mux.ServeHTTP(rr, req)

	// Verify
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid JSON payload")
}
