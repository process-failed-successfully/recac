package orchestrator

import (
	"log/slog"
	"math"
	"sort"
	"time"
)

type SimulationReport struct {
	EstimatedTotalTimeMs float64 `json:"estimated_total_time_ms"`
	JobsProcessed        int     `json:"jobs_processed"`
	TotalJobs            int     `json:"total_jobs"`
	FinalBottleneckJob   string  `json:"final_bottleneck_job"`
	Deadlocks            int     `json:"deadlocks"`
}

func (o *Orchestrator) Simulate(logger *slog.Logger) SimulationReport {
	o.mu.RLock()
	maxWorkers := o.MaxConcurrentJobs

	// Fetch all jobs
	completedJobs := make([]JobInfo, len(o.completedJobs))
	copy(completedJobs, o.completedJobs)

	var activeJobs []JobInfo
	for _, job := range o.activeJobs {
		activeJobs = append(activeJobs, job)
	}

	var pendingJobs []JobInfo
	for _, job := range o.pendingJobs {
		pendingJobs = append(pendingJobs, job)
	}
	o.mu.RUnlock()

	if maxWorkers <= 0 {
		maxWorkers = math.MaxInt32
	}

	// Calculate tag averages from completed jobs
	tagAverages := make(map[string]float64)
	tagCounts := make(map[string]int)
	var globalSum float64
	var globalCount int

	for _, job := range completedJobs {
		if !job.StartTime.IsZero() && !job.EndTime.IsZero() {
			dur := float64(job.EndTime.Sub(job.StartTime).Milliseconds())
			if dur > 0 {
				globalSum += dur
				globalCount++
				for _, tag := range job.WorkItem.Tags {
					tagAverages[tag] += dur
					tagCounts[tag]++
				}
			}
		}
	}

	for tag, total := range tagAverages {
		tagAverages[tag] = total / float64(tagCounts[tag])
	}

	globalMean := 60000.0 // Default 60s
	if globalCount > 0 {
		globalMean = globalSum / float64(globalCount)
	}

	// Combine active and pending jobs for simulation
	var jobs []JobInfo
	jobs = append(jobs, activeJobs...)
	jobs = append(jobs, pendingJobs...)

	if len(jobs) == 0 {
		return SimulationReport{}
	}

	type SimJob struct {
		ID        string
		Priority  int
		Deps      []string
		InDegree  int
		Duration  float64
		Remaining float64
	}

	simJobs := make(map[string]*SimJob)
	dependents := make(map[string][]string)

	for _, job := range jobs {
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
				rem = 1000 // At least 1s remaining
			}
			sj.Remaining = rem
		} else {
			sj.Remaining = estDur
		}

		simJobs[job.ID] = sj
	}

	for _, sj := range simJobs {
		for _, dep := range sj.Deps {
			if _, exists := simJobs[dep]; exists {
				sj.InDegree++
				dependents[dep] = append(dependents[dep], sj.ID)
			}
		}
	}

	workersAvailable := maxWorkers
	var readyQueue []*SimJob
	runningList := make(map[string]float64)

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
				return readyQueue[i].ID < readyQueue[j].ID
			}
			return readyQueue[i].Priority > readyQueue[j].Priority
		})
	}

	var currentTime float64 = 0
	var lastFinishedJob string
	var totalProcessed int

	for len(readyQueue) > 0 || len(runningList) > 0 {
		sortReadyQueue()
		for workersAvailable > 0 && len(readyQueue) > 0 {
			nextJob := readyQueue[0]
			readyQueue = readyQueue[1:]

			runningList[nextJob.ID] = currentTime + nextJob.Remaining
			workersAvailable--
		}

		if len(runningList) == 0 && len(readyQueue) == 0 {
			break
		}
		if len(runningList) == 0 && len(readyQueue) > 0 {
			if workersAvailable <= 0 {
				break
			}
		}

		nextTime := math.MaxFloat64
		for _, finishTime := range runningList {
			if finishTime < nextTime {
				nextTime = finishTime
			}
		}

		currentTime = nextTime

		var finishedNow []string
		for id, finishTime := range runningList {
			if finishTime == currentTime {
				finishedNow = append(finishedNow, id)
			}
		}

		sort.Strings(finishedNow)

		for _, id := range finishedNow {
			delete(runningList, id)
			workersAvailable++
			lastFinishedJob = id
			totalProcessed++

			for _, depID := range dependents[id] {
				depJob := simJobs[depID]
				depJob.InDegree--
				if depJob.InDegree == 0 {
					readyQueue = append(readyQueue, depJob)
				}
			}
		}
	}

	return SimulationReport{
		EstimatedTotalTimeMs: currentTime,
		JobsProcessed:        totalProcessed,
		TotalJobs:            len(simJobs),
		FinalBottleneckJob:   lastFinishedJob,
		Deadlocks:            len(simJobs) - totalProcessed,
	}
}
