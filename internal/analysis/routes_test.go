package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRoutes(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "routes_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a Go file with Gin routes
	ginFile := `
package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/ping", pingHandler)
	r.POST("/users", createUser)
	r.DELETE("/users/:id", deleteUser)
}

func pingHandler(c *gin.Context) {}
func createUser(c *gin.Context) {}
func deleteUser(c *gin.Context) {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "gin.go"), []byte(ginFile), 0644); err != nil {
		t.Fatalf("failed to write gin.go: %v", err)
	}

	// Create a Go file with Stdlib routes
	stdlibFile := `
package main

import "net/http"

func main() {
	http.HandleFunc("/health", healthCheck)
	http.Handle("/metrics", metricsHandler)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {}
var metricsHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
`
	if err := os.WriteFile(filepath.Join(tmpDir, "stdlib.go"), []byte(stdlibFile), 0644); err != nil {
		t.Fatalf("failed to write stdlib.go: %v", err)
	}

	// Create a Go file with Echo routes
	echoFile := `
package main

import "github.com/labstack/echo/v4"

func main() {
	e := echo.New()
	e.GET("/", func(c echo.Context) error { return nil })
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "echo.go"), []byte(echoFile), 0644); err != nil {
		t.Fatalf("failed to write echo.go: %v", err)
	}

	// Run ScanRoutes
	routes, err := ScanRoutes(tmpDir)
	if err != nil {
		t.Fatalf("ScanRoutes failed: %v", err)
	}

	// Expectations
	expected := map[string]string{
		"GET /ping":         "pingHandler",
		"POST /users":       "createUser",
		"DELETE /users/:id": "deleteUser",
		"ANY /health":       "healthCheck",    // HandleFunc -> ANY
		"ANY /metrics":      "metricsHandler", // Handle -> ANY
		"GET /":             "(anonymous)",
	}

	found := make(map[string]string)
	for _, r := range routes {
		key := r.Method + " " + r.Path
		found[key] = r.Handler
	}

	for k, v := range expected {
		if got, ok := found[k]; !ok {
			t.Errorf("Missing route: %s", k)
		} else if got != v {
			t.Errorf("Route %s: expected handler %s, got %s", k, v, got)
		}
	}

	if len(found) != len(expected) {
		t.Errorf("Expected %d routes, got %d", len(expected), len(found))
	}
}

func TestScanRoutes_ErrorsAndEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Test walk error by passing a file instead of a dir or a non-existent path
	_, err := ScanRoutes(filepath.Join(tmpDir, "does-not-exist"))
	if err == nil {
		t.Errorf("Expected error for non-existent root")
	}

	// Test malformed file
	malformedFile := `package main
func main() {
	unclosed := "`
	if err := os.WriteFile(filepath.Join(tmpDir, "malformed.go"), []byte(malformedFile), 0644); err != nil {
		t.Fatalf("failed to write malformed.go: %v", err)
	}

	// Test ignored dirs
	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.Mkdir(vendorDir, 0755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "ignore.go"), []byte(`package main`), 0644); err != nil {
		t.Fatalf("failed to write ignore.go: %v", err)
	}

	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.Mkdir(hiddenDir, 0755); err != nil {
		t.Fatalf("failed to create hidden dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "hidden.go"), []byte(`package main`), 0644); err != nil {
		t.Fatalf("failed to write hidden.go: %v", err)
	}

	// Test non-go file
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte(`hello`), 0644); err != nil {
		t.Fatalf("failed to write readme.md: %v", err)
	}

	routes, err := ScanRoutes(tmpDir)
	if err != nil {
		t.Fatalf("ScanRoutes failed: %v", err)
	}

	if len(routes) != 0 {
		t.Errorf("Expected 0 routes from malformed/ignored files, got %d", len(routes))
	}
}

func TestExtractSourceSnippet_Error(t *testing.T) {
	snippet := extractSourceSnippet("/non/existent/path/to/file.go", 1)
	if snippet != "" {
		t.Errorf("Expected empty snippet for non-existent file, got %q", snippet)
	}

	// Line out of bounds
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "dummy.go")
	os.WriteFile(filePath, []byte("package main\n"), 0644)
	snippet = extractSourceSnippet(filePath, 10)
	if snippet != "" {
		t.Errorf("Expected empty snippet for out-of-bounds line, got %q", snippet)
	}
}

func TestScanRoutes_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	edgeCasesFile := `
package main

import "github.com/gin-gonic/gin"

type MyHandler struct{}
func (h *MyHandler) Handle(c *gin.Context) {}

func main() {
	r := gin.Default()

	// Not enough args
	r.GET()
	r.GET(123)

	// SelectorExpr handler
	h := &MyHandler{}
	r.GET("/sel", h.Handle)

	// Complex handler
	r.GET("/complex", func() gin.HandlerFunc {
		return func(c *gin.Context) {}
	}())

	// Complex SelectorExpr where X is not Ident
	r.GET("/complexsel", (&MyHandler{}).Handle)

	// Non-string arg (should be skipped)
	r.GET(123)

	// Variable arg (not a BasicLit)
	var path string = "/varpath"
	r.GET(path, h.Handle)

	// Path without slash
	r.GET("noslash", h.Handle)

	// Unknown method
	r.UNKNOWN("/unknown", h.Handle)

	// Ident method that matches one of the expected methods when upper-cased
	var get = r.GET
	get("/ident", func(c *gin.Context) {})
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "edge.go"), []byte(edgeCasesFile), 0644); err != nil {
		t.Fatalf("failed to write edge.go: %v", err)
	}

	routes, err := ScanRoutes(tmpDir)
	if err != nil {
		t.Fatalf("ScanRoutes failed: %v", err)
	}

	expected := map[string]string{
		"GET /sel":        "h.Handle",
		"GET /complex":    "(complex)",
		"GET /complexsel": "(complex).Handle",
		"GET /ident":      "(anonymous)", // 'get' upper-cased matches "GET" in normalizeMethod
	}

	found := make(map[string]string)
	for _, r := range routes {
		key := r.Method + " " + r.Path
		found[key] = r.Handler
	}

	for k, v := range expected {
		if got, ok := found[k]; !ok {
			t.Errorf("Missing route: %s", k)
		} else if got != v {
			t.Errorf("Route %s: expected handler %s, got %s", k, v, got)
		}
	}
}
