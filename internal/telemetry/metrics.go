package telemetry

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartMetricsServer starts a HTTP server exposing Prometheus metrics.
func StartMetricsServer(addr string) error {
	http.Handle("/metrics", promhttp.Handler())
	
	fmt.Printf("Starting metrics server on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}
