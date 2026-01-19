package benchmark

import "time"

// Result represents a single benchmark result.
type Result struct {
	Name        string  `json:"name"`
	Iterations  int64   `json:"iterations"`
	NsPerOp     float64 `json:"ns_per_op"`
	MBPerSec    float64 `json:"mb_per_sec,omitempty"`
	BytesPerOp  int64   `json:"bytes_per_op"`
	AllocsPerOp int64   `json:"allocs_per_op"`
}

// Run represents a collection of benchmark results from a single execution.
type Run struct {
	Timestamp time.Time `json:"timestamp"`
	Commit    string    `json:"commit,omitempty"` // Git commit hash
	Results   []Result  `json:"results"`
}
