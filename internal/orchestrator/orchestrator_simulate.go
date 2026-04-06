package orchestrator

import (
	"log/slog"
	"math"
	"sort"
	"time"
)

type SimulationReport struct {
	EstimatedTotalTimeMs float64 `json:"estimated_total_time_ms"`
	EstimatedTotalCost   float64 `json:"estimated_total_cost"`
	EstimatedTotalTokens float64 `json:"estimated_total_tokens"`
	JobsProcessed        int     `json:"jobs_processed"`
	TotalJobs            int     `json:"total_jobs"`
	FinalBottleneckJob   string  `json:"final_bottleneck_job"`
	Deadlocks            int     `json:"deadlocks"`
}

func (o *Orchestrator) SimulatePipeline(items []WorkItem, logger *slog.Logger) SimulationReport {
	o.mu.RLock()
	maxWorkers := o.MaxConcurrentJobs

	// Fetch all jobs for tag averages calculation
	completedJobs := make([]JobInfo, len(o.completedJobs))
	copy(completedJobs, o.completedJobs)
	o.mu.RUnlock()

	if maxWorkers <= 0 {
		maxWorkers = math.MaxInt32
	}

	// Calculate tag averages and historical duration averages from completed jobs
	tagAverages := make(map[string]float64)
	tagCostAverages := make(map[string]float64)
	tagTokenAverages := make(map[string]float64)
	tagCounts := make(map[string]int)

	summaryAverages := make(map[string]float64)
	summaryCostAverages := make(map[string]float64)
	summaryTokenAverages := make(map[string]float64)
	summaryCounts := make(map[string]int)

	var globalSum float64
	var globalCostSum float64
	var globalTokenSum float64
	var globalCount int

	for _, job := range completedJobs {
		if !job.StartTime.IsZero() && !job.EndTime.IsZero() {
			dur := float64(job.EndTime.Sub(job.StartTime).Milliseconds())
			cost := job.Metrics["total_cost"]
			tokens := job.Metrics["total_tokens"]
			if dur > 0 {
				globalSum += dur
				globalCostSum += cost
				globalTokenSum += tokens
				globalCount++
				for _, tag := range job.WorkItem.Tags {
					tagAverages[tag] += dur
					tagCostAverages[tag] += cost
					tagTokenAverages[tag] += tokens
					tagCounts[tag]++
				}
				// Also group by Summary for exact job match prediction
				summaryAverages[job.WorkItem.Summary] += dur
				summaryCostAverages[job.WorkItem.Summary] += cost
				summaryTokenAverages[job.WorkItem.Summary] += tokens
				summaryCounts[job.WorkItem.Summary]++
			}
		}
	}

	for tag, total := range tagAverages {
		tagAverages[tag] = total / float64(tagCounts[tag])
		tagCostAverages[tag] = tagCostAverages[tag] / float64(tagCounts[tag])
		tagTokenAverages[tag] = tagTokenAverages[tag] / float64(tagCounts[tag])
	}

	for summary, total := range summaryAverages {
		summaryAverages[summary] = total / float64(summaryCounts[summary])
		summaryCostAverages[summary] = summaryCostAverages[summary] / float64(summaryCounts[summary])
		summaryTokenAverages[summary] = summaryTokenAverages[summary] / float64(summaryCounts[summary])
	}

	globalMean := 60000.0 // Default 60s
	globalCostMean := 0.0
	globalTokenMean := 0.0
	if globalCount > 0 {
		globalMean = globalSum / float64(globalCount)
		globalCostMean = globalCostSum / float64(globalCount)
		globalTokenMean = globalTokenSum / float64(globalCount)
	}

	var jobs []JobInfo
	for _, item := range items {
		job := JobInfo{
			ID:       item.ID,
			WorkItem: item,
			Status:   "Pending",
			Metrics:  make(map[string]float64),
		}

		// If we have an exact summary match in history, prioritize that over tag averages
		if avgDur, ok := summaryAverages[item.Summary]; ok {
			job.Metrics["estimated_duration_ns"] = avgDur * 1e6
		}
		if avgCost, ok := summaryCostAverages[item.Summary]; ok {
			job.Metrics["total_cost"] = avgCost
		}
		if avgTokens, ok := summaryTokenAverages[item.Summary]; ok {
			job.Metrics["total_tokens"] = avgTokens
		}

		jobs = append(jobs, job)
	}

	return runSimulation(jobs, maxWorkers, tagAverages, tagCostAverages, tagTokenAverages, globalMean, globalCostMean, globalTokenMean)
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
	tagCostAverages := make(map[string]float64)
	tagTokenAverages := make(map[string]float64)
	tagCounts := make(map[string]int)
	var globalSum float64
	var globalCostSum float64
	var globalTokenSum float64
	var globalCount int

	for _, job := range completedJobs {
		if !job.StartTime.IsZero() && !job.EndTime.IsZero() {
			dur := float64(job.EndTime.Sub(job.StartTime).Milliseconds())
			cost := job.Metrics["total_cost"]
			tokens := job.Metrics["total_tokens"]
			if dur > 0 {
				globalSum += dur
				globalCostSum += cost
				globalTokenSum += tokens
				globalCount++
				for _, tag := range job.WorkItem.Tags {
					tagAverages[tag] += dur
					tagCostAverages[tag] += cost
					tagTokenAverages[tag] += tokens
					tagCounts[tag]++
				}
			}
		}
	}

	for tag, total := range tagAverages {
		tagAverages[tag] = total / float64(tagCounts[tag])
		tagCostAverages[tag] = tagCostAverages[tag] / float64(tagCounts[tag])
		tagTokenAverages[tag] = tagTokenAverages[tag] / float64(tagCounts[tag])
	}

	globalMean := 60000.0 // Default 60s
	globalCostMean := 0.0
	globalTokenMean := 0.0
	if globalCount > 0 {
		globalMean = globalSum / float64(globalCount)
		globalCostMean = globalCostSum / float64(globalCount)
		globalTokenMean = globalTokenSum / float64(globalCount)
	}

	// Combine active and pending jobs for simulation
	var jobs []JobInfo
	jobs = append(jobs, activeJobs...)
	jobs = append(jobs, pendingJobs...)

	return runSimulation(jobs, maxWorkers, tagAverages, tagCostAverages, tagTokenAverages, globalMean, globalCostMean, globalTokenMean)
}

