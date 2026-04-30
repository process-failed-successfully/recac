package orchestrator

import (
	archive_tar "archive/tar"
	"bufio"
	"bytes"
	"path/filepath"
	"os"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	encoding_csv "encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/jira"

	"github.com/spf13/viper"
)

var newAgentFunc = agent.NewAgent

func GetNewAgentFunc() func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
	return newAgentFunc
}

func SetNewAgentFunc(f func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error)) {
	newAgentFunc = f
}

// RegisterAPI registers the orchestrator API handlers on the provided ServeMux.
func RegisterAPI(mux *http.ServeMux, orch *Orchestrator, logger *slog.Logger, baseCtx context.Context) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(DashboardHTML))
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := orch.Ping(r.Context()); err != nil {
			http.Error(w, fmt.Sprintf("Health check failed: %v", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		ch := orch.Subscribe()
		defer orch.Unsubscribe(ch)

		// Send an initial connected event
		fmt.Fprintf(w, "data: {\"event\": \"connected\"}\n\n")
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case eventData := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", string(eventData))
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		status := orch.GetStatus()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			logger.Error("Failed to encode status", "error", err)
		}
	})

	mux.HandleFunc("GET /groups", func(w http.ResponseWriter, r *http.Request) {
		groups := orch.GetGroups()

		if groups == nil {
			groups = []GroupInfo{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(groups); err != nil {
			logger.Error("Failed to encode groups", "error", err)
		}
	})

	mux.HandleFunc("GET /tags", func(w http.ResponseWriter, r *http.Request) {
		tagCounts := make(map[string]int)

		// GetActiveJobs() already includes Pending jobs, so we only need GetActiveJobs and GetCompletedJobs
		// Count tags sequentially to avoid large intermediate slice allocation
		for _, job := range orch.GetActiveJobs() {
			for _, tag := range job.WorkItem.Tags {
				tagCounts[tag]++
			}
		}
		for _, job := range orch.GetCompletedJobs() {
			for _, tag := range job.WorkItem.Tags {
				tagCounts[tag]++
			}
		}

		// Prepare output
		type TagInfo struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		var result []TagInfo
		for name, count := range tagCounts {
			result = append(result, TagInfo{Name: name, Count: count})
		}

		// Sort result by count descending, then name alphabetically
		sort.Slice(result, func(i, j int) bool {
			if result[i].Count == result[j].Count {
				return result[i].Name < result[j].Name
			}
			return result[i].Count > result[j].Count
		})

		// if result is nil, json.Encode encodes it to `null`. We want an empty array instead `[]`
		if result == nil {
			result = []TagInfo{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			logger.Error("Failed to encode tags", "error", err)
		}
	})

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		status := orch.GetStatus()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		fmt.Fprintf(w, "# HELP recac_active_jobs Number of active jobs\n")
		fmt.Fprintf(w, "# TYPE recac_active_jobs gauge\n")

		activeCount := 0
		for _, job := range orch.GetActiveJobs() {
			if job.Status == "Running" || job.Status == "Started" {
				activeCount++
			}
		}
		if activeCount == 0 && (len(orch.GetActiveJobs())-status.PendingJobs) > 0 {
			activeCount = len(orch.GetActiveJobs()) - status.PendingJobs
		}

		fmt.Fprintf(w, "recac_active_jobs %d\n", activeCount)

		fmt.Fprintf(w, "# HELP recac_pending_jobs Number of pending jobs\n")
		fmt.Fprintf(w, "# TYPE recac_pending_jobs gauge\n")
		fmt.Fprintf(w, "recac_pending_jobs %d\n", status.PendingJobs)

		fmt.Fprintf(w, "# HELP recac_total_spawns Total number of job spawns\n")
		fmt.Fprintf(w, "# TYPE recac_total_spawns counter\n")
		fmt.Fprintf(w, "recac_total_spawns %d\n", status.TotalSpawns)

		fmt.Fprintf(w, "# HELP recac_active_spawns Number of active spawns\n")
		fmt.Fprintf(w, "# TYPE recac_active_spawns gauge\n")
		fmt.Fprintf(w, "recac_active_spawns %d\n", status.ActiveSpawns)

		paused := 0
		if status.Paused {
			paused = 1
		}
		fmt.Fprintf(w, "# HELP recac_paused Whether the orchestrator is paused (1) or not (0)\n")
		fmt.Fprintf(w, "# TYPE recac_paused gauge\n")
		fmt.Fprintf(w, "recac_paused %d\n", paused)

		draining := 0
		if status.Draining {
			draining = 1
		}
		fmt.Fprintf(w, "# HELP recac_draining Whether the orchestrator is draining (1) or not (0)\n")
		fmt.Fprintf(w, "# TYPE recac_draining gauge\n")
		fmt.Fprintf(w, "recac_draining %d\n", draining)
	})

	mux.HandleFunc("GET /analytics", func(w http.ResponseWriter, r *http.Request) {
		analytics := orch.GetAnalytics()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(analytics); err != nil {
			logger.Error("Failed to encode analytics", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		statusFilter := r.URL.Query().Get("status")
		tagFilter := r.URL.Query().Get("tag")
		var jobs []JobInfo

		switch state {
		case "completed":
			jobs = orch.GetCompletedJobs()
		case "pending":
			jobs = orch.GetPendingJobs()
		case "all":
			jobs = append(orch.GetActiveJobs(), orch.GetCompletedJobs()...)
		default:
			jobs = orch.GetActiveJobs()
		}

		if statusFilter != "" {
			var filtered []JobInfo
			for _, job := range jobs {
				if strings.EqualFold(job.Status, statusFilter) {
					filtered = append(filtered, job)
				}
			}
			jobs = filtered
		}

		if tagFilter != "" {
			var filtered []JobInfo
			for _, job := range jobs {
				hasTag := false
				for _, tag := range job.WorkItem.Tags {
					if strings.EqualFold(tag, tagFilter) {
						hasTag = true
						break
					}
				}
				if hasTag {
					filtered = append(filtered, job)
				}
			}
			jobs = filtered
		}

		priorityFilter := r.URL.Query().Get("priority")
		if priorityFilter != "" {
			var filtered []JobInfo
			var priority int
			if p, err := strconv.Atoi(priorityFilter); err == nil {
				priority = p
				for _, job := range jobs {
					if job.WorkItem.Priority == priority {
						filtered = append(filtered, job)
					}
				}
				jobs = filtered
			} else {
				http.Error(w, fmt.Sprintf("invalid priority filter: %v", err), http.StatusBadRequest)
				return
			}
		}

		matchFilter := r.URL.Query().Get("match")
		if matchFilter != "" {
			matcher, err := regexp.Compile("(?i)" + matchFilter)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid match regex: %v", err), http.StatusBadRequest)
				return
			}
			var filtered []JobInfo
			for _, job := range jobs {
				if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
					filtered = append(filtered, job)
				}
			}
			jobs = filtered
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			logger.Error("Failed to encode jobs", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/summary", func(w http.ResponseWriter, r *http.Request) {
		summary := make(map[string]int)
		for _, job := range orch.GetActiveJobs() {
			summary[job.Status]++
		}
		for _, job := range orch.GetCompletedJobs() {
			summary[job.Status]++
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(summary); err != nil {
			logger.Error("Failed to encode summary", "error", err)
		}
	})

	mux.HandleFunc("GET /diagnose", handleDiagnose(orch, logger))
	mux.HandleFunc("GET /simulate", handleSimulate(orch, logger))
	mux.HandleFunc("POST /simulate/pipeline", handleSimulatePipeline(orch, logger))

	mux.HandleFunc("GET /jobs/analyze/costs", handleAnalyzeCosts(orch, logger))
	mux.HandleFunc("GET /jobs/analyze/anomalies", handleAnalyzeAnomalies(orch, logger))
	mux.HandleFunc("GET /jobs/analyze/agents", handleAnalyzeAgents(orch, logger))
mux.HandleFunc("GET /jobs/analyze/tags", handleAnalyzeTags(orch, logger))

	mux.HandleFunc("GET /jobs/analyze/durations", func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		limit := 10 // Default limit
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l >= 0 {
				limit = l
			}
		}

		jobs := orch.GetCompletedJobs()
		var validJobs []JobInfo
		for _, job := range jobs {
			if !job.StartTime.IsZero() && !job.EndTime.IsZero() {
				dur := job.EndTime.Sub(job.StartTime)
				if dur > 0 {
					validJobs = append(validJobs, job)
				}
			}
		}

		type TagStat struct {
			Tag          string  `json:"tag"`
			Count        int     `json:"count"`
			MeanDuration float64 `json:"mean_duration_ms"` // in milliseconds for easier frontend formatting
		}

		type DurationStats struct {
			TotalJobs      int       `json:"total_jobs"`
			TotalDuration  float64   `json:"total_duration_ms"`
			MeanDuration   float64   `json:"mean_duration_ms"`
			MedianDuration float64   `json:"median_duration_ms"`
			MinDuration    float64   `json:"min_duration_ms"`
			MaxDuration    float64   `json:"max_duration_ms"`
			TagStats       []TagStat `json:"tag_stats"`
			TopSlowest     []JobInfo `json:"top_slowest"`
		}

		if len(validJobs) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DurationStats{TagStats: []TagStat{}, TopSlowest: []JobInfo{}})
			return
		}

		var totalDuration time.Duration
		var minDuration time.Duration = -1
		var maxDuration time.Duration
		var durations []time.Duration
		tagDurations := make(map[string][]time.Duration)

		for _, job := range validJobs {
			dur := job.EndTime.Sub(job.StartTime)
			totalDuration += dur
			durations = append(durations, dur)

			if minDuration == -1 || dur < minDuration {
				minDuration = dur
			}
			if dur > maxDuration {
				maxDuration = dur
			}

			for _, tag := range job.WorkItem.Tags {
				tagDurations[tag] = append(tagDurations[tag], dur)
			}
		}

		meanDuration := totalDuration / time.Duration(len(validJobs))

		sort.Slice(durations, func(i, j int) bool {
			return durations[i] < durations[j]
		})

		var medianDuration time.Duration
		mid := len(durations) / 2
		if len(durations)%2 == 0 {
			medianDuration = (durations[mid-1] + durations[mid]) / 2
		} else {
			medianDuration = durations[mid]
		}

		sort.Slice(validJobs, func(i, j int) bool {
			return validJobs[i].EndTime.Sub(validJobs[i].StartTime) > validJobs[j].EndTime.Sub(validJobs[j].StartTime)
		})

		topSlowest := validJobs
		if len(topSlowest) > limit {
			topSlowest = topSlowest[:limit]
		}

		var tagStats []TagStat
		for tag, tagDurs := range tagDurations {
			var tagTotal time.Duration
			for _, d := range tagDurs {
				tagTotal += d
			}
			tagStats = append(tagStats, TagStat{
				Tag:          tag,
				Count:        len(tagDurs),
				MeanDuration: float64(tagTotal.Milliseconds()) / float64(len(tagDurs)),
			})
		}

		sort.Slice(tagStats, func(i, j int) bool {
			return tagStats[i].MeanDuration > tagStats[j].MeanDuration
		})

		stats := DurationStats{
			TotalJobs:      len(validJobs),
			TotalDuration:  float64(totalDuration.Milliseconds()),
			MeanDuration:   float64(meanDuration.Milliseconds()),
			MedianDuration: float64(medianDuration.Milliseconds()),
			MinDuration:    float64(minDuration.Milliseconds()),
			MaxDuration:    float64(maxDuration.Milliseconds()),
			TagStats:       tagStats,
			TopSlowest:     topSlowest,
		}

		if stats.TagStats == nil {
			stats.TagStats = []TagStat{}
		}
		if stats.TopSlowest == nil {
			stats.TopSlowest = []JobInfo{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			logger.Error("Failed to encode duration stats", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/analyze/reliability", func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		limit := 10
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l >= 0 {
				limit = l
			}
		}

		jobs := orch.GetCompletedJobs()

		var stats ReliabilityStats
		flakyMap := make(map[string]*FlakyJobStat)
		failedMap := make(map[string]*FailedJobStat)

		for _, job := range jobs {
			if job.Status == "Canceled" || job.Status == "Skipped" {
				continue
			}

			stats.TotalJobs++

			if job.Status == "Completed" {
				if job.RetryCount > 0 {
					stats.FlakyJobs++
					stats.TotalRetries += job.RetryCount

					fStat, exists := flakyMap[job.Summary]
					if !exists {
						fStat = &FlakyJobStat{Summary: job.Summary}
						flakyMap[job.Summary] = fStat
					}
					fStat.Occurrences++
					fStat.TotalRetries += job.RetryCount
				} else {
					stats.SuccessfulJobs++
				}
			} else if job.Status == "Failed" || job.Status == "error" {
				stats.FailedJobs++
				stats.TotalRetries += job.RetryCount // failed jobs might also have retries

				fStat, exists := failedMap[job.Summary]
				if !exists {
					fStat = &FailedJobStat{Summary: job.Summary}
					failedMap[job.Summary] = fStat
				}
				fStat.Occurrences++
			}
		}

		if stats.TotalJobs > 0 {
			stats.SuccessRate = float64(stats.SuccessfulJobs+stats.FlakyJobs) / float64(stats.TotalJobs) * 100.0
			stats.FlakinessRate = float64(stats.FlakyJobs) / float64(stats.TotalJobs) * 100.0
			stats.FailureRate = float64(stats.FailedJobs) / float64(stats.TotalJobs) * 100.0
		}

		for _, stat := range flakyMap {
			stat.AvgRetries = float64(stat.TotalRetries) / float64(stat.Occurrences)
			stats.TopFlakyJobs = append(stats.TopFlakyJobs, *stat)
		}
		for _, stat := range failedMap {
			stats.TopFailingJobs = append(stats.TopFailingJobs, *stat)
		}

		// Sort top flaky
		sort.Slice(stats.TopFlakyJobs, func(i, j int) bool {
			if stats.TopFlakyJobs[i].Occurrences == stats.TopFlakyJobs[j].Occurrences {
				return stats.TopFlakyJobs[i].AvgRetries > stats.TopFlakyJobs[j].AvgRetries
			}
			return stats.TopFlakyJobs[i].Occurrences > stats.TopFlakyJobs[j].Occurrences
		})
		if len(stats.TopFlakyJobs) > limit {
			stats.TopFlakyJobs = stats.TopFlakyJobs[:limit]
		}
		if stats.TopFlakyJobs == nil {
			stats.TopFlakyJobs = []FlakyJobStat{}
		}

		// Sort top failing
		sort.Slice(stats.TopFailingJobs, func(i, j int) bool {
			return stats.TopFailingJobs[i].Occurrences > stats.TopFailingJobs[j].Occurrences
		})
		if len(stats.TopFailingJobs) > limit {
			stats.TopFailingJobs = stats.TopFailingJobs[:limit]
		}
		if stats.TopFailingJobs == nil {
			stats.TopFailingJobs = []FailedJobStat{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			logger.Error("Failed to encode reliability stats", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/search", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
			return
		}

		matcher, err := regexp.Compile("(?i)" + query)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid regex query: %v", err), http.StatusBadRequest)
			return
		}

		tagFilter := r.URL.Query().Get("tag")
		statusFilter := r.URL.Query().Get("status")

		jobs := append(orch.GetPendingJobs(), orch.GetActiveJobs()...)
		jobs = append(jobs, orch.GetCompletedJobs()...)
		var filtered []JobInfo

		for _, job := range jobs {
			if statusFilter != "" && !strings.EqualFold(job.Status, statusFilter) {
				continue
			}

			if tagFilter != "" {
				hasTag := false
				for _, t := range job.WorkItem.Tags {
					if strings.EqualFold(t, tagFilter) {
						hasTag = true
						break
					}
				}
				if !hasTag {
					continue
				}
			}

			// Match against Summary, Description, and Error
			if matcher.MatchString(job.Summary) || matcher.MatchString(job.WorkItem.Description) || matcher.MatchString(job.Error) {
				filtered = append(filtered, job)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(filtered); err != nil {
			logger.Error("Failed to encode search results", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/search/logs", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
			return
		}

		matcher, err := regexp.Compile("(?i)" + query)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid regex query: %v", err), http.StatusBadRequest)
			return
		}

		contextLines := 0
		if ctxStr := r.URL.Query().Get("context"); ctxStr != "" {
			if parsed, err := strconv.Atoi(ctxStr); err == nil && parsed > 0 {
				contextLines = parsed
			}
		}

		tagFilter := r.URL.Query().Get("tag")
		statusFilter := r.URL.Query().Get("status")

		jobs := append(orch.GetActiveJobs(), orch.GetCompletedJobs()...)
		var filtered []JobInfo

		// ⚡ Bolt: Use strings.EqualFold for zero-allocation case-insensitive comparisons
		for _, job := range jobs {
			if statusFilter != "" && !strings.EqualFold(job.Status, statusFilter) {
				continue
			}

			if tagFilter != "" {
				hasTag := false
				for _, t := range job.WorkItem.Tags {
					if strings.EqualFold(t, tagFilter) {
						hasTag = true
						break
					}
				}
				if !hasTag {
					continue
				}
			}
			filtered = append(filtered, job)
		}

		type ContextLine struct {
			LineNumber int    `json:"line_number"`
			Text       string `json:"text"`
		}

		type LogMatch struct {
			LineNumber    int           `json:"line_number"`
			Text          string        `json:"text"`
			ContextBefore []ContextLine `json:"context_before,omitempty"`
			ContextAfter  []ContextLine `json:"context_after,omitempty"`
		}

		type JobLogResult struct {
			JobID   string     `json:"job_id"`
			Summary string     `json:"summary"`
			Status  string     `json:"status"`
			Matches []LogMatch `json:"matches"`
		}

		var results []JobLogResult

		for _, job := range filtered {
			logStream, err := orch.GetLogs(r.Context(), job.ID)
			if err != nil {
				continue // Skip if logs are not available
			}

			scanner := bufio.NewScanner(logStream)
			lineNum := 1
			var matches []LogMatch

			var circularBuffer []ContextLine
			var afterCountdown int
			var currentMatch *LogMatch

			for scanner.Scan() {
				line := scanner.Text()

				isMatch := matcher.MatchString(line)

				// If we are currently collecting ContextAfter lines for a previous match
				if afterCountdown > 0 {
					currentMatch.ContextAfter = append(currentMatch.ContextAfter, ContextLine{
						LineNumber: lineNum,
						Text:       line,
					})
					afterCountdown--

					// If a new match occurs while we are collecting "after" context,
					// we just finalize the current match and start a new one
					if isMatch {
						// Don't count the new match as an after line, let it be the main match
						currentMatch.ContextAfter = currentMatch.ContextAfter[:len(currentMatch.ContextAfter)-1]
						afterCountdown = 0
					} else if afterCountdown == 0 {
						matches = append(matches, *currentMatch)
						currentMatch = nil
						if len(matches) >= 10 { // Limit matches per job to avoid huge responses
							break
						}
					}
				}

				if isMatch {
					if currentMatch != nil {
						// We hit a new match before finishing the "after" context of the previous one
						matches = append(matches, *currentMatch)
						currentMatch = nil
						if len(matches) >= 10 {
							break
						}
					}

					// Copy the current circular buffer as ContextBefore
					ctxBefore := make([]ContextLine, len(circularBuffer))
					copy(ctxBefore, circularBuffer)

					currentMatch = &LogMatch{
						LineNumber:    lineNum,
						Text:          line,
						ContextBefore: ctxBefore,
					}

					if contextLines > 0 {
						afterCountdown = contextLines
					} else {
						matches = append(matches, *currentMatch)
						currentMatch = nil
						if len(matches) >= 10 {
							break
						}
					}
				}

				// Always maintain the circular buffer of past lines
				if contextLines > 0 {
					circularBuffer = append(circularBuffer, ContextLine{
						LineNumber: lineNum,
						Text:       line,
					})
					if len(circularBuffer) > contextLines {
						circularBuffer = circularBuffer[1:]
					}
				}

				lineNum++
			}

			// If EOF and we still have a pending match
			if currentMatch != nil {
				matches = append(matches, *currentMatch)
			}

			logStream.Close()

			if len(matches) > 0 {
				results = append(results, JobLogResult{
					JobID:   job.ID,
					Summary: job.Summary,
					Status:  job.Status,
					Matches: matches,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(results); err != nil {
			logger.Error("Failed to encode log search results", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/export/timeline", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")

		var jobs []JobInfo
		switch state {
		case "completed":
			jobs = orch.GetCompletedJobs()
		case "active":
			jobs = orch.GetActiveJobs()
		case "pending":
			jobs = orch.GetPendingJobs()
		default: // default active+pending+completed for timeline
			jobs = append(orch.GetActiveJobs(), orch.GetPendingJobs()...)
			jobs = append(jobs, orch.GetCompletedJobs()...)
		}

		timelineStr := ExportTimelineToMermaid(jobs)

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(timelineStr))
	})

	mux.HandleFunc("GET /jobs/export/graph", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "mermaid"
		}

		var jobs []JobInfo
		switch state {
		case "completed":
			jobs = orch.GetCompletedJobs()
		case "active":
			jobs = orch.GetActiveJobs()
		case "pending":
			jobs = orch.GetPendingJobs()
		default: // default active+pending+completed for graph
			jobs = append(orch.GetActiveJobs(), orch.GetPendingJobs()...)
			jobs = append(jobs, orch.GetCompletedJobs()...)
		}

		var graphStr string
		// ⚡ Bolt: Replace switch strings.ToLower with allocation-free strings.EqualFold
		if strings.EqualFold(format, "dot") {
			w.Header().Set("Content-Type", "text/vnd.graphviz")
			graphStr = ExportGraphToDOT(jobs)
		} else if strings.EqualFold(format, "mermaid") {
			w.Header().Set("Content-Type", "text/plain")
			graphStr = ExportGraphToMermaid(jobs)
		} else if strings.EqualFold(format, "plantuml") {
			w.Header().Set("Content-Type", "text/plain")
			graphStr = ExportGraphToPlantUML(jobs)
		} else {
			http.Error(w, "Invalid format. Supported formats: mermaid, dot, plantuml", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(graphStr))
	})

	mux.HandleFunc("GET /jobs/export/pipeline", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			name = "exported-pipeline"
		}
		state := r.URL.Query().Get("state")

		var jobs []JobInfo
		switch state {
		case "completed":
			jobs = orch.GetCompletedJobs()
		case "all":
			jobs = append(orch.GetActiveJobs(), orch.GetPendingJobs()...)
			jobs = append(jobs, orch.GetCompletedJobs()...)
		default: // default active+pending
			jobs = append(orch.GetActiveJobs(), orch.GetPendingJobs()...)
		}

		yamlData, err := ExportPipelineToYAML(name, jobs)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to export pipeline: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-yaml")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.yaml", name))
		w.Write(yamlData)
	})

	mux.HandleFunc("GET /jobs/export/trace", func(w http.ResponseWriter, r *http.Request) {
		stateFilter := r.URL.Query().Get("state")
		if stateFilter == "" {
			stateFilter = "all"
		}

		var jobs []JobInfo
		if stateFilter == "active" || stateFilter == "all" {
			jobs = append(jobs, orch.GetActiveJobs()...)
		}
		if stateFilter == "completed" || stateFilter == "failed" || stateFilter == "all" {
			completedJobs := orch.GetCompletedJobs()
			for _, job := range completedJobs {
				if stateFilter == "failed" && !strings.EqualFold(job.Status, "failed") {
					continue
				}
				if stateFilter == "completed" && !strings.EqualFold(job.Status, "completed") {
					continue
				}
				jobs = append(jobs, job)
			}
		}

		jsonData, err := ExportTraceToJSON(jobs)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to export trace: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=trace.json")
		w.Write(jsonData)
	})

	mux.HandleFunc("GET /jobs/export/metrics", func(w http.ResponseWriter, r *http.Request) {
		stateFilter := r.URL.Query().Get("state")
		if stateFilter == "" {
			stateFilter = "all"
		}

		var jobs []JobInfo
		if stateFilter == "active" || stateFilter == "all" {
			jobs = append(jobs, orch.GetActiveJobs()...)
		}
		if stateFilter == "completed" || stateFilter == "failed" || stateFilter == "all" {
			completedJobs := orch.GetCompletedJobs()
			for _, job := range completedJobs {
				if stateFilter == "failed" && !strings.EqualFold(job.Status, "failed") {
					continue
				}
				if stateFilter == "completed" && !strings.EqualFold(job.Status, "completed") {
					continue
				}
				jobs = append(jobs, job)
			}
		}

		// Collect all unique metric keys
		metricKeysMap := make(map[string]bool)
		for _, job := range jobs {
			for k := range job.Metrics {
				metricKeysMap[k] = true
			}
		}

		var metricKeys []string
		for k := range metricKeysMap {
			metricKeys = append(metricKeys, k)
		}
		sort.Strings(metricKeys) // Sort to ensure deterministic column order

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=metrics_export.csv")

		writer := encoding_csv.NewWriter(w)
		defer writer.Flush()

		// Build header
		header := []string{"JobID", "Status", "StartTime", "Duration"}
		header = append(header, metricKeys...)
		if err := writer.Write(header); err != nil {
			logger.Error("Failed to write CSV header", "error", err)
			http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
			return
		}

		for _, job := range jobs {
			startTimeStr := ""
			durationStr := ""
			if !job.StartTime.IsZero() {
				startTimeStr = job.StartTime.Format(time.RFC3339)
				endTime := job.EndTime
				if endTime.IsZero() {
					endTime = time.Now()
				}
				durationStr = endTime.Sub(job.StartTime).Round(time.Second).String()
			}

			row := []string{
				job.ID,
				job.Status,
				startTimeStr,
				durationStr,
			}

			for _, k := range metricKeys {
				valStr := ""
				if val, exists := job.Metrics[k]; exists {
					// Format to 2 decimal places to be neat, or just generic float formatting
					valStr = fmt.Sprintf("%g", val)
				}
				row = append(row, valStr)
			}

			if err := writer.Write(row); err != nil {
				logger.Error("Failed to write CSV row", "error", err)
				http.Error(w, "Failed to write CSV row", http.StatusInternalServerError)
				return
			}
		}
	})

	mux.HandleFunc("GET /jobs/export", func(w http.ResponseWriter, r *http.Request) {
		format := r.URL.Query().Get("format")
		jobs := append(orch.GetActiveJobs(), orch.GetCompletedJobs()...)

		if format == "junit" {
			xmlStr, err := ExportJobsToJUnitXML(jobs)
			if err != nil {
				logger.Error("Failed to generate JUnit XML report", "error", err)
				http.Error(w, "Failed to generate JUnit XML report", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			w.Header().Set("Content-Disposition", "attachment; filename=jobs_export.xml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(xmlStr))
			return
		}

		if format == "csv" {
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", "attachment; filename=jobs_export.csv")

			writer := encoding_csv.NewWriter(w)
			defer writer.Flush()

			// Header
			writer.Write([]string{"ID", "Summary", "Status", "StartTime", "EndTime", "RepoURL"})

			for _, job := range jobs {
				startTime := ""
				if !job.StartTime.IsZero() {
					startTime = job.StartTime.Format(time.RFC3339)
				}

				endTime := ""
				if !job.EndTime.IsZero() {
					endTime = job.EndTime.Format(time.RFC3339)
				}

				writer.Write([]string{
					job.ID,
					job.Summary,
					job.Status,
					startTime,
					endTime,
					job.WorkItem.RepoURL,
				})
			}
		} else {
			// Default to JSON
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", "attachment; filename=jobs_export.json")
			if err := json.NewEncoder(w).Encode(jobs); err != nil {
				logger.Error("Failed to encode jobs for export", "error", err)
			}
		}
	})

	mux.HandleFunc("GET /changelog/generate", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")
		provider := r.URL.Query().Get("provider")
		model := r.URL.Query().Get("model")

		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			apiKey = viper.GetString("secrets.api_key")
		}
		if provider == "" {
			provider = viper.GetString("orchestrator.agent_provider")
		}
		if model == "" {
			model = viper.GetString("orchestrator.agent_model")
		}

		changelogText, err := GenerateChangelog(r.Context(), orch, tag, match, provider, model, apiKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate changelog: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"changelog": changelogText}); err != nil {
			logger.Error("Failed to encode generated changelog response", "error", err)
		}
	})

	mux.HandleFunc("GET /postmortem/generate", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")
		provider := r.URL.Query().Get("provider")
		model := r.URL.Query().Get("model")

		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			apiKey = viper.GetString("secrets.api_key")
		}
		if provider == "" {
			provider = viper.GetString("orchestrator.agent_provider")
		}
		if model == "" {
			model = viper.GetString("orchestrator.agent_model")
		}

		postmortemText, err := GeneratePostmortem(r.Context(), orch, tag, match, provider, model, apiKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate postmortem: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"postmortem": postmortemText}); err != nil {
			logger.Error("Failed to encode generated postmortem response", "error", err)
		}
	})

	mux.HandleFunc("POST /pipeline/generate", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if req.Prompt == "" {
			http.Error(w, "Prompt is required", http.StatusBadRequest)
			return
		}

		provider := r.URL.Query().Get("provider")
		model := r.URL.Query().Get("model")

		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			apiKey = viper.GetString("secrets.api_key")
		}
		if provider == "" {
			provider = viper.GetString("orchestrator.agent_provider")
		}
		if model == "" {
			model = viper.GetString("orchestrator.agent_model")
		}

		pipelineYAML, err := GeneratePipelineYAML(r.Context(), req.Prompt, provider, model, apiKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate pipeline: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"pipeline_yaml": pipelineYAML}); err != nil {
			logger.Error("Failed to encode generated pipeline response", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/{id}/explain", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		job, err := orch.GetJob(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		provider := r.URL.Query().Get("provider")
		model := r.URL.Query().Get("model")

		logStream, err := orch.GetLogs(r.Context(), id)
		var logsText string
		if err == nil {
			logBytes, _ := io.ReadAll(logStream)
			logsText = string(logBytes)
			logStream.Close()
		}

		logLines := strings.Split(logsText, "\n")
		if len(logLines) > 1000 {
			logLines = logLines[len(logLines)-1000:]
			logsText = "... [Logs Truncated] ...\n" + strings.Join(logLines, "\n")
		}

		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			apiKey = viper.GetString("secrets.api_key")
		}
		if provider == "" {
			provider = viper.GetString("orchestrator.agent_provider")
		}
		if model == "" {
			model = viper.GetString("orchestrator.agent_model")
		}

		aiClient, err := newAgentFunc(provider, apiKey, model, "", "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to initialize AI agent: %v", err), http.StatusInternalServerError)
			return
		}

		prompt := fmt.Sprintf(`You are an expert software engineer and debugger analyzing a failed or problematic job in an autonomous coding orchestrator.

Job ID: %s
Summary: %s
Status: %s
Error: %s

Here are the last log lines from the job execution:
%s

Analyze why the job failed or had issues, explain the root cause clearly, and suggest concrete steps to fix it.`,
			job.ID, job.Summary, job.Status, job.Error, logsText)

		explanation, err := aiClient.Send(r.Context(), prompt)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get explanation from AI: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"explanation": explanation}); err != nil {
			logger.Error("Failed to encode explanation", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/explain/bulk", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk explanation", http.StatusBadRequest)
			return
		}

		provider := r.URL.Query().Get("provider")
		model := r.URL.Query().Get("model")

		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			apiKey = viper.GetString("secrets.api_key")
		}
		if provider == "" {
			provider = viper.GetString("orchestrator.agent_provider")
		}
		if model == "" {
			model = viper.GetString("orchestrator.agent_model")
		}

		aiClient, err := newAgentFunc(provider, apiKey, model, "", "")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to initialize AI agent: %v", err), http.StatusInternalServerError)
			return
		}

		jobs := append(orch.GetActiveJobs(), orch.GetCompletedJobs()...)
		var filtered []JobInfo

		if tag != "" {
			for _, job := range jobs {
				if strings.EqualFold(job.Status, "Failed") || strings.EqualFold(job.Status, "error") {
					hasTag := false
					for _, t := range job.WorkItem.Tags {
						if strings.EqualFold(t, tag) {
							hasTag = true
							break
						}
					}
					if hasTag {
						filtered = append(filtered, job)
					}
				}
			}
		} else if match != "" {
			matcher, err := regexp.Compile("(?i)" + match)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid match regex: %v", err), http.StatusBadRequest)
				return
			}
			for _, job := range jobs {
				if strings.EqualFold(job.Status, "Failed") || strings.EqualFold(job.Status, "error") {
					if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
						filtered = append(filtered, job)
					}
				}
			}
		}

		if len(filtered) == 0 {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]map[string]string{"explanations": {}}); err != nil {
				logger.Error("Failed to encode empty explanations", "error", err)
			}
			return
		}

		type result struct {
			id  string
			exp string
		}

		ch := make(chan result, len(filtered))
		for _, job := range filtered {
			go func(j JobInfo) {
				logStream, err := orch.GetLogs(r.Context(), j.ID)
				var logsText string
				if err == nil {
					logBytes, _ := io.ReadAll(logStream)
					logsText = string(logBytes)
					logStream.Close()
				}

				logLines := strings.Split(logsText, "\n")
				if len(logLines) > 1000 {
					logLines = logLines[len(logLines)-1000:]
					logsText = "... [Logs Truncated] ...\n" + strings.Join(logLines, "\n")
				}

				prompt := fmt.Sprintf(`You are an expert software engineer and debugger analyzing a failed or problematic job in an autonomous coding orchestrator.

Job ID: %s
Summary: %s
Status: %s
Error: %s

Here are the last log lines from the job execution:
%s

Analyze why the job failed or had issues, explain the root cause clearly, and suggest concrete steps to fix it.`,
					j.ID, j.Summary, j.Status, j.Error, logsText)

				explanation, err := aiClient.Send(context.Background(), prompt)
				if err != nil {
					explanation = fmt.Sprintf("Failed to get explanation from AI: %v", err)
				}
				ch <- result{id: j.ID, exp: explanation}
			}(job)
		}

		explanations := make(map[string]string)
		for i := 0; i < len(filtered); i++ {
			res := <-ch
			explanations[res.id] = res.exp
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]map[string]string{"explanations": explanations}); err != nil {
			logger.Error("Failed to encode explanations", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		job, err := orch.GetJob(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(job); err != nil {
			logger.Error("Failed to encode job", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/{id}/blockers", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		blockers, err := orch.GetJobBlockers(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// if blockers is nil, json.Encode encodes it to `null`. We want an empty array instead `[]`
		if blockers == nil {
			blockers = []JobInfo{}
		}
		if err := json.NewEncoder(w).Encode(blockers); err != nil {
			logger.Error("Failed to encode blockers", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/{id}/dependents", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		dependents, err := orch.GetJobDependents(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// if dependents is nil, json.Encode encodes it to `null`. We want an empty array instead `[]`
		if dependents == nil {
			dependents = []JobInfo{}
		}
		if err := json.NewEncoder(w).Encode(dependents); err != nil {
			logger.Error("Failed to encode dependents", "error", err)
		}
	})

	mux.HandleFunc("GET /jobs/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		logStream, err := orch.GetLogs(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		defer logStream.Close()
		io.Copy(w, logStream)
	})

	mux.HandleFunc("GET /jobs/{id}/archive", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		job, err := orch.GetJob(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.tar.gz\"", id))

		gzWriter := gzip.NewWriter(w)
		defer gzWriter.Close()

		tarWriter := archive_tar.NewWriter(gzWriter)
		defer tarWriter.Close()

		// 1. Write job.json
		jobData, err := json.MarshalIndent(job, "", "  ")
		if err == nil {
			hdr := &archive_tar.Header{
				Name: "job.json",
				Mode: 0600,
				Size: int64(len(jobData)),
			}
			if err := tarWriter.WriteHeader(hdr); err == nil {
				tarWriter.Write(jobData)
			}
		}

		// 2. Write logs.txt
		logStream, err := orch.GetLogs(r.Context(), id)
		if err == nil {
			defer logStream.Close()

			// We need the size of the logs. We can buffer them in memory since they are typically small for an agent run,
			// but a safer approach is to stream them to a temporary buffer if we need to know the size.
			// Or we can just read all.
			logData, err := io.ReadAll(logStream)
			if err == nil {
				hdr := &archive_tar.Header{
					Name: "logs.txt",
					Mode: 0600,
					Size: int64(len(logData)),
				}
				if err := tarWriter.WriteHeader(hdr); err == nil {
					tarWriter.Write(logData)
				}
			}
		}

		// 3. Write artifacts
		if orch.ArtifactsDir != "" {
			jobDir := filepath.Join(orch.ArtifactsDir, id)
			entries, err := os.ReadDir(jobDir)
			if err == nil {
				for _, entry := range entries {
					if !entry.IsDir() {
						filePath := filepath.Join(jobDir, entry.Name())
						fileData, err := os.ReadFile(filePath)
						if err == nil {
							hdr := &archive_tar.Header{
								Name: "artifacts/" + entry.Name(),
								Mode: 0600,
								Size: int64(len(fileData)),
							}
							if err := tarWriter.WriteHeader(hdr); err == nil {
								tarWriter.Write(fileData)
							}
						}
					}
				}
			}
		}
	})

	mux.HandleFunc("GET /jobs/archive/bulk", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")
		status := r.URL.Query().Get("status")
		group := r.URL.Query().Get("group")

		if tag == "" && match == "" && status == "" && group == "" {
			http.Error(w, "Either 'tag', 'match', 'status', or 'group' query parameter is required for bulk archive", http.StatusBadRequest)
			return
		}

		jobs := append(orch.GetActiveJobs(), orch.GetCompletedJobs()...)
		var filtered []JobInfo

		if tag != "" {
			for _, job := range jobs {
				hasTag := false
				for _, t := range job.WorkItem.Tags {
					if strings.EqualFold(t, tag) {
						hasTag = true
						break
					}
				}
				if hasTag {
					filtered = append(filtered, job)
				}
			}
		} else if match != "" {
			matcher, err := regexp.Compile("(?i)" + match)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid match regex: %v", err), http.StatusBadRequest)
				return
			}
			for _, job := range jobs {
				if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
					filtered = append(filtered, job)
				}
			}
		} else if status != "" {
			for _, job := range jobs {
				if strings.EqualFold(job.Status, status) {
					filtered = append(filtered, job)
				}
			}
		} else if group != "" {
			for _, job := range jobs {
				if strings.EqualFold(job.WorkItem.ConcurrencyGroup, group) {
					filtered = append(filtered, job)
				}
			}
		}

		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", "attachment; filename=\"bulk_archive.tar.gz\"")

		gzWriter := gzip.NewWriter(w)
		defer gzWriter.Close()

		tarWriter := archive_tar.NewWriter(gzWriter)
		defer tarWriter.Close()

		for _, job := range filtered {
			jobData, err := json.MarshalIndent(job, "", "  ")
			if err == nil {
				hdr := &archive_tar.Header{Name: fmt.Sprintf("%s/job.json", job.ID), Mode: 0600, Size: int64(len(jobData))}
				if err := tarWriter.WriteHeader(hdr); err == nil {
					tarWriter.Write(jobData)
				}
			}
			logStream, err := orch.GetLogs(r.Context(), job.ID)
			if err == nil {
				logData, err := io.ReadAll(logStream)
				logStream.Close()
				if err == nil {
					hdr := &archive_tar.Header{Name: fmt.Sprintf("%s/logs.txt", job.ID), Mode: 0600, Size: int64(len(logData))}
					if err := tarWriter.WriteHeader(hdr); err == nil {
						tarWriter.Write(logData)
					}
				}
			}

			if orch.ArtifactsDir != "" {
				jobDir := filepath.Join(orch.ArtifactsDir, job.ID)
				entries, err := os.ReadDir(jobDir)
				if err == nil {
					for _, entry := range entries {
						if !entry.IsDir() {
							filePath := filepath.Join(jobDir, entry.Name())
							fileData, err := os.ReadFile(filePath)
							if err == nil {
								hdr := &archive_tar.Header{
									Name: fmt.Sprintf("%s/artifacts/%s", job.ID, entry.Name()),
									Mode: 0600,
									Size: int64(len(fileData)),
								}
								if err := tarWriter.WriteHeader(hdr); err == nil {
									tarWriter.Write(fileData)
								}
							}
						}
					}
				}
			}
		}
	})

	mux.HandleFunc("POST /jobs/{id}/metrics", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Metrics map[string]float64 `json:"metrics"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.AddJobMetrics(id, req.Metrics, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "success", "job_id": "%s"}`, id)
	})

	mux.HandleFunc("POST /jobs/{id}/output", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Outputs map[string]string `json:"outputs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.SetJobOutput(id, req.Outputs, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "success", "job_id": "%s"}`, id)
	})

	mux.HandleFunc("PUT /jobs/dependencies", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk dependency update", http.StatusBadRequest)
			return
		}
		if tag != "" && match != "" {
			http.Error(w, "Cannot provide both 'tag' and 'match' query parameters for bulk dependency update", http.StatusBadRequest)
			return
		}

		var req struct {
			DependsOn []string `json:"depends_on"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		var count int
		var err error

		if tag != "" {
			count, err = orch.UpdateJobsDependenciesByTag(r.Context(), tag, req.DependsOn, logger)
		} else if match != "" {
			count, err = orch.UpdateJobsDependenciesByMatch(r.Context(), match, req.DependsOn, logger)
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"updated": %d}`, count)
	})

	mux.HandleFunc("PUT /jobs/env", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk env update", http.StatusBadRequest)
			return
		}
		if tag != "" && match != "" {
			http.Error(w, "Cannot provide both 'tag' and 'match' query parameters for bulk env update", http.StatusBadRequest)
			return
		}

		var req struct {
			EnvVars map[string]string `json:"env_vars"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		var count int
		var err error

		if tag != "" {
			count, err = orch.UpdateJobsEnvByTag(r.Context(), tag, req.EnvVars, logger)
		} else if match != "" {
			count, err = orch.UpdateJobsEnvByMatch(r.Context(), match, req.EnvVars, logger)
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"updated": %d}`, count)
	})

	mux.HandleFunc("PUT /jobs/{id}/dependencies", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			DependsOn []string `json:"depends_on"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobDependencies(r.Context(), id, req.DependsOn, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "circular dependency") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respData, _ := json.Marshal(req)
		w.Write(respData)
	})

	mux.HandleFunc("PUT /jobs/{id}/progress", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Progress      *int    `json:"progress,omitempty"`
			StatusMessage *string `json:"status_message,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobProgress(id, req.Progress, req.StatusMessage, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respData, _ := json.Marshal(req)
		w.Write(respData)
	})

	mux.HandleFunc("PUT /jobs/priority", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk priority update", http.StatusBadRequest)
			return
		}
		if tag != "" && match != "" {
			http.Error(w, "Cannot provide both 'tag' and 'match' query parameters for bulk priority update", http.StatusBadRequest)
			return
		}

		var req struct {
			Priority int `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		var count int
		var err error

		if tag != "" {
			count, err = orch.UpdateJobsPriorityByTag(r.Context(), tag, req.Priority, logger)
		} else if match != "" {
			count, err = orch.UpdateJobsPriorityByMatch(r.Context(), match, req.Priority, logger)
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"updated": %d}`, count)
	})
	mux.HandleFunc("PUT /jobs/{id}/priority", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Priority int `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobPriority(r.Context(), id, req.Priority, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"priority": %d}`, req.Priority)
	})

	mux.HandleFunc("PUT /jobs/{id}/artifacts/{filename}", handleUploadArtifact(orch, logger))
	mux.HandleFunc("GET /jobs/{id}/artifacts/{filename}", handleDownloadArtifact(orch, logger))
	mux.HandleFunc("GET /jobs/{id}/artifacts", handleListArtifacts(orch, logger))
	mux.HandleFunc("DELETE /jobs/{id}/artifacts/{filename}", handleDeleteArtifact(orch, logger))

	mux.HandleFunc("POST /jobs/demote/bulk", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")
		group := r.URL.Query().Get("group")

		var count int
		var err error

		if tag != "" {
			count, err = orch.DemoteJobsByTag(r.Context(), tag, logger)
		} else if match != "" {
			count, err = orch.DemoteJobsByMatch(r.Context(), match, logger)
		} else if group != "" {
			count, err = orch.DemoteJobsByGroup(r.Context(), group, logger)
		} else {
			http.Error(w, "Either 'tag', 'match', or 'group' query parameter is required for bulk demote", http.StatusBadRequest)
			return
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"demoted": count,
		})
	})

	mux.HandleFunc("POST /jobs/{id}/demote", func(w http.ResponseWriter, r *http.Request) {
		jobID := r.PathValue("id")

		newPriority, err := orch.DemoteJob(baseCtx, jobID, logger)
		if err != nil {
			if err.Error() == "job "+jobID+" not found in pending queue" {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"priority": %d}`, newPriority)
	})

	mux.HandleFunc("POST /jobs/promote/bulk", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")
		group := r.URL.Query().Get("group")

		var count int
		var err error

		if tag != "" {
			count, err = orch.PromoteJobsByTag(r.Context(), tag, logger)
		} else if match != "" {
			count, err = orch.PromoteJobsByMatch(r.Context(), match, logger)
		} else if group != "" {
			count, err = orch.PromoteJobsByGroup(r.Context(), group, logger)
		} else {
			http.Error(w, "Either 'tag', 'match', or 'group' query parameter is required for bulk promote", http.StatusBadRequest)
			return
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"promoted": count,
		})
	})

	mux.HandleFunc("POST /jobs/{id}/promote", func(w http.ResponseWriter, r *http.Request) {
		jobID := r.PathValue("id")

		newPriority, err := orch.PromoteJob(baseCtx, jobID, logger)
		if err != nil {
			if err.Error() == "job "+jobID+" not found in pending queue" {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       jobID,
			"priority": newPriority,
		})
	})

	mux.HandleFunc("PUT /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var newItem WorkItem
		if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if newItem.ID == "" {
			newItem.ID = id
		} else if newItem.ID != id {
			http.Error(w, "Job ID in body must match URL", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobWorkItem(r.Context(), id, newItem, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s work item updated", id)
	})

	mux.HandleFunc("PUT /jobs/{id}/env", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			EnvVars map[string]string `json:"env_vars"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobEnv(r.Context(), id, req.EnvVars, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respData, _ := json.Marshal(req)
		w.Write(respData)
	})


	mux.HandleFunc("PUT /jobs/{id}/tags/add", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.AddJobTags(r.Context(), id, req.Tags, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Tags added successfully"}`))
	})

	mux.HandleFunc("PUT /jobs/{id}/tags/remove", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.RemoveJobTags(r.Context(), id, req.Tags, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Tags removed successfully"}`))
	})

	mux.HandleFunc("PUT /jobs/tags/add", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk tag add", http.StatusBadRequest)
			return
		}

		if tag != "" && match != "" {
			http.Error(w, "Only one of 'tag' or 'match' query parameter can be provided", http.StatusBadRequest)
			return
		}

		var req struct {
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		var regex *regexp.Regexp
		if match != "" {
			var err error
			regex, err = regexp.Compile("(?i)" + match)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid regex match parameter: %v", err), http.StatusBadRequest)
				return
			}
		}

		updatedCount := 0
		for _, job := range orch.GetPendingJobs() {
			matches := false
			if tag != "" {
				for _, t := range job.WorkItem.Tags {
					if strings.EqualFold(t, tag) {
						matches = true
						break
					}
				}
			} else if regex != nil {
				if regex.MatchString(job.ID) || regex.MatchString(job.WorkItem.Description) || regex.MatchString(job.WorkItem.Summary) {
					matches = true
				}
			}

			if matches {
				if err := orch.AddJobTags(r.Context(), job.ID, req.Tags, logger); err == nil {
					updatedCount++
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respData, _ := json.Marshal(map[string]int{"updated": updatedCount})
		w.Write(respData)
	})

	mux.HandleFunc("PUT /jobs/tags/remove", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk tag remove", http.StatusBadRequest)
			return
		}

		if tag != "" && match != "" {
			http.Error(w, "Only one of 'tag' or 'match' query parameter can be provided", http.StatusBadRequest)
			return
		}

		var req struct {
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		var regex *regexp.Regexp
		if match != "" {
			var err error
			regex, err = regexp.Compile("(?i)" + match)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid regex match parameter: %v", err), http.StatusBadRequest)
				return
			}
		}

		updatedCount := 0
		for _, job := range orch.GetPendingJobs() {
			matches := false
			if tag != "" {
				for _, t := range job.WorkItem.Tags {
					if strings.EqualFold(t, tag) {
						matches = true
						break
					}
				}
			} else if regex != nil {
				if regex.MatchString(job.ID) || regex.MatchString(job.WorkItem.Description) || regex.MatchString(job.WorkItem.Summary) {
					matches = true
				}
			}

			if matches {
				if err := orch.RemoveJobTags(r.Context(), job.ID, req.Tags, logger); err == nil {
					updatedCount++
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respData, _ := json.Marshal(map[string]int{"updated": updatedCount})
		w.Write(respData)
	})

	mux.HandleFunc("PUT /jobs/tags", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk tag update", http.StatusBadRequest)
			return
		}
		if tag != "" && match != "" {
			http.Error(w, "Cannot provide both 'tag' and 'match' query parameters for bulk tag update", http.StatusBadRequest)
			return
		}

		var req struct {
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		var count int
		var err error

		if tag != "" {
			count, err = orch.UpdateJobsTagsByTag(r.Context(), tag, req.Tags, logger)
		} else if match != "" {
			count, err = orch.UpdateJobsTagsByMatch(r.Context(), match, req.Tags, logger)
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"updated": %d}`, count)
	})

	mux.HandleFunc("PUT /jobs/agent", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk agent update", http.StatusBadRequest)
			return
		}
		if tag != "" && match != "" {
			http.Error(w, "Cannot provide both 'tag' and 'match' query parameters for bulk agent update", http.StatusBadRequest)
			return
		}

		var req struct {
			AgentProvider string `json:"agent_provider"`
			AgentModel    string `json:"agent_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		var count int
		var err error

		if tag != "" {
			count, err = orch.UpdateJobsAgentByTag(r.Context(), tag, req.AgentProvider, req.AgentModel, logger)
		} else if match != "" {
			count, err = orch.UpdateJobsAgentByMatch(r.Context(), match, req.AgentProvider, req.AgentModel, logger)
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"updated": %d}`, count)
	})

	mux.HandleFunc("PUT /jobs/{id}/tags", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobTags(r.Context(), id, req.Tags, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "tags updated successfully"})
	})

	mux.HandleFunc("PUT /jobs/{id}/agent", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			AgentProvider string `json:"agent_provider"`
			AgentModel    string `json:"agent_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobAgent(r.Context(), id, req.AgentProvider, req.AgentModel, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respData, _ := json.Marshal(req)
		w.Write(respData)
	})

	mux.HandleFunc("PUT /jobs/max-retries", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either tag or match query parameter is required", http.StatusBadRequest)
			return
		}

		var req struct {
			MaxRetries int `json:"max_retries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		var count int
		var err error

		if tag != "" {
			count, err = orch.UpdateJobsMaxRetriesByTag(r.Context(), tag, req.MaxRetries, logger)
		} else {
			count, err = orch.UpdateJobsMaxRetriesByMatch(r.Context(), match, req.MaxRetries, logger)
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to update max retries: %v", err), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"updated": %d, "max_retries": %d}`, count, req.MaxRetries)
	})

	mux.HandleFunc("PUT /jobs/{id}/max-retries", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			MaxRetries int `json:"max_retries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobMaxRetries(r.Context(), id, req.MaxRetries, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"max_retries": %d}`, req.MaxRetries)
	})

	mux.HandleFunc("PUT /jobs/timeout", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk timeout update", http.StatusBadRequest)
			return
		}
		if tag != "" && match != "" {
			http.Error(w, "Cannot provide both 'tag' and 'match' query parameters for bulk timeout update", http.StatusBadRequest)
			return
		}

		var req struct {
			Timeout string `json:"timeout"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		parsedTimeout, err := time.ParseDuration(req.Timeout)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid timeout format: %v", err), http.StatusBadRequest)
			return
		}

		var count int

		if tag != "" {
			count, err = orch.UpdateJobsTimeoutByTag(r.Context(), tag, parsedTimeout, logger)
		} else if match != "" {
			count, err = orch.UpdateJobsTimeoutByMatch(r.Context(), match, parsedTimeout, logger)
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"updated": %d}`, count)
	})

	mux.HandleFunc("PUT /jobs/{id}/rename", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			NewID string `json:"new_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if req.NewID == "" {
			http.Error(w, "new_id is required", http.StatusBadRequest)
			return
		}

		if err := orch.RenameJob(r.Context(), id, req.NewID, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") || strings.Contains(err.Error(), "already exists") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s renamed to %s", id, req.NewID)
	})

	mux.HandleFunc("PUT /jobs/{id}/timeout", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Timeout string `json:"timeout"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		parsedTimeout, err := time.ParseDuration(req.Timeout)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid timeout format: %v", err), http.StatusBadRequest)
			return
		}

		if err := orch.UpdateJobTimeout(r.Context(), id, parsedTimeout, logger); err != nil {
			if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"timeout": "%s"}`, parsedTimeout.String())
	})

	mux.HandleFunc("POST /jobs/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		downstream := r.URL.Query().Get("downstream") == "true"

		var overrides *RetryOverrides
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				var ov RetryOverrides
				if err := json.Unmarshal(bodyBytes, &ov); err != nil {
					http.Error(w, "Invalid JSON body", http.StatusBadRequest)
					return
				}
				overrides = &ov
			}
		}

		// Use r.Context() but ensure logger is available
		if downstream {
			retriedJobs, err := orch.RetryJobDownstream(r.Context(), id, overrides, logger)
			if err != nil {
				if strings.Contains(err.Error(), "active") || strings.Contains(err.Error(), "pending") {
					http.Error(w, err.Error(), http.StatusConflict)
				} else if strings.Contains(err.Error(), "not found") {
					http.Error(w, err.Error(), http.StatusNotFound)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"retried_jobs": retriedJobs}); err != nil {
				logger.Error("Failed to encode retried jobs response", "error", err)
			}
		} else {
			if err := orch.RetryJob(r.Context(), id, overrides, logger); err != nil {
				if strings.Contains(err.Error(), "already active") {
					http.Error(w, err.Error(), http.StatusConflict)
				} else if strings.Contains(err.Error(), "not found") {
					http.Error(w, err.Error(), http.StatusNotFound)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprintf(w, "Job %s retry submitted", id)
		}
	})

	mux.HandleFunc("POST /jobs/approve", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		var count int
		var err error

		if tag != "" {
			count, err = orch.ApproveJobsByTag(r.Context(), tag, logger)
		} else if match != "" {
			count, err = orch.ApproveJobsByMatch(r.Context(), match, logger)
		} else {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk approve", http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"approved": %d}`, count)
	})

	mux.HandleFunc("POST /jobs/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.ApproveJob(r.Context(), id, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else if strings.Contains(err.Error(), "not pending approval") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s approved", id)
	})

	mux.HandleFunc("POST /jobs/hold", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		var count int
		var err error

		if tag != "" {
			count, err = orch.HoldJobsByTag(r.Context(), tag, logger)
		} else if match != "" {
			count, err = orch.HoldJobsByMatch(r.Context(), match, logger)
		} else {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk hold", http.StatusBadRequest)
			return
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"held": %d}`, count)
	})

	mux.HandleFunc("POST /jobs/unhold", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		var count int
		var err error

		if tag != "" {
			count, err = orch.UnholdJobsByTag(r.Context(), tag, logger)
		} else if match != "" {
			count, err = orch.UnholdJobsByMatch(r.Context(), match, logger)
		} else {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk unhold", http.StatusBadRequest)
			return
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"unheld": %d}`, count)
	})

	mux.HandleFunc("POST /jobs/{id}/hold", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.HoldJob(r.Context(), id, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s held", id)
	})

	mux.HandleFunc("POST /jobs/{id}/unhold", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.UnholdJob(r.Context(), id, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else if strings.Contains(err.Error(), "already active") || strings.Contains(err.Error(), "already completed") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s unheld", id)
	})

	mux.HandleFunc("POST /jobs/skip", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")
		group := r.URL.Query().Get("group")

		var count int
		var err error

		if tag != "" {
			count, err = orch.SkipJobsByTag(r.Context(), tag, logger)
		} else if match != "" {
			count, err = orch.SkipJobsByMatch(r.Context(), match, logger)
		} else if group != "" {
			count, err = orch.SkipJobsByGroup(r.Context(), group, logger)
		} else {
			http.Error(w, "Either 'tag', 'match', or 'group' query parameter is required for bulk skip", http.StatusBadRequest)
			return
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"skipped": %d}`, count)
	})

	mux.HandleFunc("POST /jobs/{id}/skip", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.SkipJob(r.Context(), id, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s skipped", id)
	})

	mux.HandleFunc("POST /jobs/clone/bulk", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		if tag == "" && match == "" {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk clone", http.StatusBadRequest)
			return
		}

		var overrides struct {
			EnvVars           map[string]string `json:"env_vars"`
			Priority          *int              `json:"priority"`
			DependsOn         []string          `json:"depends_on"`
			RemapDependencies bool              `json:"remap_dependencies"`
		}

		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				if err := json.Unmarshal(bodyBytes, &overrides); err != nil {
					http.Error(w, "Invalid JSON body", http.StatusBadRequest)
					return
				}
			}
		}

		jobs := append(orch.GetActiveJobs(), orch.GetCompletedJobs()...)
		var filtered []JobInfo

		if tag != "" {
			for _, job := range jobs {
				hasTag := false
				for _, t := range job.WorkItem.Tags {
					if strings.EqualFold(t, tag) {
						hasTag = true
						break
					}
				}
				if hasTag {
					filtered = append(filtered, job)
				}
			}
		} else if match != "" {
			matcher, err := regexp.Compile("(?i)" + match)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid match regex: %v", err), http.StatusBadRequest)
				return
			}
			for _, job := range jobs {
				if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
					filtered = append(filtered, job)
				}
			}
		}

		// Pre-generate new IDs and build mapping if remap is requested
		idMap := make(map[string]string)
		nowNano := time.Now().UnixNano()
		for i, job := range filtered {
			idMap[job.ID] = fmt.Sprintf("%s-clone-%d-%d", job.ID, nowNano, i)
		}

		var clonedIDs []string
		for _, job := range filtered {
			newItem := job.WorkItem
			newItem.ID = idMap[job.ID]

			if job.WorkItem.EnvVars != nil {
				newItem.EnvVars = make(map[string]string)
				for k, v := range job.WorkItem.EnvVars {
					newItem.EnvVars[k] = v
				}
			}
			if overrides.EnvVars != nil {
				if newItem.EnvVars == nil {
					newItem.EnvVars = make(map[string]string)
				}
				for k, v := range overrides.EnvVars {
					newItem.EnvVars[k] = v
				}
			}

			if overrides.Priority != nil {
				newItem.Priority = *overrides.Priority
			}

			if overrides.DependsOn != nil {
				newItem.DependsOn = make([]string, len(overrides.DependsOn))
				copy(newItem.DependsOn, overrides.DependsOn)
			} else if job.WorkItem.DependsOn != nil {
				newItem.DependsOn = make([]string, len(job.WorkItem.DependsOn))
				copy(newItem.DependsOn, job.WorkItem.DependsOn)

				if overrides.RemapDependencies {
					for idx, dep := range newItem.DependsOn {
						if newDepID, exists := idMap[dep]; exists {
							newItem.DependsOn[idx] = newDepID
						}
					}
				}
			}

			if err := orch.SubmitJob(baseCtx, newItem, logger); err != nil {
				logger.Error("Failed to submit bulk cloned job", "job_id", newItem.ID, "error", err)
				continue
			}
			clonedIDs = append(clonedIDs, newItem.ID)
		}

		respData := map[string]interface{}{
			"cloned":         len(clonedIDs),
			"cloned_job_ids": clonedIDs,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(respData); err != nil {
			logger.Error("Failed to encode bulk clone response", "error", err)
		}
	})

	mux.HandleFunc("POST /jobs/{id}/clone", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var overrides struct {
			NewID     string            `json:"new_id"`
			EnvVars   map[string]string `json:"env_vars"`
			Priority  *int              `json:"priority"`
			DependsOn []string          `json:"depends_on"`
		}

		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				if err := json.Unmarshal(bodyBytes, &overrides); err != nil {
					http.Error(w, "Invalid JSON body", http.StatusBadRequest)
					return
				}
			}
		}

		job, err := orch.GetJob(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		newItem := job.WorkItem

		if overrides.NewID != "" {
			newItem.ID = overrides.NewID
		} else {
			newItem.ID = fmt.Sprintf("%s-clone-%d", newItem.ID, time.Now().UnixNano())
		}

		if job.WorkItem.EnvVars != nil {
			newItem.EnvVars = make(map[string]string)
			for k, v := range job.WorkItem.EnvVars {
				newItem.EnvVars[k] = v
			}
		}
		if overrides.EnvVars != nil {
			if newItem.EnvVars == nil {
				newItem.EnvVars = make(map[string]string)
			}
			for k, v := range overrides.EnvVars {
				newItem.EnvVars[k] = v
			}
		}

		if overrides.Priority != nil {
			newItem.Priority = *overrides.Priority
		}

		if overrides.DependsOn != nil {
			newItem.DependsOn = make([]string, len(overrides.DependsOn))
			copy(newItem.DependsOn, overrides.DependsOn)
		} else if job.WorkItem.DependsOn != nil {
			newItem.DependsOn = make([]string, len(job.WorkItem.DependsOn))
			copy(newItem.DependsOn, job.WorkItem.DependsOn)
		}

		if err := orch.SubmitJob(baseCtx, newItem, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"cloned_job_id": "%s"}`, newItem.ID)
	})

	mux.HandleFunc("POST /jobs/{id}/force-complete", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.ForceCompleteJob(r.Context(), id, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else if strings.Contains(err.Error(), "already completed") || strings.Contains(err.Error(), "already skipped") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s force completed", id)
	})

	mux.HandleFunc("POST /jobs/force-complete", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		var count int
		var err error

		if tag != "" {
			count, err = orch.ForceCompleteJobsByTag(r.Context(), tag, logger)
		} else if match != "" {
			count, err = orch.ForceCompleteJobsByMatch(r.Context(), match, logger)
		} else {
			http.Error(w, "Either 'tag' or 'match' query parameter is required for bulk force complete", http.StatusBadRequest)
			return
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"force_completed": %d}`, count)
	})

	mux.HandleFunc("POST /jobs/{id}/fail", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.FailJob(r.Context(), id, logger); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s manually failed", id)
	})

	mux.HandleFunc("POST /jobs/fail", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")
		group := r.URL.Query().Get("group")

		var count int
		var err error

		if tag != "" {
			count, err = orch.FailJobsByTag(r.Context(), tag, logger)
		} else if match != "" {
			count, err = orch.FailJobsByMatch(r.Context(), match, logger)
		} else if group != "" {
			count, err = orch.FailJobsByGroup(r.Context(), group, logger)
		} else {
			http.Error(w, "Either 'tag', 'match', or 'group' query parameter is required for bulk fail", http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"failed": %d}`, count)
	})

	mux.HandleFunc("POST /jobs/retry-failed", func(w http.ResponseWriter, r *http.Request) {
		match := r.URL.Query().Get("match")
		tag := r.URL.Query().Get("tag")
		count, err := orch.RetryFailedJobs(r.Context(), match, tag, logger)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"retried": %d}`, count)
	})

	mux.HandleFunc("POST /jobs/{id}/heal", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		newID, err := orch.HealJob(r.Context(), id, logger)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else if strings.Contains(err.Error(), "not in a failed state") || strings.Contains(err.Error(), "active") || strings.Contains(err.Error(), "pending") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"healed_job_id": "%s"}`, newID)
	})

	mux.HandleFunc("POST /jobs/heal/bulk", func(w http.ResponseWriter, r *http.Request) {
		match := r.URL.Query().Get("match")
		tag := r.URL.Query().Get("tag")

		if match == "" && tag == "" {
			http.Error(w, "Either 'match' or 'tag' query parameter is required", http.StatusBadRequest)
			return
		}

		count, err := orch.HealJobs(r.Context(), match, tag, logger)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"healed": %d}`, count)
	})

	mux.HandleFunc("DELETE /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		downstream := r.URL.Query().Get("downstream") == "true"

		if downstream {
			canceledIDs, err := orch.CancelJobDownstream(r.Context(), id, logger)
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					http.Error(w, err.Error(), http.StatusNotFound)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"canceled_jobs": canceledIDs}); err != nil {
				logger.Error("Failed to encode canceled jobs response", "error", err)
			}
		} else {
			if err := orch.CancelJob(r.Context(), id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Job %s cancellation requested", id)
		}
	})

	mux.HandleFunc("DELETE /jobs", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		status := r.URL.Query().Get("status")
		match := r.URL.Query().Get("match")
		group := r.URL.Query().Get("group")
		olderThanStr := r.URL.Query().Get("older_than")

		var count int
		var err error

		if olderThanStr != "" {
			d, parseErr := time.ParseDuration(olderThanStr)
			if parseErr != nil {
				http.Error(w, fmt.Sprintf("invalid duration for older_than: %v", parseErr), http.StatusBadRequest)
				return
			}
			count, err = orch.CancelJobsOlderThan(r.Context(), d, logger)
		} else if tag != "" {
			count, err = orch.CancelJobsByTag(r.Context(), tag, logger)
		} else if status != "" {
			count, err = orch.CancelJobsByStatus(r.Context(), status, logger)
		} else if match != "" {
			count, err = orch.CancelJobsByMatch(r.Context(), match, logger)
		} else if group != "" {
			count, err = orch.CancelJobsByConcurrencyGroup(r.Context(), group, logger)
		} else {
			count, err = orch.CancelAllJobs(r.Context())
		}

		if err != nil && count == 0 {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"canceled": %d}`, count)
	})

	mux.HandleFunc("DELETE /jobs/{id}/pending", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.DeletePendingJob(r.Context(), id, logger); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Pending job %s deleted", id)
	})

	mux.HandleFunc("DELETE /jobs/pending", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")
		group := r.URL.Query().Get("group")

		if tag == "" && match == "" && group == "" {
			http.Error(w, "Either 'tag', 'match', or 'group' query parameter is required for bulk delete pending jobs", http.StatusBadRequest)
			return
		}

		var count int
		var err error

		if tag != "" {
			count, err = orch.DeletePendingJobsByTag(r.Context(), tag, logger)
		} else if match != "" {
			count, err = orch.DeletePendingJobsByMatch(r.Context(), match, logger)
		} else if group != "" {
			count, err = orch.DeletePendingJobsByConcurrencyGroup(r.Context(), group, logger)
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"deleted": %d}`, count)
	})

	mux.HandleFunc("DELETE /pending", func(w http.ResponseWriter, r *http.Request) {
		count := orch.ClearPendingJobs(r.Context(), logger)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"cleared": %d}`, count)
	})

	mux.HandleFunc("DELETE /history/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := orch.PurgeJob(id, logger); err != nil {
			if strings.Contains(err.Error(), "cannot purge") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Job %s purged successfully", id)
	})

	mux.HandleFunc("DELETE /history", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		status := r.URL.Query().Get("status")
		match := r.URL.Query().Get("match")
		group := r.URL.Query().Get("group")
		olderThanStr := r.URL.Query().Get("older_than")

		var count int
		var err error

		if olderThanStr != "" {
			d, parseErr := time.ParseDuration(olderThanStr)
			if parseErr != nil {
				http.Error(w, fmt.Sprintf("invalid duration for older_than: %v", parseErr), http.StatusBadRequest)
				return
			}
			count, err = orch.PurgeJobsOlderThan(d, logger)
		} else if tag != "" {
			count, err = orch.PurgeJobsByTag(tag, logger)
		} else if status != "" {
			count, err = orch.PurgeJobsByStatus(status, logger)
		} else if match != "" {
			count, err = orch.PurgeJobsByMatch(match, logger)
		} else if group != "" {
			count, err = orch.PurgeJobsByGroup(group, logger)
		} else {
			count, err = orch.ClearHistory(logger)
		}

		if err != nil {
			if strings.Contains(err.Error(), "invalid match regex") {
				http.Error(w, fmt.Sprintf("Failed to clear history: %v", err), http.StatusBadRequest)
			} else {
				http.Error(w, fmt.Sprintf("Failed to clear history: %v", err), http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"cleared": count})
	})

	mux.HandleFunc("POST /poll", func(w http.ResponseWriter, r *http.Request) {
		orch.ForcePoll()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator poll triggered")
	})

	mux.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		var item WorkItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if item.ID == "" {
			http.Error(w, "Job ID is required", http.StatusBadRequest)
			return
		}

		// Use the baseCtx (captured from main) to ensure job runs independently of the request context
		// but respects orchestrator shutdown.
		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if err == ErrDraining {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "Job %s submitted successfully", item.ID)
	})

	mux.HandleFunc("POST /jobs/batch", func(w http.ResponseWriter, r *http.Request) {
		var items []WorkItem
		if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
			http.Error(w, "Invalid JSON array body", http.StatusBadRequest)
			return
		}

		for _, item := range items {
			if item.ID == "" {
				http.Error(w, "Job ID is required for all jobs in batch", http.StatusBadRequest)
				return
			}
		}

		submitted := make([]string, 0)
		errors := make([]string, 0)

		for _, item := range items {
			if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
				if err == ErrAtCapacity {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "at capacity"))
				} else if err == ErrDraining {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "draining"))
				} else if strings.Contains(err.Error(), "already active") {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "already active"))
				} else {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, err.Error()))
				}
			} else {
				submitted = append(submitted, item.ID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"submitted": submitted,
			"errors":    errors,
		}); err != nil {
			logger.Error("Failed to encode batch submission response", "error", err)
		}
	})

	mux.HandleFunc("POST /jobs/pipeline", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		target := r.URL.Query().Get("target")

		vars := make(map[string]string)
		if r.URL.Query().Has("var") {
			for _, v := range r.URL.Query()["var"] {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}
		}

		items, err := ParsePipelineToWorkItems(bodyBytes, target, vars, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		submitted := make([]string, 0)
		errors := make([]string, 0)

		for _, item := range items {
			if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
				if err == ErrAtCapacity {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "at capacity"))
				} else if err == ErrDraining {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "draining"))
				} else if strings.Contains(err.Error(), "already active") {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "already active"))
				} else {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, err.Error()))
				}
			} else {
				submitted = append(submitted, item.ID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"submitted": submitted,
			"errors":    errors,
		}); err != nil {
			logger.Error("Failed to encode pipeline submission response", "error", err)
		}
	})

	mux.HandleFunc("POST /jobs/pipeline/import", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		target := r.URL.Query().Get("target")

		vars := make(map[string]string)
		if r.URL.Query().Has("var") {
			for _, v := range r.URL.Query()["var"] {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}
		}

		items, err := ParsePipelineToWorkItems(bodyBytes, target, vars, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		submitted := make([]string, 0)
		errors := make([]string, 0)

		for _, item := range items {
			item.Hold = true // Key difference: hold the job so it doesn't start automatically
			if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
				if err == ErrAtCapacity {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "at capacity"))
				} else if err == ErrDraining {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "draining"))
				} else if strings.Contains(err.Error(), "already active") {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "already active"))
				} else {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, err.Error()))
				}
			} else {
				submitted = append(submitted, item.ID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"submitted": submitted,
			"errors":    errors,
		}); err != nil {
			logger.Error("Failed to encode pipeline import response", "error", err)
		}
	})

	mux.HandleFunc("POST /jobs/pipeline/dry-run", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		target := r.URL.Query().Get("target")

		vars := make(map[string]string)
		if r.URL.Query().Has("var") {
			for _, v := range r.URL.Query()["var"] {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					vars[parts[0]] = parts[1]
				}
			}
		}

		items, err := ParsePipelineToWorkItems(bodyBytes, target, vars, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(items); err != nil {
			logger.Error("Failed to encode pipeline dry-run response", "error", err)
		}
	})

	mux.HandleFunc("POST /jobs/matrix", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			BaseItem WorkItem            `json:"base_item"`
			Matrix   map[string][]string `json:"matrix"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if req.BaseItem.ID == "" {
			http.Error(w, "Base Job ID is required", http.StatusBadRequest)
			return
		}

		// Generate combinations deterministically (sort keys)
		var keys []string
		for k := range req.Matrix {
			keys = append(keys, k)
		}

		// Sort keys to ensure deterministic combination generation
		sort.Strings(keys)

		var combinations []map[string]string
		var generate func(int, map[string]string)
		generate = func(idx int, current map[string]string) {
			if idx == len(keys) {
				cp := make(map[string]string)
				for k, v := range current {
					cp[k] = v
				}
				combinations = append(combinations, cp)
				return
			}
			key := keys[idx]
			for _, val := range req.Matrix[key] {
				current[key] = val
				generate(idx+1, current)
			}
		}
		generate(0, make(map[string]string))

		// If matrix is empty, just submit the base item
		if len(combinations) == 0 {
			combinations = append(combinations, make(map[string]string))
		}

		submitted := make([]string, 0)
		errors := make([]string, 0)

		for i, combo := range combinations {
			item := req.BaseItem // shallow copy

			// deep copy EnvVars
			item.EnvVars = make(map[string]string)
			for k, v := range req.BaseItem.EnvVars {
				item.EnvVars[k] = v
			}

			// Add matrix variables
			suffixParts := []string{}
			for _, k := range keys {
				if v, ok := combo[k]; ok {
					item.EnvVars[k] = v
					suffixParts = append(suffixParts, fmt.Sprintf("%s=%s", k, v))
				}
			}

			if len(suffixParts) > 0 {
				item.ID = fmt.Sprintf("%s-%d", req.BaseItem.ID, i+1)
				item.Summary = fmt.Sprintf("%s [%s]", req.BaseItem.Summary, strings.Join(suffixParts, ", "))
			}

			// Deep copy DependsOn
			if req.BaseItem.DependsOn != nil {
				item.DependsOn = make([]string, len(req.BaseItem.DependsOn))
				copy(item.DependsOn, req.BaseItem.DependsOn)
			}
			// Deep copy Tags
			if req.BaseItem.Tags != nil {
				item.Tags = make([]string, len(req.BaseItem.Tags))
				copy(item.Tags, req.BaseItem.Tags)
			}

			if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
				if err == ErrAtCapacity {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "at capacity"))
				} else if err == ErrDraining {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "draining"))
				} else if strings.Contains(err.Error(), "already active") {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, "already active"))
				} else {
					errors = append(errors, fmt.Sprintf("%s: %v", item.ID, err.Error()))
				}
			} else {
				submitted = append(submitted, item.ID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"submitted": submitted,
			"errors":    errors,
		}); err != nil {
			logger.Error("Failed to encode matrix submission response", "error", err)
		}
	})

	mux.HandleFunc("POST /webhook/jira", func(w http.ResponseWriter, r *http.Request) {
		secret := viper.GetString("orchestrator.jira_webhook_secret")
		if secret != "" {
			reqSecret := r.URL.Query().Get("secret")
			if reqSecret == "" {
				http.Error(w, "Missing secret query parameter", http.StatusUnauthorized)
				return
			}
			// Use constant-time comparison to prevent timing attacks
			if !hmac.Equal([]byte(reqSecret), []byte(secret)) {
				http.Error(w, "Invalid secret query parameter", http.StatusUnauthorized)
				return
			}
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		// Webhook event type is typically in `webhookEvent`. We care about creations/updates.
		event, ok := payload["webhookEvent"].(string)
		if ok && event != "jira:issue_created" && event != "jira:issue_updated" {
			// Ignore other events
			w.WriteHeader(http.StatusOK)
			return
		}

		issue, ok := payload["issue"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing issue in payload", http.StatusBadRequest)
			return
		}

		key, _ := issue["key"].(string)
		fields, _ := issue["fields"].(map[string]interface{})
		if key == "" || fields == nil {
			http.Error(w, "Missing issue key or fields", http.StatusBadRequest)
			return
		}

		summary, _ := fields["summary"].(string)
		description, _ := fields["description"].(string)

		// A hack since ParseDescription needs Client, but we just want string extraction if it's there
		// Normally Jira webhooks might have ADF or string description
		// For simplicity we just use what we get. The Jira poller Client handles ADF but we can't easily here.
		// If description is missing or complex, we will still extract what we can.
		// In Jira, fields["description"] might be string or map. Let's try to handle string.
		if description == "" {
			// if it's an object (ADF), this might be nil, but let's assume standard extraction for now.
			// Or we can try to JSON marshal it to string if it's an object.
			if descObj, ok := fields["description"].(map[string]interface{}); ok {
				if dBytes, err := json.Marshal(descObj); err == nil {
					description = string(dBytes)
				}
			}
		}

		repoURL := extractRepoURL(description, jira.RepoRegex)
		if repoURL == "" {
			// No repo URL found, we can't process it. But we shouldn't fail the webhook.
			w.WriteHeader(http.StatusOK)
			return
		}

		item := WorkItem{
			ID:               key,
			Summary:          summary,
			Description:      description,
			RepoURL:          repoURL,
			ConcurrencyGroup: key, // Jira tickets act as their own concurrency group
			CancelInProgress: true,
			EnvVars: map[string]string{
				"JIRA_TICKET": key,
			},
		}

		if features := extractRequiredFeatures(description); len(features) > 0 {
			fl := db.FeatureList{
				ProjectName: summary,
				Features:    features,
			}
			if data, err := json.Marshal(fl); err == nil {
				item.EnvVars["RECAC_INJECTED_FEATURES"] = string(data)
			}
		}

		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "Jira Webhook Job %s submitted successfully", key)
	})

	mux.HandleFunc("POST /webhook/github", func(w http.ResponseWriter, r *http.Request) {
		event := r.Header.Get("X-GitHub-Event")
		if event != "issues" && event != "issue_comment" {
			// Ignore other events
			w.WriteHeader(http.StatusOK)
			return
		}

		// Read body for signature validation
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		// Restore body for any subsequent reading if necessary
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		secret := viper.GetString("orchestrator.github_webhook_secret")
		if secret != "" {
			signature := r.Header.Get("X-Hub-Signature-256")
			if signature == "" {
				http.Error(w, "Missing X-Hub-Signature-256 header", http.StatusUnauthorized)
				return
			}
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(bodyBytes)
			expectedMAC := hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(signature), []byte("sha256="+expectedMAC)) {
				http.Error(w, "Invalid X-Hub-Signature-256 signature", http.StatusUnauthorized)
				return
			}
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		action, _ := payload["action"].(string)
		if action != "opened" && action != "created" && action != "labeled" {
			// We only care about specific actions
			w.WriteHeader(http.StatusOK)
			return
		}

		// Extract fields
		issue, ok := payload["issue"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing issue in payload", http.StatusBadRequest)
			return
		}
		repoMap, ok := payload["repository"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing repository in payload", http.StatusBadRequest)
			return
		}

		repoURL, _ := repoMap["clone_url"].(string)
		repoName, _ := repoMap["name"].(string)
		issueNum, _ := issue["number"].(float64)
		title, _ := issue["title"].(string)

		description, _ := issue["body"].(string)

		// If it's a comment, use the comment body and append to description
		if event == "issue_comment" {
			comment, _ := payload["comment"].(map[string]interface{})
			if commentBody, ok := comment["body"].(string); ok {
				description = fmt.Sprintf("Comment:\n%s\n\nIssue Context:\n%s", commentBody, description)
			}
		}

		if repoURL == "" || issueNum == 0 {
			http.Error(w, "Missing required fields (repository.clone_url or issue.number)", http.StatusBadRequest)
			return
		}

		concurrencyGroup := fmt.Sprintf("gh-%s-%d", repoName, int(issueNum))
		jobID := fmt.Sprintf("%s-%d", concurrencyGroup, time.Now().UnixNano())

		item := WorkItem{
			ID:               jobID,
			Summary:          title,
			Description:      description,
			RepoURL:          repoURL,
			ConcurrencyGroup: concurrencyGroup,
			CancelInProgress: true,
			EnvVars: map[string]string{
				"GITHUB_ISSUE": fmt.Sprintf("%d", int(issueNum)),
				"GITHUB_REPO":  repoName,
			},
		}

		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "GitHub Webhook Job %s submitted successfully", jobID)
	})

	mux.HandleFunc("POST /webhook/linear", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		secret := viper.GetString("orchestrator.linear_webhook_secret")
		if secret != "" {
			signature := r.Header.Get("Linear-Signature")
			if signature == "" {
				http.Error(w, "Missing Linear-Signature header", http.StatusUnauthorized)
				return
			}
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(bodyBytes)
			expectedMAC := hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
				http.Error(w, "Invalid Linear-Signature header", http.StatusUnauthorized)
				return
			}
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		action, _ := payload["action"].(string)
		if action != "create" && action != "update" {
			w.WriteHeader(http.StatusOK)
			return
		}

		dataType, _ := payload["type"].(string)
		if dataType != "Issue" {
			w.WriteHeader(http.StatusOK)
			return
		}

		data, ok := payload["data"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing data in payload", http.StatusBadRequest)
			return
		}

		issueID, _ := data["id"].(string)
		issueIdentifier, _ := data["identifier"].(string)
		title, _ := data["title"].(string)
		description, _ := data["description"].(string)
		url, _ := payload["url"].(string)

		if issueID == "" {
			http.Error(w, "Missing required fields (data.id)", http.StatusBadRequest)
			return
		}

		concurrencyGroup := fmt.Sprintf("ln-%s", issueID)
		jobID := fmt.Sprintf("%s-%d", concurrencyGroup, time.Now().UnixNano())

		item := WorkItem{
			ID:               jobID,
			Summary:          title,
			Description:      description,
			RepoURL:          url,
			ConcurrencyGroup: concurrencyGroup,
			CancelInProgress: true,
			EnvVars: map[string]string{
				"LINEAR_ISSUE_ID":         issueID,
				"LINEAR_ISSUE_IDENTIFIER": issueIdentifier,
			},
		}

		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "Linear Webhook Job %s submitted successfully", jobID)
	})

	mux.HandleFunc("HEAD /webhook/trello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /webhook/trello", func(w http.ResponseWriter, r *http.Request) {
		// Read body for validation/parsing
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		// Restore body for any subsequent reading if necessary
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		secret := viper.GetString("orchestrator.trello_webhook_secret")
		if secret != "" {
			// Trello uses X-Trello-Webhook which is base64(hmac_sha1(request_body + callback_url, secret))
			// However, since we might not reliably know the callback_url in a reverse proxy setup,
			// we simply check a query parameter or custom header for simplicity if Trello supports it,
			// OR we perform the HMAC validation if we know the full URL.
			// Trello does not use a simple constant token, but users often append a ?token=XYZ to the webhook URL.
			// We will check for the X-Trello-Webhook header. We can skip the strict callback URL check
			// by just checking a custom token in the header if they set one, or doing standard Trello signature validation
			// But since callback URL is tricky, standard practice for simple webhooks is to check a query param.
			// Lets implement Trello HMAC validation properly: we need the absolute URL.
			// Trello signature: base64(hmac_sha1(payload + full_url, secret))
			signature := r.Header.Get("X-Trello-Webhook")
			if signature == "" {
				http.Error(w, "Missing X-Trello-Webhook header", http.StatusUnauthorized)
				return
			}

			scheme := "http"
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				scheme = "https"
			}
			host := r.Host
			if h := r.Header.Get("X-Forwarded-Host"); h != "" {
				host = h
			}
			callbackURL := fmt.Sprintf("%s://%s%s", scheme, host, r.URL.RequestURI())

			mac := hmac.New(sha1.New, []byte(secret))
			mac.Write(bodyBytes)
			mac.Write([]byte(callbackURL))
			expectedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
				http.Error(w, "Invalid X-Trello-Webhook signature", http.StatusUnauthorized)
				return
			}
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		action, ok := payload["action"].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusOK) // Ignore non-action payloads
			return
		}

		actionType, _ := action["type"].(string)
		if actionType != "createCard" && actionType != "updateCard" && actionType != "commentCard" {
			w.WriteHeader(http.StatusOK) // Ignore unsupported actions
			return
		}

		data, _ := action["data"].(map[string]interface{})
		card, _ := data["card"].(map[string]interface{})
		if card == nil {
			w.WriteHeader(http.StatusOK) // Missing card info
			return
		}

		cardID, _ := card["id"].(string)
		cardName, _ := card["name"].(string)
		cardDesc, _ := card["desc"].(string)

		// For updates, the desc might be in old or we just use the current one
		// We need to extract the repo URL from the description
		repoURL := extractRepoURL(cardDesc, RepoRegex)
		if repoURL == "" {
			// Also try to find it in the comment text if it is a commentCard
			if actionType == "commentCard" {
				if text, ok := data["text"].(string); ok {
					repoURL = extractRepoURL(text, RepoRegex)
				}
			}
		}

		item := WorkItem{
			ID:          cardID,
			Summary:     cardName,
			Description: cardDesc,
			RepoURL:     repoURL,
			EnvVars: map[string]string{
				"TRELLO_CARD_ID": cardID,
			},
		}

		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "Trello Webhook Job %s submitted successfully", cardID)
	})

	mux.HandleFunc("POST /webhook/generic", func(w http.ResponseWriter, r *http.Request) {
		if !viper.GetBool("orchestrator.generic_webhook_enabled") {
			http.Error(w, "Generic webhook is disabled", http.StatusForbidden)
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		secret := viper.GetString("orchestrator.generic_webhook_secret")
		if secret != "" {
			signature := r.Header.Get("X-Webhook-Signature")
			if signature == "" {
				http.Error(w, "Missing X-Webhook-Signature header", http.StatusUnauthorized)
				return
			}
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(bodyBytes)
			expectedMAC := hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(signature), []byte("sha256="+expectedMAC)) {
				http.Error(w, "Invalid X-Webhook-Signature header", http.StatusUnauthorized)
				return
			}
		}

		var item WorkItem
		if err := json.Unmarshal(bodyBytes, &item); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		if item.ID == "" {
			item.ID = fmt.Sprintf("webhook-%d", time.Now().UnixNano())
		}

		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if err == ErrDraining {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"job_id": "%s"}`, item.ID)
	})

	mux.HandleFunc("POST /webhook/gitlab", func(w http.ResponseWriter, r *http.Request) {
		event := r.Header.Get("X-Gitlab-Event")
		if event != "Issue Hook" && event != "Note Hook" {
			// Ignore other events
			w.WriteHeader(http.StatusOK)
			return
		}

		// Read body for validation/parsing
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		// Restore body for any subsequent reading if necessary
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		secret := viper.GetString("orchestrator.gitlab_webhook_secret")
		if secret != "" {
			token := r.Header.Get("X-Gitlab-Token")
			if token == "" {
				http.Error(w, "Missing X-Gitlab-Token header", http.StatusUnauthorized)
				return
			}
			// Use constant-time comparison to prevent timing attacks
			if !hmac.Equal([]byte(token), []byte(secret)) {
				http.Error(w, "Invalid X-Gitlab-Token header", http.StatusUnauthorized)
				return
			}
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		objectKind, _ := payload["object_kind"].(string)

		// Check if it's an issue or note on an issue
		if objectKind == "note" {
			attr, _ := payload["object_attributes"].(map[string]interface{})
			if noteableType, ok := attr["noteable_type"].(string); !ok || noteableType != "Issue" {
				w.WriteHeader(http.StatusOK)
				return
			}
		} else if objectKind != "issue" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Extract action from object_attributes.action
		attr, ok := payload["object_attributes"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing object_attributes in payload", http.StatusBadRequest)
			return
		}

		action, _ := attr["action"].(string)
		if objectKind == "issue" && action != "open" && action != "reopen" && action != "update" {
			// We only care about specific actions for issues
			w.WriteHeader(http.StatusOK)
			return
		}

		// For notes, the action might not be explicitly set, but object_kind == note means a comment was made
		// We can proceed.

		// Extract fields
		var issue map[string]interface{}
		if objectKind == "issue" {
			issue = attr
		} else if objectKind == "note" {
			issue, ok = payload["issue"].(map[string]interface{})
			if !ok {
				http.Error(w, "Missing issue in note payload", http.StatusBadRequest)
				return
			}
		}

		projectMap, ok := payload["project"].(map[string]interface{})
		if !ok {
			http.Error(w, "Missing project in payload", http.StatusBadRequest)
			return
		}

		repoURL, _ := projectMap["git_http_url"].(string)
		if repoURL == "" {
			repoURL, _ = projectMap["web_url"].(string) // Fallback
		}

		issueNum, _ := issue["iid"].(float64)
		title, _ := issue["title"].(string)
		description, _ := issue["description"].(string)

		// If it's a comment, use the comment body and append to description
		if objectKind == "note" {
			if commentBody, ok := attr["note"].(string); ok {
				description = fmt.Sprintf("Comment:\n%s\n\nIssue Context:\n%s", commentBody, description)
			}
		}

		if repoURL == "" || issueNum == 0 {
			http.Error(w, "Missing required fields (project.git_http_url or issue.iid)", http.StatusBadRequest)
			return
		}

		concurrencyGroup := fmt.Sprintf("gl-%d", int(issueNum))
		jobID := fmt.Sprintf("%s-%d", concurrencyGroup, time.Now().UnixNano())

		item := WorkItem{
			ID:               jobID,
			Summary:          title,
			Description:      description,
			RepoURL:          repoURL,
			ConcurrencyGroup: concurrencyGroup,
			CancelInProgress: true,
			EnvVars: map[string]string{
				"GITLAB_ISSUE": fmt.Sprintf("%d", int(issueNum)),
			},
		}

		if err := orch.SubmitJob(baseCtx, item, logger); err != nil {
			if err == ErrAtCapacity {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
			} else if strings.Contains(err.Error(), "already active") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "GitLab Webhook Job %s submitted successfully", jobID)
	})

	mux.HandleFunc("POST /pause", func(w http.ResponseWriter, r *http.Request) {
		orch.Pause()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator paused")
	})

	mux.HandleFunc("POST /resume", func(w http.ResponseWriter, r *http.Request) {
		orch.Resume()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator resumed")
	})

	mux.HandleFunc("POST /drain", func(w http.ResponseWriter, r *http.Request) {
		orch.Drain(logger)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator draining")
	})

	mux.HandleFunc("POST /undrain", func(w http.ResponseWriter, r *http.Request) {
		orch.Undrain(logger)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Orchestrator undraining")
	})

	mux.HandleFunc("POST /interval", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Interval string `json:"interval"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if req.Interval == "" {
			http.Error(w, "interval cannot be empty", http.StatusBadRequest)
			return
		}

		parsedInterval, err := time.ParseDuration(req.Interval)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid interval format: %v", err), http.StatusBadRequest)
			return
		}

		if parsedInterval <= 0 {
			http.Error(w, "interval must be greater than zero", http.StatusBadRequest)
			return
		}

		orch.UpdatePollInterval(parsedInterval)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"interval": "%s"}`, parsedInterval.String())
	})

	mux.HandleFunc("POST /groups/{group}/pause", func(w http.ResponseWriter, r *http.Request) {
		group := r.PathValue("group")
		if group == "" {
			http.Error(w, "Group is required", http.StatusBadRequest)
			return
		}
		orch.PauseGroup(group, logger)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Concurrency group %s paused", group)
	})

	mux.HandleFunc("POST /groups/{group}/resume", func(w http.ResponseWriter, r *http.Request) {
		group := r.PathValue("group")
		if group == "" {
			http.Error(w, "Group is required", http.StatusBadRequest)
			return
		}
		orch.ResumeGroup(group, logger)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Concurrency group %s resumed", group)
	})

	mux.HandleFunc("POST /scale", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxConcurrentJobs int `json:"max_concurrent_jobs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		// Prevent negative numbers
		if req.MaxConcurrentJobs < 0 {
			http.Error(w, "max_concurrent_jobs cannot be negative", http.StatusBadRequest)
			return
		}

		orch.SetConcurrency(baseCtx, req.MaxConcurrentJobs, logger)

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"max_concurrent_jobs": %d}`, req.MaxConcurrentJobs)
	})
}
