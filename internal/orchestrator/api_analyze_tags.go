package orchestrator

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type TagPerformance struct {
	Tag             string        `json:"tag"`
	TotalJobs       int           `json:"total_jobs"`
	SuccessfulJobs  int           `json:"successful_jobs"`
	FailedJobs      int           `json:"failed_jobs"`
	SuccessRate     float64       `json:"success_rate"`
	AverageDuration time.Duration `json:"average_duration"`
	AverageCost     float64       `json:"average_cost"`
	TotalCost       float64       `json:"total_cost"`
	TotalTokens     float64       `json:"total_tokens"`
}

type TagStatsResponse struct {
	Tags []TagPerformance `json:"tags"`
}

func handleAnalyzeTags(orch *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		limit := 10
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l >= 0 {
				limit = l
			}
		}

		jobs := orch.GetCompletedJobs()

		tagMap := make(map[string]*TagPerformance)

		for _, job := range jobs {
			for _, tag := range job.WorkItem.Tags {
				if tag == "" {
					continue
				}

				stat, exists := tagMap[tag]
				if !exists {
					stat = &TagPerformance{
						Tag: tag,
					}
					tagMap[tag] = stat
				}

				stat.TotalJobs++
				if job.Status == "Completed" {
					stat.SuccessfulJobs++
				} else {
					stat.FailedJobs++
				}

				if job.Metrics != nil {
					if cost, ok := job.Metrics["cost_usd"]; ok {
						stat.TotalCost += cost
					}
					if tokens, ok := job.Metrics["tokens_total"]; ok {
						stat.TotalTokens += tokens
					} else if prompt, ok := job.Metrics["tokens_prompt"]; ok {
						completion := job.Metrics["tokens_completion"]
						stat.TotalTokens += prompt + completion
					}
				}
				if !job.EndTime.IsZero() && !job.StartTime.IsZero() {
					duration := job.EndTime.Sub(job.StartTime)
					if duration > 0 {
						stat.AverageDuration += duration
					}
				}
			}
		}

		var resp TagStatsResponse
		for _, stat := range tagMap {
			if stat.TotalJobs > 0 {
				stat.SuccessRate = float64(stat.SuccessfulJobs) / float64(stat.TotalJobs)
				stat.AverageCost = stat.TotalCost / float64(stat.TotalJobs)
				stat.AverageDuration = time.Duration(int64(stat.AverageDuration) / int64(stat.TotalJobs))
			}
			resp.Tags = append(resp.Tags, *stat)
		}

		sort.Slice(resp.Tags, func(i, j int) bool {
			if resp.Tags[i].TotalJobs == resp.Tags[j].TotalJobs {
				return resp.Tags[i].Tag < resp.Tags[j].Tag
			}
			return resp.Tags[i].TotalJobs > resp.Tags[j].TotalJobs
		})

		if limit > 0 && len(resp.Tags) > limit {
			resp.Tags = resp.Tags[:limit]
		}
		if resp.Tags == nil {
			resp.Tags = []TagPerformance{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logger.Error("Failed to encode tag stats", "error", err)
		}
	}
}
