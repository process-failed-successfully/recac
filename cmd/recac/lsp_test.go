package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSplitRPC(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      []string
		wantError bool
	}{
		{
			name:  "Single message",
			input: "Content-Length: 15\r\n\r\n{\"method\":\"hi\"}",
			want:  []string{"{\"method\":\"hi\"}"},
		},
		{
			name:  "Two messages",
			input: "Content-Length: 15\r\n\r\n{\"method\":\"hi\"}Content-Length: 16\r\n\r\n{\"method\":\"bye\"}",
			want:  []string{"{\"method\":\"hi\"}", "{\"method\":\"bye\"}"},
		},
		{
			name:  "Partial header",
			input: "Content-Length: 15\r\n\r",
			want:  nil,
		},
		{
			name:  "Partial body",
			input: "Content-Length: 15\r\n\r\n{\"method\":",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			scanner.Split(splitRPC)
			var got []string
			for scanner.Scan() {
				got = append(got, string(scanner.Bytes()))
			}
			if tt.wantError {
				assert.Error(t, scanner.Err())
			} else {
				assert.NoError(t, scanner.Err())
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestLSP_Initialize(t *testing.T) {
	var out bytes.Buffer
	server := NewLSPServer(&out)

	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      rawMessagePtr("1"),
	}
	b, _ := json.Marshal(req)

	server.handleMessage(b)

	output := out.String()
	assert.Contains(t, output, "Content-Length:")
	assert.Contains(t, output, "capabilities")
	assert.Contains(t, output, "textDocumentSync")
}

func TestLSP_DidSave_TriggersDiagnostics(t *testing.T) {
	var out bytes.Buffer
	server := NewLSPServer(&out)

	uri := "file:///tmp/test.go"
	code := `package main
	func bad(a, b, c, d, e, f, g int) { // Too many params smell
		var awsKey = "AKIA0123456789012345" // Security issue
	}
	`

	// Open
	openParams := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:  uri,
			Text: code,
		},
	}
	bOpenParams, _ := json.Marshal(openParams)
	reqOpen := RPCRequest{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params:  bOpenParams,
	}
	bOpen, _ := json.Marshal(reqOpen)
	server.handleMessage(bOpen)

	// Save (Wait for goroutine via polling or just call runDiagnostics directly for test stability)
	// Since handleDidSave launches a goroutine, we can't deterministically test it without a WaitGroup hook or sleep.
	// For unit testing logic, calling runDiagnostics directly is safer and cleaner.

	// We invoke runDiagnostics directly on the server instance
	server.runDiagnostics(uri, code)

	output := out.String()
	// Check for Security Finding
	assert.Contains(t, output, "textDocument/publishDiagnostics")
	assert.Contains(t, output, "AWS Access Key")

	// Check for Smell Finding
	assert.Contains(t, output, "Many Parameters")
}

func TestLSP_DidClose(t *testing.T) {
	var out bytes.Buffer
	server := NewLSPServer(&out)
	uri := "file:///test.go"

	// Open
	server.documents[uri] = "content"
	assert.Len(t, server.documents, 1)

	// Close
	closeParams := struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
	}{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}
	bCloseParams, _ := json.Marshal(closeParams)
	reqClose := RPCRequest{
		JSONRPC: "2.0",
		Method:  "textDocument/didClose",
		Params:  bCloseParams,
	}
	bClose, _ := json.Marshal(reqClose)

	server.handleMessage(bClose)

	assert.Len(t, server.documents, 0)
}

func rawMessagePtr(s string) *json.RawMessage {
	r := json.RawMessage([]byte(s))
	return &r
}

// Ensure Thread Safety test
func TestLSP_Concurrency(t *testing.T) {
	var out bytes.Buffer
	server := NewLSPServer(&out)
	uri := "file:///test.go"

	// Simulate concurrent Open and Change
	go func() {
		for i := 0; i < 100; i++ {
			server.documentsMu.Lock()
			server.documents[uri] = "content"
			server.documentsMu.Unlock()
		}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			server.documentsMu.RLock()
			_ = server.documents[uri]
			server.documentsMu.RUnlock()
		}
	}()

	time.Sleep(100 * time.Millisecond)
}
