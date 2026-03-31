package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"

	"recac/internal/orchestrator"
)

// Define locally because DurationStats is not exported in internal/orchestrator
type DurationStats struct {
	MeanDuration float64   `json:"mean_duration_ms"`
	TagStats     []TagStat `json:"tag_stats"`
}

type TagStat struct {
	Tag          string  `json:"tag"`
	MeanDuration float64 `json:"mean_duration_ms"`
}

func simulateExecution(host string) {
	// 1. Fetch Status (for MaxConcurrentJobs)
	statusResp, err := http.Get(fmt.Sprintf("%s/status", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer statusResp.Body.Close()

	var status orchestrator.Status
	if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
		fmt.Fprintf(stdout, "Failed to decode status: %v\n", err)
		exitFunc(1)
		return
	}

	// 2. Fetch Duration Stats
	statsResp, err := http.Get(fmt.Sprintf("%s/jobs/analyze/durations", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to fetch duration stats: %v\n", err)
		exitFunc(1)
		return
	}
	defer statsResp.Body.Close()

	var stats DurationStats
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		fmt.Fprintf(stdout, "Failed to decode duration stats: %v\n", err)
		exitFunc(1)
		return
	}

	tagAverages := make(map[string]float64)
	for _, ts := range stats.TagStats {
		tagAverages[ts.Tag] = ts.MeanDuration
	}
	globalMean := stats.MeanDuration
	if globalMean <= 0 {
		globalMean = 60000 // default 60 seconds
	}

	// 3. Fetch Active Jobs (Running, Spawning, Pending)
	jobsResp, err := http.Get(fmt.Sprintf("%s/jobs?state=active", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to fetch active jobs: %v\n", err)
		exitFunc(1)
		return
	}
	defer jobsResp.Body.Close()

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(jobsResp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode jobs: %v\n", err)
		exitFunc(1)
		return
	}

	if len(jobs) == 0 {
		fmt.Fprintln(stdout, lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("No active or pending jobs. Simulation complete: 0s."))
		return
	}

	// Setup simulation state
	type SimJob struct {
		ID        string
		Priority  int
		Deps      []string
		InDegree  int
		Duration  float64 // ms
		Remaining float64 // ms
	}

	simJobs := make(map[string]*SimJob)
	dependents := make(map[string][]string)

	for _, job := range jobs {
		// Calculate estimated duration
		estDur := globalMean
		if len(job.WorkItem.Tags) > 0 {
			var sumDur float64
			var count int
			for _, tag := range job.WorkItem.Tags {
				if d, ok := tagAverages[tag]; ok {
					sumDur += d
					count++
				}
			}
			if count > 0 {
				estDur = sumDur / float64(count)
			}
		}

		sj := &SimJob{
			ID:       job.ID,
			Priority: job.WorkItem.Priority,
			Duration: estDur,
			Deps:     job.WorkItem.DependsOn,
		}

		if job.Status == "Running" || job.Status == "Spawning" {
			elapsed := float64(time.Since(job.StartTime).Milliseconds())
			rem := estDur - elapsed
			if rem <= 0 {
				rem = 1000 // At least 1 second remaining if it exceeded average
			}
			sj.Remaining = rem
		} else {
			sj.Remaining = estDur
		}

		simJobs[job.ID] = sj
	}

	// Calculate inDegrees and dependents mapping
	// Only count dependencies that are currently in our active pool (not completed)
	for _, sj := range simJobs {
		for _, dep := range sj.Deps {
			if _, exists := simJobs[dep]; exists {
				sj.InDegree++
				dependents[dep] = append(dependents[dep], sj.ID)
			}
		}
	}

	maxWorkers := status.MaxConcurrentJobs
	if maxWorkers <= 0 {
		maxWorkers = math.MaxInt32
	}
	workersAvailable := maxWorkers

	// Priority queue for ready jobs (inDegree == 0)
	var readyQueue []*SimJob

	// runningList tracks currently running jobs: map[jobID]finishTime
	runningList := make(map[string]float64)

	// Add currently running jobs
	for _, job := range jobs {
		if job.Status == "Running" || job.Status == "Spawning" {
			sj := simJobs[job.ID]
			runningList[sj.ID] = sj.Remaining
			workersAvailable--
		} else if simJobs[job.ID].InDegree == 0 {
			readyQueue = append(readyQueue, simJobs[job.ID])
		}
	}

	sortReadyQueue := func() {
		sort.SliceStable(readyQueue, func(i, j int) bool {
			if readyQueue[i].Priority == readyQueue[j].Priority {
				return readyQueue[i].ID < readyQueue[j].ID // Deterministic tie-breaker
			}
			return readyQueue[i].Priority > readyQueue[j].Priority
		})
	}

	var currentTime float64 = 0
	var lastFinishedJob string
	var totalProcessed int

	for len(readyQueue) > 0 || len(runningList) > 0 {
		// Assign available workers to ready jobs
		sortReadyQueue()
		for workersAvailable > 0 && len(readyQueue) > 0 {
			// Pop from front
			nextJob := readyQueue[0]
			readyQueue = readyQueue[1:]

			runningList[nextJob.ID] = currentTime + nextJob.Remaining
			workersAvailable--
		}

		if len(runningList) == 0 && len(readyQueue) == 0 {
			break
		}

		if len(runningList) == 0 && len(readyQueue) > 0 {
			// Actually this shouldn't happen unless workers=0?
			if workersAvailable <= 0 {
				break
			}
		}

		// Find the next finishing job
		nextTime := math.MaxFloat64
		for _, finishTime := range runningList {
			if finishTime < nextTime {
				nextTime = finishTime
			}
		}

		// Fast-forward to nextTime
		currentTime = nextTime

		// Complete jobs that finished at nextTime
		var finishedNow []string
		for id, finishTime := range runningList {
			if finishTime == currentTime { // exact match due to assignments
				finishedNow = append(finishedNow, id)
			}
		}

		// Sort to ensure deterministic processing of dependents
		sort.Strings(finishedNow)

		for _, id := range finishedNow {
			delete(runningList, id)
			workersAvailable++
			lastFinishedJob = id
			totalProcessed++

			// Decrement inDegree of dependents
			for _, depID := range dependents[id] {
				depJob := simJobs[depID]
				depJob.InDegree--
				if depJob.InDegree == 0 {
					readyQueue = append(readyQueue, depJob)
				}
			}
		}
	}

	deadlocks := len(simJobs) - totalProcessed

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(25)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	warnStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196"))

	fmt.Fprintln(stdout, titleStyle.Render("Simulation Report"))
	fmt.Fprintln(stdout, "")

	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Estimated Total Time:"), valueStyle.Render(time.Duration(currentTime*1e6).Round(time.Second).String()))
	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Jobs Processed:"), valueStyle.Render(fmt.Sprintf("%d / %d", totalProcessed, len(simJobs))))

	if lastFinishedJob != "" {
		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Final Bottleneck Job:"), valueStyle.Render(lastFinishedJob))
	}

	if deadlocks > 0 {
		fmt.Fprintln(stdout, "")
		fmt.Fprintf(stdout, "%s\n", warnStyle.Render(fmt.Sprintf("WARNING: %d jobs could not be processed due to unresolved/circular dependencies!", deadlocks)))
	}
	fmt.Fprintln(stdout, "")
}
