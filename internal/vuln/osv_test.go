package vuln

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOSVClient_Scan_EdgeCases(t *testing.T) {
	// 1. Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Request
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		queries := reqBody["queries"].([]interface{})

		// Response based on input count
		if len(queries) == 1 {
			// Success Case
			resp := osvBatchResponse{
				Results: []osvResult{
					{
						Vulns: []osvVuln{
							{
								ID:      "GHSA-123",
								Summary: "Test Vuln",
								Severity: []struct {
									Type  string `json:"type"`
									Score string `json:"score"`
								}{
									{Type: "CVSS_V3", Score: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"},
								},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			// Empty Response or other logic
			json.NewEncoder(w).Encode(osvBatchResponse{Results: []osvResult{}})
		}
	}))
	defer ts.Close()

	// 2. Init Client
	client := NewOSVClient()
	client.APIURL = ts.URL // Override URL

	// 3. Test Cases

	// Case A: Empty Input
	vulns, err := client.Scan(nil)
	assert.NoError(t, err)
	assert.Nil(t, vulns)

	// Case B: Success Scan
	pkgs := []Package{
		{Name: "vulnerable-package", Version: "1.0.0", Ecosystem: "Go"},
	}
	vulns, err = client.Scan(pkgs)
	assert.NoError(t, err)
	assert.Len(t, vulns, 1)
	assert.Equal(t, "GHSA-123", vulns[0].ID)
	assert.Equal(t, "vulnerable-package", vulns[0].PackageName)
	assert.Equal(t, "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", vulns[0].Severity)

	// Case C: Server Error
	tsError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsError.Close()
	client.APIURL = tsError.URL

	vulns, err = client.Scan(pkgs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OSV API returned status")

	// Case D: Malformed JSON
	tsBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer tsBadJSON.Close()
	client.APIURL = tsBadJSON.URL

	vulns, err = client.Scan(pkgs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode OSV response")
}
