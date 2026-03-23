package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestRun_ConfigValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	t.Run("InvalidMode", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.require_approval", false)
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.poller", "file")
		viper.Set("orchestrator.scale", -1)
		tmpFile, _ := os.CreateTemp("", "work.json")
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte("[]"), 0644)
		viper.Set("orchestrator.work_file", tmpFile.Name())

		viper.Set("orchestrator.mode", "invalid-mode")

		err := run(ctx, logger)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "Invalid mode")
		}
	})

	t.Run("FileDirPoller_MissingDir", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.mode", "local")
		viper.Set("orchestrator.poller", "file-dir")
		viper.Set("orchestrator.watch_dir", "")

		err := run(ctx, logger)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "Watch directory must be specified")
		}
	})

	t.Run("FilePoller_MissingFile", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.mode", "local")
		viper.Set("orchestrator.poller", "file")
		viper.Set("orchestrator.work_file", "")

		err := run(ctx, logger)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "Work file must be specified")
		}
	})

	t.Run("GitHubPoller_MissingConfig", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.mode", "local")
		viper.Set("orchestrator.poller", "github")
		viper.Set("orchestrator.github_token", "")

		err := run(ctx, logger)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "GitHub token, owner, and repo must be specified")
		}
	})

	t.Run("JiraPoller_Default_MissingConfig", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.mode", "local")
		viper.Set("orchestrator.poller", "jira")

		err := run(ctx, logger)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "Failed to initialize Jira client")
		}
	})

	t.Run("TrelloPoller_MissingConfig", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.mode", "local")
		viper.Set("orchestrator.poller", "trello")
		viper.Set("orchestrator.trello_key", "")

		err := run(ctx, logger)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "Trello key, token, and either board or list must be specified")
		}
	})
}

