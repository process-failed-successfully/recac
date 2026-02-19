package vuln

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOSVClient_Scan_Empty(t *testing.T) {
	client := NewOSVClient()
	vulns, err := client.Scan(nil)
	assert.NoError(t, err)
	assert.Nil(t, vulns)

	vulns, err = client.Scan([]Package{})
	assert.NoError(t, err)
	assert.Nil(t, vulns)
}

type ErrorTransport struct{}

func (t *ErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, errors.New("network failure")
}

func TestOSVClient_Scan_NetworkError(t *testing.T) {
	client := NewOSVClient()
	client.HTTPClient.Transport = &ErrorTransport{}

	_, err := client.Scan([]Package{{Name: "pkg"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OSV API request failed")
	assert.Contains(t, err.Error(), "network failure")
}

func TestOSVClient_Scan_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	client := NewOSVClient()
	client.APIURL = ts.URL

	_, err := client.Scan([]Package{{Name: "pkg"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode OSV response")
}