func runSimulation(jobs []JobInfo, maxWorkers int, tagAverages map[string]float64, tagCostAverages map[string]float64, tagTokenAverages map[string]float64, globalMean float64, globalCostMean float64, globalTokenMean float64) SimulationReport {
	if len(jobs) == 0 {
		return SimulationReport{}
	}

	type SimJob struct {
		ID        string
		Priority  int
		Deps      []string
		InDegree  int
		Duration  float64
		Cost      float64
		Tokens    float64
		Remaining float64
	}

	simJobs := make(map[string]*SimJob)
	dependents := make(map[string][]string)

	var totalEstimatedCost float64
	var totalEstimatedTokens float64

	for _, job := range jobs {
		estDur := globalMean
		estCost := globalCostMean
		estTokens := globalTokenMean

		// Check if we have an explicitly injected historical average
		if job.Metrics != nil {
			if d, ok := job.Metrics["estimated_duration_ns"]; ok {
				estDur = d / 1e6 // Convert ns to ms
			} else if len(job.WorkItem.Tags) > 0 {
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

			if c, ok := job.Metrics["total_cost"]; ok {
				estCost = c
			} else if len(job.WorkItem.Tags) > 0 {
				var sumCost float64
				var count int
				for _, tag := range job.WorkItem.Tags {
					if c, ok := tagCostAverages[tag]; ok {
						sumCost += c
						count++
					}
				}
				if count > 0 {
					estCost = sumCost / float64(count)
				}
			}

			if t, ok := job.Metrics["total_tokens"]; ok {
				estTokens = t
			} else if len(job.WorkItem.Tags) > 0 {
				var sumTokens float64
				var count int
				for _, tag := range job.WorkItem.Tags {
					if t, ok := tagTokenAverages[tag]; ok {
						sumTokens += t
						count++
					}
				}
				if count > 0 {
					estTokens = sumTokens / float64(count)
				}
			}
		} else if len(job.WorkItem.Tags) > 0 {
			var sumDur float64
			var sumCost float64
			var sumTokens float64
			var count int
			for _, tag := range job.WorkItem.Tags {
				if d, ok := tagAverages[tag]; ok {
					sumDur += d
					sumCost += tagCostAverages[tag]
					sumTokens += tagTokenAverages[tag]
					count++
				}
			}
			if count > 0 {
				estDur = sumDur / float64(count)
				estCost = sumCost / float64(count)
				estTokens = sumTokens / float64(count)
			}
		}

		sj := &SimJob{
			ID:       job.ID,
			Priority: job.WorkItem.Priority,
			Duration: estDur,
			Cost:     estCost,
			Tokens:   estTokens,
			Deps:     job.WorkItem.DependsOn,
		}

		if job.Status == "Running" || job.Status == "Spawning" {
			elapsed := float64(time.Since(job.StartTime).Milliseconds())
			rem := estDur - elapsed
			if rem <= 0 {
				rem = 1000 // At least 1s remaining
			}
			sj.Remaining = rem

			// Adjust remaining cost/tokens proportionally to remaining time
			if estDur > 0 {
				ratio := rem / estDur
				if ratio < 0 {
					ratio = 0
				} else if ratio > 1 {
					ratio = 1
				}
				totalEstimatedCost += estCost * ratio
				totalEstimatedTokens += estTokens * ratio
			}
		} else {
			sj.Remaining = estDur
			totalEstimatedCost += estCost
			totalEstimatedTokens += estTokens
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
		EstimatedTotalCost:   totalEstimatedCost,
		EstimatedTotalTokens: totalEstimatedTokens,
		JobsProcessed:        totalProcessed,
		TotalJobs:            len(simJobs),
		FinalBottleneckJob:   lastFinishedJob,
		Deadlocks:            len(simJobs) - totalProcessed,
	}
}