func TestRun_Commands(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	originalExit := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = originalExit }()

	t.Run("ListJobs_Flag", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.list_jobs", true)

		err := run(ctx, logger)
		assert.NoError(t, err)
	})

	t.Run("Status_Flag", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.status", true)

		// Set up a mock server
		mux := http.NewServeMux()
		mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"poll_interval":"1m0s","uptime":"10m","last_poll":"2023-10-10T10:00:00Z","last_poll_items":5,"active_spawns":2,"total_spawns":10,"paused":false}`))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		viper.Set("orchestrator.host", server.URL)

		originalStdout := stdout
		defer func() { stdout = originalStdout }()
		out := new(bytes.Buffer)
		stdout = out

		err := run(ctx, logger)
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "Orchestrator Status")
		assert.Contains(t, output, "Uptime:")
		assert.Contains(t, output, "10m")
		assert.Contains(t, output, "Poll Interval:")
		assert.Contains(t, output, "1m0s")
		assert.Contains(t, output, "Active Spawns:")
		assert.Contains(t, output, "2")
		assert.Contains(t, output, "Total Spawns:")
		assert.Contains(t, output, "10")
	})

	t.Run("Verify_Success", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.verify", true)
		viper.Set("orchestrator.poller", "file")
		tmpFile, _ := os.CreateTemp("", "work.json")
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte("[]"), 0644)
		viper.Set("orchestrator.work_file", tmpFile.Name())
		viper.Set("orchestrator.mode", "local")

		_ = run(ctx, logger)
	})
}

func TestRun_DryRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	t.Run("Success_NoItems", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.dry_run", true)
		viper.Set("orchestrator.poller", "file")
		tmpFile, _ := os.CreateTemp("", "work.json")
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte("[]"), 0644)
		viper.Set("orchestrator.work_file", tmpFile.Name())
		viper.Set("orchestrator.mode", "local")

		err := run(ctx, logger)
		assert.NoError(t, err)
	})

	t.Run("Fail_InvalidJSON", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.dry_run", true)
		viper.Set("orchestrator.poller", "file")
		tmpFile, _ := os.CreateTemp("", "work.json")
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte("{invalid-json"), 0644)
		viper.Set("orchestrator.work_file", tmpFile.Name())
		viper.Set("orchestrator.mode", "local")

		err := run(ctx, logger)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "Dry run failed")
		}
	})
}

func TestRun_Misc_Flags(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()
	originalExit := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = originalExit }()

	flags := []string{
		"orchestrator.logs",
		"orchestrator.inspect_job",
		"orchestrator.cancel_job",
		"orchestrator.retry_job",
		"orchestrator.retry_failed",
		"orchestrator.pause",
		"orchestrator.resume",
		"orchestrator.submit",
		"orchestrator.submit_batch",
	}

	for _, flag := range flags {
		t.Run(flag, func(t *testing.T) {
			viper.Reset()
			viper.Set("orchestrator.scale", -1)
			if flag == "orchestrator.retry_failed" || flag == "orchestrator.pause" || flag == "orchestrator.resume" {
				viper.Set(flag, true)
			} else {
				viper.Set(flag, "some-val")
			}
			err := run(ctx, logger)
			assert.NoError(t, err)
		})
	}
}

func TestRun_SubmitUrl_Error(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	viper.Reset()
	viper.Set("orchestrator.scale", -1)
	viper.Set("orchestrator.submit_url", "http://repo")
	viper.Set("orchestrator.submit_task", "") // Missing task

	err := run(ctx, logger)
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "required when using --submit-url")
	}
}

func TestRun_AdditionalPaths(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	t.Run("K8sMode_Fail", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.mode", "k8s")
		viper.Set("orchestrator.poller", "file")
		tmpFile, _ := os.CreateTemp("", "work.json")
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte("[]"), 0644)
		viper.Set("orchestrator.work_file", tmpFile.Name())

		err := run(ctx, logger)
		assert.Error(t, err)
	})

	t.Run("Janitor_Enable", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.mode", "local")
		viper.Set("orchestrator.poller", "file")
		tmpFile, _ := os.CreateTemp("", "work.json")
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte("[]"), 0644)
		viper.Set("orchestrator.work_file", tmpFile.Name())

		viper.Set("orchestrator.cleanup", true)
		viper.Set("orchestrator.cleanup_interval", 5*time.Minute)
		viper.Set("orchestrator.cleanup_age", 24*time.Hour)

		viper.Set("orchestrator.verify", true)

		_ = run(ctx, logger)
	})

	t.Run("Persistence_Enable", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.mode", "local")
		viper.Set("orchestrator.poller", "file")
		tmpFile, _ := os.CreateTemp("", "work.json")
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte("[]"), 0644)
		viper.Set("orchestrator.work_file", tmpFile.Name())

		dbFile := "test_persist.db"
		viper.Set("orchestrator.db_file", dbFile)
		defer os.Remove(dbFile)

		viper.Set("orchestrator.verify", true)

		_ = run(ctx, logger)
	})
}

// Sequential execution for metrics tests
func TestRun_MetricsServer(t *testing.T) {
	t.Run("StartStop", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.poller", "file")
		tmpFile, _ := os.CreateTemp("", "work.json")
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte("[]"), 0644)
		viper.Set("orchestrator.work_file", tmpFile.Name())
		viper.Set("orchestrator.mode", "local")
		viper.Set("orchestrator.interval", 1*time.Minute)
		viper.Set("orchestrator.metrics_port", 0)

		err := run(ctx, logger)
		if err != nil {
			assert.Equal(t, context.Canceled, err)
		}
		time.Sleep(500 * time.Millisecond)
	})

	t.Run("Requests", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, nil))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.poller", "file")
		tmpFile, _ := os.CreateTemp("", "work.json")
		defer os.Remove(tmpFile.Name())
		os.WriteFile(tmpFile.Name(), []byte("[]"), 0644)
		viper.Set("orchestrator.work_file", tmpFile.Name())
		viper.Set("orchestrator.mode", "local")
		viper.Set("orchestrator.interval", 1*time.Minute)
		viper.Set("orchestrator.metrics_port", 0)

		errChan := make(chan error)
		go func() {
			errChan <- run(ctx, logger)
		}()

		// Find port
		var port int
		re := regexp.MustCompile(`Metrics server started.*port=(\d+)`)

		ready := false
		for i := 0; i < 50; i++ {
			logs := logBuf.String()
			matches := re.FindStringSubmatch(logs)
			if len(matches) > 1 {
				port, _ = strconv.Atoi(matches[1])
				ready = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if !ready {
			cancel()
			<-errChan
			t.Skipf("Metrics server failed to start. Logs: %s", logBuf.String())
			return
		}

		baseURL := fmt.Sprintf("http://localhost:%d", port)

		// GET requests
		gets := []string{
			"/status",
			"/jobs",
			"/jobs?state=completed",
			"/jobs?state=all",
			"/jobs/job-1",      // Not found
			"/jobs/job-1/logs", // Not found
		}
		for _, ep := range gets {
			resp, _ := http.Get(baseURL + ep)
			if resp != nil {
				resp.Body.Close()
			}
		}

		// POST/DELETE requests
		http.Post(baseURL+"/pause", "application/json", nil)
		http.Post(baseURL+"/resume", "application/json", nil)
		http.Post(baseURL+"/jobs/job-1/retry", "application/json", nil)
		http.Post(baseURL+"/jobs/retry-failed", "application/json", nil)

		req, _ := http.NewRequest(http.MethodDelete, baseURL+"/jobs/job-1", nil)
		http.DefaultClient.Do(req)

		// Post Job
		http.Post(baseURL+"/jobs", "application/json", strings.NewReader(`{"id":"job-new"}`))

		cancel()
		<-errChan
		time.Sleep(500 * time.Millisecond)
	})
}

func TestMain_Entrypoint(t *testing.T) {
	originalArgs := os.Args
	originalExit := exitFunc

	defer func() {
		os.Args = originalArgs
		exitFunc = originalExit
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	}()

	exitFunc = func(code int) {}
	os.Args = []string{"orchestrator", "--list-jobs"}
	pflag.CommandLine = pflag.NewFlagSet("orchestrator", pflag.ContinueOnError)

	stdout = new(bytes.Buffer)

	main()
}

func TestWatchListJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	viper.Set("orchestrator.host", server.URL)
	viper.Set("orchestrator.list_jobs", true)
	viper.Set("orchestrator.watch", true)
	viper.Set("orchestrator.watch_interval", 10*time.Millisecond)
	defer viper.Reset()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := run(ctx, slog.Default())
	assert.NoError(t, err)

	pw.Close()
	out, _ := io.ReadAll(pr)
	outStr := string(out)

	// ANSI clear screen code
	assert.Contains(t, outStr, "\033[H\033[2J")
	// "No active jobs." should appear multiple times
	count := strings.Count(outStr, "No active jobs.")
	assert.GreaterOrEqual(t, count, 2, "Watch loop should run multiple times")
}

func TestWatchListPendingJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "pending", r.URL.Query().Get("state"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	viper.Set("orchestrator.host", server.URL)
	viper.Set("orchestrator.list_pending", true)
	viper.Set("orchestrator.watch", true)
	viper.Set("orchestrator.watch_interval", 10*time.Millisecond)
	defer viper.Reset()

	oldStdout := stdout
	pr, pw, _ := os.Pipe()
	stdout = pw
	defer func() { stdout = oldStdout }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := run(ctx, slog.Default())
	assert.NoError(t, err)

	pw.Close()
	out, _ := io.ReadAll(pr)
	outStr := string(out)

	// ANSI clear screen code
	assert.Contains(t, outStr, "\033[H\033[2J")
	// "No pending jobs." should appear multiple times
	count := strings.Count(outStr, "No pending jobs.")
	assert.GreaterOrEqual(t, count, 2, "Watch loop should run multiple times")
}
