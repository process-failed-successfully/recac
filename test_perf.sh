go test -bench=BenchmarkParseBenchOutput ./cmd/recac/ -benchmem -run=^$ || echo "No benchmark"
