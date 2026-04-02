package orchestrator

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
)

type CostStats struct {
	TotalCost             float64 `json:"total_cost"`
	TotalTokensPrompt     float64 `json:"total_tokens_prompt"`
	TotalTokensCompletion float64 `json:"total_tokens_completion"`
	TotalJobs             int     `json:"total_jobs"`
}

type CostByTag struct {
	Tag              string  `json:"tag"`
	Cost             float64 `json:"cost"`
	TokensPrompt     float64 `json:"tokens_prompt"`
	TokensCompletion float64 `json:"tokens_completion"`
	JobsCount        int     `json:"jobs_count"`
}

type CostByModel struct {
	Model            string  `json:"model"`
	Cost             float64 `json:"cost"`
	TokensPrompt     float64 `json:"tokens_prompt"`
	TokensCompletion float64 `json:"tokens_completion"`
	JobsCount        int     `json:"jobs_count"`
}

type CostStatsResponse struct {
	TotalStats       CostStats     `json:"total_stats"`
	TagStats         []CostByTag   `json:"tag_stats"`
	ModelStats       []CostByModel `json:"model_stats"`
	TopExpensiveJobs []JobInfo     `json:"top_expensive_jobs"`
}

func handleAnalyzeCosts(orch *Orchestrator, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		limit := 10
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l >= 0 {
				limit = l
			}
		}

		jobs := orch.GetCompletedJobs()

		var total CostStats
		tagMap := make(map[string]*CostByTag)
		modelMap := make(map[string]*CostByModel)
		var validJobs []JobInfo

		for _, job := range jobs {
			if job.Metrics == nil {
				continue
			}

			cost, hasCost := job.Metrics["cost_usd"]
			if !hasCost {
				continue
			}
			prompt := job.Metrics["tokens_prompt"]
			completion := job.Metrics["tokens_completion"]

			validJobs = append(validJobs, job)

			total.TotalCost += cost
			total.TotalTokensPrompt += prompt
			total.TotalTokensCompletion += completion
			total.TotalJobs++

			model := job.WorkItem.AgentModel
			if model == "" {
				model = "unknown"
			}

			mStat, exists := modelMap[model]
			if !exists {
				mStat = &CostByModel{Model: model}
				modelMap[model] = mStat
			}
			mStat.Cost += cost
			mStat.TokensPrompt += prompt
			mStat.TokensCompletion += completion
			mStat.JobsCount++

			for _, tag := range job.WorkItem.Tags {
				tStat, exists := tagMap[tag]
				if !exists {
					tStat = &CostByTag{Tag: tag}
					tagMap[tag] = tStat
				}
				tStat.Cost += cost
				tStat.TokensPrompt += prompt
				tStat.TokensCompletion += completion
				tStat.JobsCount++
			}
		}

		var resp CostStatsResponse
		resp.TotalStats = total

		for _, stat := range tagMap {
			resp.TagStats = append(resp.TagStats, *stat)
		}
		for _, stat := range modelMap {
			resp.ModelStats = append(resp.ModelStats, *stat)
		}

		sort.Slice(resp.TagStats, func(i, j int) bool {
			if resp.TagStats[i].Cost == resp.TagStats[j].Cost {
				return resp.TagStats[i].Tag < resp.TagStats[j].Tag
			}
			return resp.TagStats[i].Cost > resp.TagStats[j].Cost
		})
		if resp.TagStats == nil {
			resp.TagStats = []CostByTag{}
		}

		sort.Slice(resp.ModelStats, func(i, j int) bool {
			if resp.ModelStats[i].Cost == resp.ModelStats[j].Cost {
				return resp.ModelStats[i].Model < resp.ModelStats[j].Model
			}
			return resp.ModelStats[i].Cost > resp.ModelStats[j].Cost
		})
		if resp.ModelStats == nil {
			resp.ModelStats = []CostByModel{}
		}

		sort.Slice(validJobs, func(i, j int) bool {
			return validJobs[i].Metrics["cost_usd"] > validJobs[j].Metrics["cost_usd"]
		})

		resp.TopExpensiveJobs = validJobs
		if len(resp.TopExpensiveJobs) > limit {
			resp.TopExpensiveJobs = resp.TopExpensiveJobs[:limit]
		}
		if resp.TopExpensiveJobs == nil {
			resp.TopExpensiveJobs = []JobInfo{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logger.Error("Failed to encode cost stats", "error", err)
		}
	}
}
