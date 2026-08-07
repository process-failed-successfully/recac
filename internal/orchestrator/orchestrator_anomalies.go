package orchestrator

import (
	"log/slog"
	"math"
	"time"
)

// AnomalyReport represents a detected anomaly in a job's execution.
type AnomalyReport struct {
	JobID       string        `json:"job_id"`
	Model       string        `json:"model"`
	Duration    time.Duration `json:"duration,omitempty"`
	DurationDev float64       `json:"duration_dev,omitempty"`
	Cost        float64       `json:"cost,omitempty"`
	CostDev     float64       `json:"cost_dev,omitempty"`
	Status      string        `json:"status"`
}

type modelStats struct {
	durations []float64
	costs     []float64
}

// AnalyzeAnomalies calculates the mean and standard deviation of duration and cost per model
// and identifies jobs that deviate by more than 2 standard deviations.
func (o *Orchestrator) AnalyzeAnomalies(logger *slog.Logger) ([]AnomalyReport, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var anomalies []AnomalyReport
	statsByModel := make(map[string]*modelStats)
	allJobs := make([]JobInfo, 0, len(o.activeJobs)+len(o.completedJobs))

	// Collect active jobs that have completed tasks (or we can just skip active jobs to avoid incomplete data)
	// Actually, it's better to just look at completed jobs (which include Failed, Completed, Canceled)
	// to get accurate final durations and costs.
	for _, job := range o.completedJobs {
		allJobs = append(allJobs, job)
	}

	if len(allJobs) == 0 {
		return anomalies, nil
	}

	// 1. Gather all durations and costs per model
	for _, job := range allJobs {
		if job.WorkItem.AgentModel == "" {
			continue // skip jobs without model
		}

		duration := time.Since(job.StartTime).Seconds()
		if !job.EndTime.IsZero() {
			duration = job.EndTime.Sub(job.StartTime).Seconds()
		}

		cost := job.Metrics["total_cost"]

		stats, ok := statsByModel[job.WorkItem.AgentModel]
		if !ok {
			stats = &modelStats{}
			statsByModel[job.WorkItem.AgentModel] = stats
		}
		stats.durations = append(stats.durations, duration)
		stats.costs = append(stats.costs, cost)
	}

	// 2. Calculate Mean and StdDev per model
	type meanStd struct {
		durMean   float64
		durStdDev float64
		costMean  float64
		costStdDev float64
	}

	calcStats := make(map[string]meanStd)
	for model, stats := range statsByModel {
		var durSum, costSum float64
		for _, d := range stats.durations {
			durSum += d
		}
		for _, c := range stats.costs {
			costSum += c
		}

		n := float64(len(stats.durations))
		if n == 0 {
			continue
		}
		durMean := durSum / n
		costMean := costSum / n

		var durSqSum, costSqSum float64
		for _, d := range stats.durations {
			durSqSum += (d - durMean) * (d - durMean)
		}
		for _, c := range stats.costs {
			costSqSum += (c - costMean) * (c - costMean)
		}

		durStdDev := 0.0
		costStdDev := 0.0
		if n > 1 {
			durStdDev = math.Sqrt(durSqSum / n) // population standard deviation
			costStdDev = math.Sqrt(costSqSum / n)
		}

		calcStats[model] = meanStd{
			durMean:   durMean,
			durStdDev: durStdDev,
			costMean:  costMean,
			costStdDev: costStdDev,
		}
	}

	// 3. Find anomalies (> 2 StdDevs)
	for _, job := range allJobs {
		model := job.WorkItem.AgentModel
		if model == "" {
			continue
		}

		st, ok := calcStats[model]
		if !ok {
			continue
		}

		duration := time.Since(job.StartTime).Seconds()
		if !job.EndTime.IsZero() {
			duration = job.EndTime.Sub(job.StartTime).Seconds()
		}
		cost := job.Metrics["total_cost"]

		durDev := 0.0
		costDev := 0.0

		if st.durStdDev > 0 {
			durDev = (duration - st.durMean) / st.durStdDev
		}
		if st.costStdDev > 0 {
			costDev = (cost - st.costMean) / st.costStdDev
		}

		// Check if it's an anomaly (positive deviation > 2)
		isAnomaly := false
		var anomalyReport AnomalyReport

		if durDev > 2.0 {
			isAnomaly = true
			anomalyReport.Duration = time.Duration(duration * float64(time.Second))
			anomalyReport.DurationDev = durDev
		}

		if costDev > 2.0 {
			isAnomaly = true
			anomalyReport.Cost = cost
			anomalyReport.CostDev = costDev
		}

		if isAnomaly {
			anomalyReport.JobID = job.ID
			anomalyReport.Model = model
			anomalyReport.Status = job.Status
			anomalies = append(anomalies, anomalyReport)
		}
	}

	return anomalies, nil
}
