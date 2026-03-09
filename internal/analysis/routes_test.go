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
