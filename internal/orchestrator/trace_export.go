package orchestrator

import (
	"encoding/json"
	"time"
)

// TraceEvent represents a single event in the Chrome Trace Event format.
type TraceEvent struct {
	Name      string                 `json:"name"`
	Cat       string                 `json:"cat"`
	Ph        string                 `json:"ph"`
	Ts        int64                  `json:"ts"`
	Dur       int64                  `json:"dur,omitempty"`
	Pid       int                    `json:"pid"`
	Tid       int                    `json:"tid"`
	Args      map[string]interface{} `json:"args,omitempty"`
	Cname     string                 `json:"cname,omitempty"`
}

// ExportTraceToJSON converts a list of JobInfo into Chrome Trace Event JSON format.
func ExportTraceToJSON(jobs []JobInfo) ([]byte, error) {
	var events []TraceEvent

	for _, job := range jobs {
		if job.StartTime.IsZero() {
			continue // Skip jobs that haven't started
		}

		endTime := job.EndTime
		if endTime.IsZero() {
			endTime = time.Now()
		}

		durUs := endTime.Sub(job.StartTime).Microseconds()
		if durUs < 0 {
			durUs = 0
		}

		args := map[string]interface{}{
			"id":       job.ID,
			"summary":  job.Summary,
			"status":   job.Status,
		}

		if job.Error != "" {
			args["error"] = job.Error
		}
		if job.WorkItem.AgentProvider != "" {
			args["provider"] = job.WorkItem.AgentProvider
		}
		if job.WorkItem.AgentModel != "" {
			args["model"] = job.WorkItem.AgentModel
		}

		// Color mapping based on status
		cname := "thread_state_runnable" // default (often blue/greenish)
		switch job.Status {
		case "Completed", "Success":
			cname = "good"
		case "Failed", "Error":
			cname = "terrible"
		case "Canceled":
			cname = "thread_state_unknown"
		case "Pending", "Pending Approval", "Pending Dependencies":
			cname = "thread_state_sleeping"
		}

		event := TraceEvent{
			Name:  job.ID,
			Cat:   "job",
			Ph:    "X", // Complete event (start + duration)
			Ts:    job.StartTime.UnixMicro(),
			Dur:   durUs,
			Pid:   1, // Arbitrary PID for all jobs
			Tid:   1, // Group all into one thread, or could use go routine ID if known, but 1 is fine for a flat timeline
			Args:  args,
			Cname: cname,
		}

		events = append(events, event)
	}

	return json.MarshalIndent(events, "", "  ")
}
