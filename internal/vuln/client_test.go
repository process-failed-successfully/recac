package vuln

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestOSVClient_Scan(t *testing.T) {
	// Mock OSV API
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/querybatch" {
			t.Errorf("Expected path /v1/querybatch, got %s", r.URL.Path)
		}

		// Decode request to verify
		var req struct {
			Queries []osvQuery `json:"queries"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Queries) != 2 {
			t.Errorf("Expected 2 queries, got %d", len(req.Queries))
		}
		if req.Queries[0].Package.Name != "vulnerable-pkg" {
			t.Errorf("Expected first package to be vulnerable-pkg")
		}

		// Response
		resp := osvBatchResponse{
			Results: []osvResult{
				{
					Vulns: []osvVuln{
						{
							ID:      "GHSA-123",
							Summary: "Bad vulnerability",
							Details: "Very bad",
							Published: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
						},
					},
				},
				{
					Vulns: []osvVuln{}, // safe-pkg
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewOSVClient()
	client.APIURL = ts.URL + "/v1/querybatch" // Override URL

	pkgs := []Package{
		{Name: "vulnerable-pkg", Version: "1.0.0", Ecosystem: "Go"},
		{Name: "safe-pkg", Version: "2.0.0", Ecosystem: "npm"},
	}

	vulns, err := client.Scan(pkgs)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(vulns) != 1 {
		t.Errorf("Expected 1 vulnerability, got %d", len(vulns))
	}

	if vulns[0].ID != "GHSA-123" {
		t.Errorf("Expected ID GHSA-123, got %s", vulns[0].ID)
	}
	if vulns[0].PackageName != "vulnerable-pkg" {
		t.Errorf("Expected PackageName vulnerable-pkg, got %s", vulns[0].PackageName)
	}
}

func TestParsers(t *testing.T) {
	// Test GoModParser
	goModContent := `
module example.com/test

go 1.21

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/text v0.3.7 // indirect
)

require github.com/stretchr/testify v1.8.0
`
	// Create temp file
	tmpGoMod, _ := writeTempFile(t, "go.mod", goModContent)

	goParser := &GoModParser{}
	goPkgs, err := goParser.Parse(tmpGoMod)
	if err != nil {
		t.Fatalf("GoModParser failed: %v", err)
	}

	expectedGo := map[string]string{
		"github.com/pkg/errors":       "v0.9.1",
		"golang.org/x/text":           "v0.3.7",
		"github.com/stretchr/testify": "v1.8.0",
	}

	if len(goPkgs) != 3 {
		t.Errorf("Expected 3 Go packages, got %d", len(goPkgs))
	}

	for _, p := range goPkgs {
		if v, ok := expectedGo[p.Name]; !ok {
			t.Errorf("Unexpected package: %s", p.Name)
		} else if p.Version != v {
			t.Errorf("Package %s: expected version %s, got %s", p.Name, v, p.Version)
		}
		if p.Ecosystem != "Go" {
			t.Errorf("Package %s: expected ecosystem Go, got %s", p.Name, p.Ecosystem)
		}
	}

	// Test PackageJsonParser
	pkgJsonContent := `
{
  "name": "test-project",
  "dependencies": {
    "react": "^18.0.0",
    "lodash": "4.17.21"
  },
  "devDependencies": {
    "jest": "~29.0.0"
  }
}
`
	tmpPkgJson, _ := writeTempFile(t, "package.json", pkgJsonContent)

	npmParser := &PackageJsonParser{}
	npmPkgs, err := npmParser.Parse(tmpPkgJson)
	if err != nil {
		t.Fatalf("PackageJsonParser failed: %v", err)
	}

	expectedNpm := map[string]string{
		"react":  "18.0.0",
		"lodash": "4.17.21",
		"jest":   "29.0.0",
	}

	if len(npmPkgs) != 3 {
		t.Errorf("Expected 3 npm packages, got %d", len(npmPkgs))
	}

	for _, p := range npmPkgs {
		if v, ok := expectedNpm[p.Name]; !ok {
			t.Errorf("Unexpected package: %s", p.Name)
		} else if p.Version != v {
			t.Errorf("Package %s: expected version %s, got %s", p.Name, v, p.Version)
		}
		if p.Ecosystem != "npm" {
			t.Errorf("Package %s: expected ecosystem npm, got %s", p.Name, p.Ecosystem)
		}
	}
}

func writeTempFile(t *testing.T, name, content string) (string, func()) {
	t.Helper()
	f, err := createTempFile(name, content)
	if err != nil {
		t.Fatal(err)
	}
	return f.Name(), func() { os.Remove(f.Name()) }
}

func createTempFile(name, content string) (*os.File, error) {
	// We can't easily control the name in MkdirTemp without subdirs,
	// but parsers take a path, so filename doesn't strictly matter unless we check extension logic inside parser
	// The parsers implemented don't check extension themselves, runVulnScan does.
	// But `cleanNpmVersion` test uses generic names.

	// Create a temp file with any name
	f, err := os.CreateTemp("", "*"+name)
	if err != nil {
		return nil, err
	}
	if _, err := f.WriteString(content); err != nil {
		return nil, err
	}
	f.Close()
	return f, nil
}
