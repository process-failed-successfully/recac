package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

// List of colors for job prefixes to make output readable
var colors = []string{
	"#00FFFF", // Cyan
	"#FF00FF", // Magenta
	"#FFFF00", // Yellow
	"#00FF00", // Green
	"#FF8C00", // DarkOrange
	"#FF1493", // DeepPink
	"#00BFFF", // DeepSkyBlue
	"#32CD32", // LimeGreen
}

type multiplexer struct {
	host         string
	tag          string
	match        string
	group        string
	active       map[string]context.CancelFunc
	mu           sync.Mutex
	wg           sync.WaitGroup
	colorCounter int
}

func tailSingleJob(ctx context.Context, host, jobID string) error {
	m := &multiplexer{
		host:   host,
		active: make(map[string]context.CancelFunc),
	}

	fmt.Fprintf(stdout, "Starting Log Stream (tailing job %s from %s)...\n", jobID, host)
	fmt.Fprintf(stdout, "Press Ctrl+C to stop.\n\n")

	jobCtx, cancel := context.WithCancel(ctx)
	m.active[jobID] = cancel

	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[0])).Bold(true)
	prefix := style.Render(fmt.Sprintf("[%s]", jobID))

	m.wg.Add(1)
	go m.tailJob(jobCtx, jobID, prefix)

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		fmt.Fprintf(stdout, "\nShutting down stream, waiting for logs to finish...\n")
		m.mu.Lock()
		if cancelFn, ok := m.active[jobID]; ok {
			cancelFn()
		}
		m.mu.Unlock()
		m.wg.Wait()
		return nil
	case <-done:
		return nil
	}
}

func tailActiveJobs(ctx context.Context, host, tag, match, group string) error {
	m := &multiplexer{
		host:   host,
		tag:    tag,
		match:  match,
		group:  group,
		active: make(map[string]context.CancelFunc),
	}

	fmt.Fprintf(stdout, "Starting Log Multiplexer (tailing active jobs from %s)...\n", host)
	fmt.Fprintf(stdout, "Press Ctrl+C to stop.\n\n")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Initial poll
	m.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(stdout, "\nShutting down multiplexer, waiting for log streams to finish...\n")
			m.mu.Lock()
			for _, cancel := range m.active {
				cancel()
			}
			m.mu.Unlock()
			m.wg.Wait()
			return nil
		case <-ticker.C:
			m.poll(ctx)
		}
	}
}

func (m *multiplexer) poll(ctx context.Context) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", m.host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		return
	}

	q := u.Query()
	q.Set("state", "active") // optimization, since we skip completed/failed anyway
	if m.tag != "" {
		q.Set("tag", m.tag)
	}
	if m.match != "" {
		q.Set("match", m.match)
	}
	if m.group != "" {
		q.Set("group", m.group)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Just log and retry later
		fmt.Fprintf(stdout, "[Multiplexer] Error fetching jobs: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "[Multiplexer] Error fetching jobs: status %d\n", resp.StatusCode)
		return
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "[Multiplexer] Error decoding jobs: %v\n", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, job := range jobs {
		// We only want to tail jobs that are actually running (Spawning or Active)
		if job.Status == "Completed" || job.Status == "Failed" || job.Status == "Canceled" || job.Status == "Pending" {
			continue
		}

		if _, exists := m.active[job.ID]; !exists {
			jobCtx, cancel := context.WithCancel(ctx)
			m.active[job.ID] = cancel

			// Assign a color
			colorHex := colors[m.colorCounter%len(colors)]
			m.colorCounter++

			style := lipgloss.NewStyle().Foreground(lipgloss.Color(colorHex)).Bold(true)
			prefix := style.Render(fmt.Sprintf("[%s]", job.ID))

			m.wg.Add(1)
			go m.tailJob(jobCtx, job.ID, prefix)
		}
	}
}

func (m *multiplexer) tailJob(ctx context.Context, jobID, prefix string) {
	defer m.wg.Done()
	defer func() {
		m.mu.Lock()
		delete(m.active, jobID)
		m.mu.Unlock()
		fmt.Fprintf(stdout, "%s --- Log Stream Finished ---\n", prefix)
	}()

	fmt.Fprintf(stdout, "%s --- Started Tailing Logs ---\n", prefix)

	url := fmt.Sprintf("%s/jobs/%s/logs", m.host, jobID)

	// Since the logs endpoint streams, we must retry if it fails initially (container not started yet)
	retryDelay := 1 * time.Second
	maxRetries := 10
	retries := 0

	for {
		if ctx.Err() != nil {
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			fmt.Fprintf(stdout, "%s Failed to create log request: %v\n", prefix, err)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, exit normally
			}
			retries++
			if retries > maxRetries {
				fmt.Fprintf(stdout, "%s Failed to connect to log stream after %d retries: %v\n", prefix, maxRetries, err)
				return
			}
			time.Sleep(retryDelay)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			retries++
			if retries > maxRetries {
				fmt.Fprintf(stdout, "%s Failed to fetch logs (status %d) after %d retries\n", prefix, resp.StatusCode, maxRetries)
				return
			}
			time.Sleep(retryDelay)
			continue
		}

		// Success, stream lines
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if ctx.Err() != nil {
				resp.Body.Close()
				return
			}
			fmt.Fprintf(stdout, "%s %s\n", prefix, scanner.Text())
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			fmt.Fprintf(stdout, "%s Stream error: %v\n", prefix, err)
		}

		resp.Body.Close()
		return // Stream finished naturally (EOF)
	}
}
