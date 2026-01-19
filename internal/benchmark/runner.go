package benchmark

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Runner defines the interface for running benchmarks.
type Runner interface {
	Run(ctx context.Context, packagePath string) ([]Result, error)
}

// GoRunner implements Runner using the 'go test' command.
type GoRunner struct{}

var (
	// Regex to parse standard Go benchmark output
	// BenchmarkName-8   1000000   1000 ns/op   100 B/op   10 allocs/op
	benchRegex = regexp.MustCompile(`^(Benchmark\w+)(?:-\d+)?\s+(\d+)\s+([\d\.]+)\s+ns/op(?:\s+([\d\.]+)\s+MB/s)?(?:\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op)?`)
)

func NewGoRunner() *GoRunner {
	return &GoRunner{}
}

func (r *GoRunner) Run(ctx context.Context, packagePath string) ([]Result, error) {
	// go test -bench=. -benchmem -run=^$ <packagePath>
	args := []string{"test", "-bench=.", "-benchmem", "-run=^$", packagePath}
	cmd := exec.CommandContext(ctx, "go", args...)

	// Capture output
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out // Capture stderr too just in case, or maybe separate? standard go test output goes to stdout usually.

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("benchmark execution failed: %w\nOutput:\n%s", err, out.String())
	}

	return ParseOutput(out.String()), nil
}

// ParseOutput parses standard Go benchmark output.
func ParseOutput(output string) []Result {
	var results []Result
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		matches := benchRegex.FindStringSubmatch(line)
		if matches != nil {
			res := Result{
				Name: matches[1],
			}

			// Parse iterations
			if val, err := strconv.ParseInt(matches[2], 10, 64); err == nil {
				res.Iterations = val
			}

			// Parse ns/op
			if val, err := strconv.ParseFloat(matches[3], 64); err == nil {
				res.NsPerOp = val
			}

			// Parse MB/s (optional)
			if len(matches) > 4 && matches[4] != "" {
				if val, err := strconv.ParseFloat(matches[4], 64); err == nil {
					res.MBPerSec = val
				}
			}

			// Parse B/op (optional)
			if len(matches) > 5 && matches[5] != "" {
				if val, err := strconv.ParseInt(matches[5], 10, 64); err == nil {
					res.BytesPerOp = val
				}
			}

			// Parse allocs/op (optional)
			if len(matches) > 6 && matches[6] != "" {
				if val, err := strconv.ParseInt(matches[6], 10, 64); err == nil {
					res.AllocsPerOp = val
				}
			}

			results = append(results, res)
		}
	}

	return results
}
