package benchmark

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseOutput(t *testing.T) {
	output := `
goos: linux
goarch: amd64
pkg: recac/internal/benchmark
cpu: Intel(R) Core(TM) i9-9900K CPU @ 3.60GHz
BenchmarkParseOutput-16    	100000000	        10.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkComplex-16        	 5000000	       250.0 ns/op	      10.0 MB/s	      64 B/op	       2 allocs/op
PASS
ok  	recac/internal/benchmark	1.500s
`
	results := ParseOutput(output)

	assert.Len(t, results, 2)

	assert.Equal(t, "BenchmarkParseOutput", results[0].Name)
	assert.Equal(t, int64(100000000), results[0].Iterations)
	assert.Equal(t, 10.5, results[0].NsPerOp)
	assert.Equal(t, int64(0), results[0].BytesPerOp)
	assert.Equal(t, int64(0), results[0].AllocsPerOp)

	assert.Equal(t, "BenchmarkComplex", results[1].Name)
	assert.Equal(t, int64(5000000), results[1].Iterations)
	assert.Equal(t, 250.0, results[1].NsPerOp)
	assert.Equal(t, 10.0, results[1].MBPerSec)
	assert.Equal(t, int64(64), results[1].BytesPerOp)
	assert.Equal(t, int64(2), results[1].AllocsPerOp)
}

func TestParseOutput_Minimal(t *testing.T) {
	output := `
BenchmarkSimple   100   200 ns/op
`
	results := ParseOutput(output)
	assert.Len(t, results, 1)
	assert.Equal(t, "BenchmarkSimple", results[0].Name)
	assert.Equal(t, int64(100), results[0].Iterations)
	assert.Equal(t, 200.0, results[0].NsPerOp)
}
