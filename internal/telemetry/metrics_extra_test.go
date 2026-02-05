package telemetry

import (
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestStartMetricsServer_PortContention(t *testing.T) {
	// 1. Occupy the base port
	basePort := 10000
	l, err := net.Listen("tcp", ":"+strconv.Itoa(basePort))
	if err != nil {
		t.Fatalf("Failed to bind base port: %v", err)
	}
	defer l.Close()

	// 2. Start Metrics Server in background
	// It should pick 10001

	// We need to wait for it to start, but StartMetricsServer blocks.
	// We can't easily signal success from inside StartMetricsServer without modifying it.
	// However, we can check if port 10001 is listening after a short delay.

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// This will block until we close the listener or cancel (if it supported context, but it doesn't)
		// It uses http.Serve which returns error on Close.
		// We'll close the server by ... checking later?
		// StartMetricsServer doesn't return the server instance, so we can't Close() it gracefully!
		// This makes testing hard.
		// But for coverage, we just need to hit the "port occupied" branch.

		// Reset state
		metricsMu.Lock()
		metricsRunning = false
		metricsMu.Unlock()

		// Start
		// This will bind 10001 and serve.
		err := StartMetricsServer(basePort)
		if err != nil {
			// It might fail if we kill it or if binding failed
			// For this test, we expect success (nil or "Server closed")
			// But since we can't close it, it runs forever.
		}
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Check if port 10001 is open
	conn, err := net.DialTimeout("tcp", ":"+strconv.Itoa(basePort+1), 100*time.Millisecond)
	if err != nil {
		t.Errorf("Expected port %d to be listening (Metrics Server), got error: %v", basePort+1, err)
	} else {
		conn.Close()
	}

	// Clean up: We can't kill the server goroutine easily since StartMetricsServer doesn't expose Shutdown.
	// This might leak a goroutine/port in tests.
	// Ideally we should refactor StartMetricsServer to return the server or take a context.
	// But for now, we leave it running (it will die when test process exits).
	// To avoid conflicts with other tests, we used a unique port range (10000).
}

func TestStartMetricsServer_AllPortsBusy(t *testing.T) {
	basePort := 11000
	var listeners []net.Listener

	// Occupy 10 ports
	for i := 0; i < 10; i++ {
		l, err := net.Listen("tcp", ":"+strconv.Itoa(basePort+i))
		if err != nil {
			t.Fatalf("Failed to bind port %d: %v", basePort+i, err)
		}
		listeners = append(listeners, l)
	}
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()

	// Reset state
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()

	// Start Server -> Should fail
	err := StartMetricsServer(basePort)
	if err == nil {
		t.Error("Expected error when all ports are busy, got nil")
	}
}

func TestStartMetricsServer_AlreadyRunning(t *testing.T) {
	metricsMu.Lock()
	metricsRunning = true
	metricsMu.Unlock()

	// Should return nil immediately
	err := StartMetricsServer(8080)
	if err != nil {
		t.Errorf("Expected nil when already running, got %v", err)
	}

	// Reset
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()
}
