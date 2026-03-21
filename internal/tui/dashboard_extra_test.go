package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCancelJobDownstream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/jobs/123?downstream=true", r.URL.String())
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cmd := cancelJobDownstream(ts.URL, "123")
	msg := cmd()

	switch m := msg.(type) {
	case actionMsg:
		assert.NoError(t, m.Err)
		assert.Equal(t, "Cancelled (Downstream)", m.Message)
	default:
		t.Fatalf("Unexpected message type: %T", msg)
	}
}

func TestCancelJobDownstream_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := cancelJobDownstream(ts.URL, "123")
	msg := cmd()

	switch m := msg.(type) {
	case actionMsg:
		assert.Error(t, m.Err)
		assert.Equal(t, "status 500", m.Err.Error())
	default:
		t.Fatalf("Unexpected message type: %T", msg)
	}
}

func TestRetryJobDownstream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/jobs/123/retry?downstream=true", r.URL.String())
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	cmd := retryJobDownstream(ts.URL, "123")
	msg := cmd()

	switch m := msg.(type) {
	case actionMsg:
		assert.NoError(t, m.Err)
		assert.Equal(t, "Retry submitted (Downstream)", m.Message)
	default:
		t.Fatalf("Unexpected message type: %T", msg)
	}
}

func TestRetryJobDownstream_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cmd := retryJobDownstream(ts.URL, "123")
	msg := cmd()

	switch m := msg.(type) {
	case actionMsg:
		assert.Error(t, m.Err)
		assert.Equal(t, "status 500", m.Err.Error())
	default:
		t.Fatalf("Unexpected message type: %T", msg)
	}
}
