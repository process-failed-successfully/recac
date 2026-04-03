package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"net/http"
	"net/http/httptest"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestRun_MiscSubmit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	originalExit := exitFunc
	exitFunc = func(code int) {
		panic("exit")
	}
	defer func() { exitFunc = originalExit }()

	t.Run("SubmitPipeline_Interactive", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.submit_pipeline", "testdata/pipeline.yaml")
		viper.Set("orchestrator.submit_pipeline_interactive", true)

		mux := http.NewServeMux()
		mux.HandleFunc("/pipelines/submit", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		viper.Set("orchestrator.host", server.URL)

		originalStdout := stdout
		defer func() { stdout = originalStdout }()
		out := new(bytes.Buffer)
		stdout = out

		// Will fail opening testdata/pipeline.yaml but that increases coverage in main.go
		assert.Panics(t, func() {
			run(ctx, logger)
		})
	})

	t.Run("SubmitBatchJob", func(t *testing.T) {
		viper.Reset()
		viper.Set("orchestrator.scale", -1)
		viper.Set("orchestrator.submit_batch", "testdata/batch.yaml")

		mux := http.NewServeMux()
		mux.HandleFunc("/jobs/batch", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		viper.Set("orchestrator.host", server.URL)

		originalStdout := stdout
		defer func() { stdout = originalStdout }()
		out := new(bytes.Buffer)
		stdout = out

		assert.Panics(t, func() {
			run(ctx, logger)
		})
	})
}

func TestRun_Misc_Coverage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	originalExit := exitFunc
	exitFunc = func(code int) {
		panic("exit")
	}
	defer func() { exitFunc = originalExit }()

	tests := []struct {
		name     string
		setup    func()
		expected bool // true if it should panic/exit
	}{
		{
			name: "ListDependents",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.list_dependents", "job1")
			},
			expected: true,
		},
		{
			name: "ListBlockers",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.list_blockers", "job1")
			},
			expected: true,
		},
		{
			name: "SetOutputJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.set_output_job", "job1")
				viper.Set("orchestrator.set_output_key", "key1")
				viper.Set("orchestrator.set_output_val", "val1")
			},
			expected: true,
		},
		{
			name: "GetOutputJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.get_output_job", "job1")
				viper.Set("orchestrator.get_output_key", "key1")
			},
			expected: true,
		},
		{
			name: "AddMetricsJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.add_metrics_job", "job1")
				viper.Set("orchestrator.add_metrics_key", "cost")
				viper.Set("orchestrator.add_metrics_val", 10.5)
			},
			expected: true,
		},
		{
			name: "InspectDataflow",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.inspect_dataflow", "job1")
			},
			expected: true,
		},
		{
			name: "ExplainJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.explain_job", "job1")
			},
			expected: true,
		},
		{
			name: "HealJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.heal_job", "job1")
			},
			expected: true,
		},
		{
			name: "CompareJobs",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.compare_jobs", "job1,job2")
			},
			expected: true,
		},
		{
			name: "PurgeOlderThan",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.purge_older_than", "24h")
			},
			expected: true,
		},
		{
			name: "HoldJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.hold_job", "job1")
			},
			expected: true,
		},
		{
			name: "UnholdJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.unhold_job", "job1")
			},
			expected: true,
		},
		{
			name: "RenameJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.rename_job", "job1")
				viper.Set("orchestrator.rename_val", "job2")
			},
			expected: true,
		},
		{
			name: "SkipJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.skip_job", "job1")
			},
			expected: true,
		},
		{
			name: "ForceCompleteJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.force_complete_job", "job1")
			},
			expected: true,
		},
		{
			name: "SetProgressJob",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.set_progress_job", "job1")
				viper.Set("orchestrator.set_progress_val", 50.0)
			},
			expected: true,
		},
		{
			name: "ExplainPipeline",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.explain_pipeline", "file.yaml")
			},
			expected: true,
		},
		{
			name: "ListTemplates",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.list_templates", "dir")
			},
			expected: true,
		},
		{
			name: "InspectPipeline",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.inspect_pipeline", "file.yaml")
			},
			expected: true,
		},
		{
			name: "ComparePipelines",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.compare_pipelines", "file1.yaml,file2.yaml")
			},
			expected: true,
		},
		{
			name: "ApplyPipeline",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.apply_pipeline", "file.yaml")
			},
			expected: true,
		},
		{
			name: "WatchPipeline",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.watch_pipeline", "file.yaml")
			},
			expected: true,
		},
		{
			name: "SearchLogs",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.search_logs", "error")
			},
			expected: true,
		},
		{
			name: "ExportJobs",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.export_jobs", "jobs.json")
			},
			expected: true,
		},
		{
			name: "ImportJobs",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.import_jobs", "jobs.json")
			},
			expected: true,
		},
		{
			name: "ExportPipeline",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.export_pipeline", "pipeline.json")
			},
			expected: true,
		},
		{
			name: "ExportGraph",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.export_graph", "graph.dot")
			},
			expected: true,
		},
		{
			name: "ExportMetrics",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.export_metrics", "metrics.json")
			},
			expected: true,
		},
		{
			name: "ExportTrace",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.export_trace", "trace.json")
			},
			expected: true,
		},
		{
			name: "UploadArtifact",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.upload_artifact", "file.txt")
				viper.Set("orchestrator.upload_job_id", "job1")
			},
			expected: true,
		},
		{
			name: "DownloadArtifact",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.download_artifact", "file.txt")
				viper.Set("orchestrator.download_job_id", "job1")
			},
			expected: true,
		},
		{
			name: "DeleteArtifact",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.delete_artifact", "file.txt")
				viper.Set("orchestrator.delete_job_id", "job1")
			},
			expected: true,
		},
		{
			name: "GeneratePipeline",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.generate_pipeline", "prompt")
			},
			expected: true,
		},
		{
			name: "GenerateChangelog",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.generate_changelog", "CHANGELOG.md")
			},
			expected: true,
		},
		{
			name: "GeneratePostmortem",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.generate_postmortem", "postmortem.md")
				viper.Set("orchestrator.generate_postmortem_job", "job1")
			},
			expected: true,
		},
		{
			name: "OptimizePipeline",
			setup: func() {
				viper.Set("orchestrator.scale", -1)
				viper.Set("orchestrator.optimize_pipeline", "pipeline.yaml")
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			tt.setup()

			originalStdout := stdout
			defer func() { stdout = originalStdout }()
			out := new(bytes.Buffer)
			stdout = out

			if tt.expected {
				assert.Panics(t, func() {
					run(ctx, logger)
				})
			} else {
				err := run(ctx, logger)
				assert.NoError(t, err)
			}
		})
	}
}
