package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"recac/internal/security"

	"github.com/spf13/cobra"
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Start the Language Server",
	Long:  `Starts the recac Language Server Protocol (LSP) server.
This allows editors like VS Code, Neovim, etc., to integrate directly with recac
to provide real-time code analysis, security checks, and more.`,
	RunE: runLSP,
}

func init() {
	rootCmd.AddCommand(lspCmd)
}

type LSPServer struct {
	out         io.Writer
	documentsMu sync.RWMutex
	documents   map[string]string // URI -> Content
}

func NewLSPServer(out io.Writer) *LSPServer {
	return &LSPServer{
		out:       out,
		documents: make(map[string]string),
	}
}

func runLSP(cmd *cobra.Command, args []string) error {
	server := NewLSPServer(cmd.OutOrStdout())
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(splitRPC)

	for scanner.Scan() {
		msg := scanner.Bytes()
		server.handleMessage(msg)
	}

	if err := scanner.Err(); err != nil {
		if err != io.EOF {
			return fmt.Errorf("read error: %w", err)
		}
	}
	return nil
}

// splitRPC is a split function for bufio.Scanner that parses JSON-RPC messages with Content-Length headers
func splitRPC(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Find Content-Length
	const headerPrefix = "Content-Length: "
	headerEnd := strings.Index(string(data), "\r\n\r\n")
	if headerEnd == -1 {
		return 0, nil, nil // Need more data
	}

	// Parse Content-Length
	headerSection := string(data[:headerEnd])
	idx := strings.Index(headerSection, headerPrefix)
	if idx == -1 {
		return 0, nil, fmt.Errorf("missing Content-Length")
	}

	clStr := headerSection[idx+len(headerPrefix):]
	if endOfLine := strings.Index(clStr, "\r\n"); endOfLine != -1 {
		clStr = clStr[:endOfLine]
	}

	contentLength, err := strconv.Atoi(clStr)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid Content-Length: %w", err)
	}

	totalLength := headerEnd + 4 + contentLength
	if len(data) < totalLength {
		return 0, nil, nil // Need more data
	}

	return totalLength, data[headerEnd+4 : totalLength], nil
}

func (s *LSPServer) handleMessage(data []byte) {
	var req RPCRequest
	if err := json.Unmarshal(data, &req); err == nil && req.Method != "" {
		s.handleRequest(&req)
		return
	}
}

func (s *LSPServer) handleRequest(req *RPCRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "shutdown":
		s.reply(req.ID, nil)
	case "exit":
		os.Exit(0)
	case "textDocument/didOpen":
		s.handleDidOpen(req)
	case "textDocument/didChange":
		s.handleDidChange(req)
	case "textDocument/didSave":
		s.handleDidSave(req)
	case "textDocument/didClose":
		s.handleDidClose(req)
	default:
		// Unknown method
	}
}

func (s *LSPServer) reply(id *json.RawMessage, result interface{}) {
	if id == nil {
		return
	}
	resp := RPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
	s.send(resp)
}

func (s *LSPServer) send(v interface{}) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n%s", len(b), b)
}

func (s *LSPServer) sendNotification(method string, params interface{}) {
	bParams, _ := json.Marshal(params)
	msg := RPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  bParams,
	}
	s.send(msg)
}

// Handlers

func (s *LSPServer) handleInitialize(req *RPCRequest) {
	res := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: 1, // Full
			HoverProvider:    false,
		},
	}
	s.reply(req.ID, res)
}

func (s *LSPServer) handleDidOpen(req *RPCRequest) {
	var params DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return
	}
	s.documentsMu.Lock()
	s.documents[params.TextDocument.URI] = params.TextDocument.Text
	s.documentsMu.Unlock()
}

func (s *LSPServer) handleDidChange(req *RPCRequest) {
	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return
	}
	if len(params.ContentChanges) > 0 {
		s.documentsMu.Lock()
		s.documents[params.TextDocument.URI] = params.ContentChanges[0].Text
		s.documentsMu.Unlock()
	}
}

func (s *LSPServer) handleDidClose(req *RPCRequest) {
	// Params is just TextDocumentIdentifier usually (DidCloseTextDocumentParams)
	// We can reuse struct or map generic
	var params struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return
	}
	s.documentsMu.Lock()
	delete(s.documents, params.TextDocument.URI)
	s.documentsMu.Unlock()
}

func (s *LSPServer) handleDidSave(req *RPCRequest) {
	var params DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return
	}

	s.documentsMu.RLock()
	text, ok := s.documents[params.TextDocument.URI]
	s.documentsMu.RUnlock()

	if !ok {
		return
	}

	// Run analysis in goroutine
	go s.runDiagnostics(params.TextDocument.URI, text)
}

func (s *LSPServer) runDiagnostics(uri, text string) {
	path, err := uriToPath(uri)
	if err != nil {
		return
	}

	var diagnostics []Diagnostic

	// 1. Security Scan
	secResults, err := scanContentForSecurity(path, text, security.NewRegexScanner())
	if err == nil {
		for _, r := range secResults {
			diagnostics = append(diagnostics, Diagnostic{
				Range: Range{
					Start: Position{Line: r.Line - 1, Character: 0},
					End:   Position{Line: r.Line - 1, Character: 100},
				},
				Severity: 1, // Error
				Source:   "recac-security",
				Message:  r.Description,
				Code:     r.Type,
			})
		}
	}

	// 2. Smell Analysis
	smellResults, err := analyzeFileSmells(path, []byte(text))
	if err == nil {
		for _, s := range smellResults {
			diagnostics = append(diagnostics, Diagnostic{
				Range: Range{
					Start: Position{Line: s.Line - 1, Character: 0},
					End:   Position{Line: s.Line - 1, Character: 100},
				},
				Severity: 2, // Warning
				Source:   "recac-smell",
				Message:  fmt.Sprintf("%s: %d (Max %d)", s.Type, s.Value, s.Threshold),
				Code:     "SMELL",
			})
		}
	}

	// Publish
	s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

func uriToPath(uri string) (string, error) {
	if strings.HasPrefix(uri, "file://") {
		parsed, err := url.Parse(uri)
		if err != nil {
			return "", err
		}
		return filepath.FromSlash(parsed.Path), nil
	}
	return uri, nil
}
