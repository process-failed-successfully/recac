package orchestrator

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type AgentPerformance struct {
	AgentProvider    string        `json:"agent_provider"`
	AgentModel       string        `json:"agent_model"`
	TotalJobs        int           `json:"total_jobs"`
	SuccessfulJobs   int           `json:"successful_jobs"`
	FailedJobs       int           `json:"failed_jobs"`
	SuccessRate      float64       `json:"success_rate"`
	AverageDuration  time.Duration `json:"average_duration"`
	AverageCost      float64       `json:"average_cost"`
	TotalCost        float64       `json:"total_cost"`
	TotalTokens      float64       `json:"total_tokens"`
}

type AgentStatsResponse struct {
	Agents []AgentPerformance `json:"agents"`
}

func handleAnalyzeAgents(orch *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		limit := 10
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l >= 0 {
				limit = l
			}
		}

		jobs := orch.GetCompletedJobs()

		agentMap := make(map[string]*AgentPerformance)

		for _, job := range jobs {
			provider := job.WorkItem.AgentProvider
			model := job.WorkItem.AgentModel

			if provider == "" {
				provider = "unknown"
			}
			if model == "" {
				model = "unknown"
			}

			key := provider + "/" + model

			stat, exists := agentMap[key]
			if !exists {
				stat = &AgentPerformance{
					AgentProvider: provider,
					AgentModel:    model,
				}
				agentMap[key] = stat
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
					// We'll calculate the sum here, and average later
					stat.AverageDuration += duration
				}
			}
		}

		var resp AgentStatsResponse
		for _, stat := range agentMap {
			if stat.TotalJobs > 0 {
				stat.SuccessRate = float64(stat.SuccessfulJobs) / float64(stat.TotalJobs)
				stat.AverageCost = stat.TotalCost / float64(stat.TotalJobs)
				stat.AverageDuration = time.Duration(int64(stat.AverageDuration) / int64(stat.TotalJobs))
			}
			resp.Agents = append(resp.Agents, *stat)
		}

		sort.Slice(resp.Agents, func(i, j int) bool {
			if resp.Agents[i].TotalJobs == resp.Agents[j].TotalJobs {
				return resp.Agents[i].AgentModel < resp.Agents[j].AgentModel
			}
			return resp.Agents[i].TotalJobs > resp.Agents[j].TotalJobs
		})

		if limit > 0 && len(resp.Agents) > limit {
			resp.Agents = resp.Agents[:limit]
		}
		if resp.Agents == nil {
			resp.Agents = []AgentPerformance{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logger.Error("Failed to encode agent stats", "error", err)
		}
	}
}
