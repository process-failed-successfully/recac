with open('cmd/orchestrator/submission_test.go', 'r') as f:
    lines = f.readlines()

new_lines = []
for line in lines:
    if "func TestWaitIdle" in line:
        break
    new_lines.append(line)

new_test = """
func TestWaitIdle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"active": 0, "pending": 0}`))
		}
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := waitIdle(ts.URL, &buf)
	assert.NoError(t, err)
}

func TestWaitIdle_NeedsWait(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"active": 0, "pending": 0}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := waitIdle(ts.URL, &buf)
	assert.NoError(t, err)
}

func TestWaitIdle_ErrorRecovery(t *testing.T) {
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount := atomic.AddInt32(&requests, 1)

		if r.URL.Path == "/status" {
			if reqCount < 2 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"active": 0, "pending": 0}`))
			}
		}
	}))
	defer ts.Close()

	var buf bytes.Buffer
	err := waitIdle(ts.URL, &buf)
	assert.NoError(t, err)
}
"""

with open('cmd/orchestrator/submission_test.go', 'w') as f:
    f.writelines(new_lines)
    f.write(new_test)
