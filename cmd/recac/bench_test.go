package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"recac/internal/benchmark"

	"github.com/stretchr/testify/assert"
)

type mockRunner struct {
	results []benchmark.Result
	err     error
}

func (m *mockRunner) Run(ctx context.Context, packagePath string) ([]benchmark.Result, error) {
	return m.results, m.err
}

type mockStore struct {
	saved  []benchmark.Run
	latest *benchmark.Run
}

func (m *mockStore) Save(run benchmark.Run) error {
	m.saved = append(m.saved, run)
	return nil
}

func (m *mockStore) LoadLatest() (*benchmark.Run, error) {
	return m.latest, nil
}

func (m *mockStore) LoadAll() ([]benchmark.Run, error) {
	return nil, nil
}

func TestBenchCmd(t *testing.T) {
	// Restore globals after test
	defer func() {
		newRunnerFunc = func() benchmark.Runner { return benchmark.NewGoRunner() }
		newStoreFunc = func(path string) (benchmark.Store, error) { return benchmark.NewFileStore(path) }
	}()

	// Setup Mocks
	mockR := &mockRunner{
		results: []benchmark.Result{
			{Name: "BenchmarkOne", NsPerOp: 100},
		},
	}
	mockS := &mockStore{}

	newRunnerFunc = func() benchmark.Runner { return mockR }
	newStoreFunc = func(path string) (benchmark.Store, error) { return mockS, nil }

	// Test Run
	cmd := newBenchCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Running benchmarks for .")
	assert.Contains(t, output, "BenchmarkOne")
	assert.Contains(t, output, "Results saved")
	assert.Len(t, mockS.saved, 1)
}

func TestBenchCmd_Compare(t *testing.T) {
	// Restore globals after test
	defer func() {
		newRunnerFunc = func() benchmark.Runner { return benchmark.NewGoRunner() }
		newStoreFunc = func(path string) (benchmark.Store, error) { return benchmark.NewFileStore(path) }
	}()

	// Mock previous run
	prevRun := benchmark.Run{
		Timestamp: time.Now().Add(-1 * time.Hour),
		Results: []benchmark.Result{
			{Name: "BenchmarkOne", NsPerOp: 50}, // Was faster
		},
	}

	// Setup Mocks
	mockR := &mockRunner{
		results: []benchmark.Result{
			{Name: "BenchmarkOne", NsPerOp: 100}, // Now slower
		},
	}
	mockS := &mockStore{
		latest: &prevRun,
	}

	newRunnerFunc = func() benchmark.Runner { return mockR }
	newStoreFunc = func(path string) (benchmark.Store, error) { return mockS, nil }

	// Test Run with Compare
	cmd := newBenchCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--compare"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Comparison with previous run")
	assert.Contains(t, output, "+100.00%") // 50 -> 100 is 100% slower
}

func TestBenchCmd_FailThreshold(t *testing.T) {
	// Restore globals after test
	defer func() {
		newRunnerFunc = func() benchmark.Runner { return benchmark.NewGoRunner() }
		newStoreFunc = func(path string) (benchmark.Store, error) { return benchmark.NewFileStore(path) }
	}()

	// Mock previous run
	prevRun := benchmark.Run{
		Results: []benchmark.Result{
			{Name: "BenchmarkOne", NsPerOp: 100},
		},
	}

	// Setup Mocks
	mockR := &mockRunner{
		results: []benchmark.Result{
			{Name: "BenchmarkOne", NsPerOp: 120}, // 20% slower
		},
	}
	mockS := &mockStore{
		latest: &prevRun,
	}

	newRunnerFunc = func() benchmark.Runner { return mockR }
	newStoreFunc = func(path string) (benchmark.Store, error) { return mockS, nil }

	// Test Run with Fail Threshold
	cmd := newBenchCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--compare", "--fail-threshold", "10"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "performance regression detected")
	assert.Contains(t, err.Error(), "20.00% slower")
}
