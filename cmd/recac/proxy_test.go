package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRoundTripper for testing transport
type MockRoundTripper struct {
	mock.Mock
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestRecordingTransport(t *testing.T) {
	mockRT := new(MockRoundTripper)

	// Create a dummy response
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString("response body")),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "text/plain")

	mockRT.On("RoundTrip", mock.Anything).Return(resp, nil)

	var captured Interaction
	rt := &recordingTransport{
		transport: mockRT,
		onRecord: func(i Interaction) {
			captured = i
		},
	}

	req, _ := http.NewRequest("POST", "http://example.com/api", bytes.NewBufferString("request body"))
	req.Header.Set("Content-Type", "application/json")

	res, err := rt.RoundTrip(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)

	// Verify captured interaction
	assert.Equal(t, "POST", captured.Request.Method)
	assert.Equal(t, "http://example.com/api", captured.Request.URL)
	assert.Equal(t, "request body", captured.Request.Body)
	assert.Equal(t, "response body", captured.Response.Body)
	assert.Equal(t, 200, captured.Response.Status)
}

func TestNewProxyHandler(t *testing.T) {
	// Start a mock target server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("target response"))
	}))
	defer targetServer.Close()

	targetURL, _ := url.Parse(targetServer.URL)
	tempFile := t.TempDir() + "/recording.json"

	var captured Interaction
	handler := NewProxyHandler(targetURL, func(i Interaction) {
		captured = i
	}, tempFile)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "target response", string(body))

	// Verify callback
	// The URL captured is the one sent to the target, so it includes the full target URL
	assert.Equal(t, targetServer.URL+"/test", captured.Request.URL)
	assert.Equal(t, http.StatusCreated, captured.Response.Status)
	assert.Equal(t, "target response", captured.Response.Body)

	// Verify file write
	content, err := os.ReadFile(tempFile)
	assert.NoError(t, err)

	var i Interaction
	err = json.Unmarshal(content, &i) // It writes JSONL, but single line is valid JSON
	assert.NoError(t, err)
	assert.Equal(t, "target response", i.Response.Body)
}

// ProxyMockAgent
type ProxyMockAgent struct {
	mock.Mock
}

func (m *ProxyMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *ProxyMockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	return "", nil
}

func (m *ProxyMockAgent) SetResponse(response string) {}

func (m *ProxyMockAgent) AddMemory(key, value string) {}

func TestRunProxy_Generation(t *testing.T) {
	// Setup interaction file
	interactions := []Interaction{
		{
			Timestamp: time.Now(),
			Request: ReqDump{
				Method: "GET",
				URL:    "http://example.com/api/test",
			},
			Response: ResDump{
				Status: 200,
				Body:   `{"status": "ok"}`,
			},
		},
	}

	tempDir := t.TempDir()
	recordFile := tempDir + "/traffic.json"
	outputFile := tempDir + "/test_gen.go"

	f, _ := os.Create(recordFile)
	data, _ := json.Marshal(interactions[0])
	f.Write(data)
	f.Close()

	// Mock Agent
	mockAgent := new(ProxyMockAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("package main\n\nfunc TestAPI() {}", nil)

	// Override factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Set flags
	// Note: We are testing runProxyGeneration logic indirectly via runProxy or directly if possible.
	// Since runProxyGeneration takes *cobra.Command, we can call it if it was exported or accessible.
	// But it is unexported in main package. We are in main package here (package main_test vs package main).
	// Go tests in the same folder with package main can access unexported identifiers.

	// Temporarily set global vars
	oldProxyGenerate := proxyGenerate
	oldProxyTarget := proxyTarget
	oldProxyRecordFile := proxyRecordFile
	oldProxyOutput := proxyOutput

	defer func() {
		proxyGenerate = oldProxyGenerate
		proxyTarget = oldProxyTarget
		proxyRecordFile = oldProxyRecordFile
		proxyOutput = oldProxyOutput
	}()

	proxyGenerate = true
	proxyTarget = "" // Trigger generation mode
	proxyRecordFile = recordFile
	proxyOutput = outputFile

	// Create dummy command
	cmd := proxyCmd

	err := runProxy(cmd, []string{})
	assert.NoError(t, err)

	// Verify file created
	genContent, err := os.ReadFile(outputFile)
	assert.NoError(t, err)
	assert.Contains(t, string(genContent), "func TestAPI()")
}
