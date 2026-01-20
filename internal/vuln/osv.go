package vuln

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const osvQueryBatchURL = "https://api.osv.dev/v1/querybatch"

// OSVClient checks for vulnerabilities using OSV API.
type OSVClient struct {
	HTTPClient *http.Client
	APIURL     string
}

func NewOSVClient() *OSVClient {
	return &OSVClient{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		APIURL:     osvQueryBatchURL,
	}
}

type osvQuery struct {
	Package osvPackage `json:"package,omitempty"`
	Version string     `json:"version,omitempty"`
}

type osvPackage struct {
	Name      string `json:"name,omitempty"`
	Ecosystem string `json:"ecosystem,omitempty"`
}

type osvBatchResponse struct {
	Results []osvResult `json:"results"`
}

type osvResult struct {
	Vulns []osvVuln `json:"vulns"`
}

type osvVuln struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	Details   string    `json:"details"`
	Published time.Time `json:"published"`
	References []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"references"`
	DatabaseSpecific struct {
		Severity string `json:"severity"` // Not standard, but sometimes present
	} `json:"database_specific"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
}

func (c *OSVClient) Scan(packages []Package) ([]Vulnerability, error) {
	if len(packages) == 0 {
		return nil, nil
	}

	// Prepare queries
	var queries []osvQuery
	// We need to batch requests if too many? API allows 1000 per call.
	// For now, assume < 1000 deps.

	for _, p := range packages {
		queries = append(queries, osvQuery{
			Package: osvPackage{
				Name:      p.Name,
				Ecosystem: p.Ecosystem,
			},
			Version: p.Version,
		})
	}

	reqBody := map[string]interface{}{
		"queries": queries,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.APIURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("OSV API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSV API returned status: %s", resp.Status)
	}

	var batchResp osvBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to decode OSV response: %w", err)
	}

	var results []Vulnerability

	for i, res := range batchResp.Results {
		if len(res.Vulns) > 0 {
			pkg := packages[i]
			for _, v := range res.Vulns {
				vuln := Vulnerability{
					ID:          v.ID,
					Summary:     v.Summary,
					Details:     v.Details,
					Published:   v.Published,
					PackageName: pkg.Name,
					Ecosystem:   pkg.Ecosystem,
				}

				// Normalize References
				for _, ref := range v.References {
					vuln.References = append(vuln.References, ref.URL)
				}

				// Determine Severity
				// Try CVSS score first
				for _, s := range v.Severity {
					if s.Type == "CVSS_V3" || s.Type == "CVSS_V2" {
						// We won't parse the vector string here, just store it or use database specific field if available
						// Actually, OSV "database_specific" isn't always reliable across ecosystems.
						// Let's just default to "UNKNOWN" unless we find something interesting.
						vuln.Severity = s.Score // This is the vector, e.g. CVSS:3.1/AV:N...
					}
				}

				// Very crude severity mapping if we don't have better info
				// or if vector is present, maybe we can label it roughly?
				// For now, just leave it as is.

				results = append(results, vuln)
			}
		}
	}

	return results, nil
}
