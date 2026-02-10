package doctor

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FormatReport generates a human-readable TUI report.
func FormatReport(results []CheckResult) string {
	var sb strings.Builder

	sb.WriteString("\n🏥 RECAC Doctor Report\n")
	sb.WriteString("======================\n\n")

	for _, r := range results {
		icon := "✅"

		if r.Skipped {
			icon = "⏭️"
		} else if !r.Passed {
			icon = "❌"
		}

		// Line: [ICON] Name ..... Message
		sb.WriteString(fmt.Sprintf("%s %-20s %s\n", icon, r.Name, r.Message))
		if !r.Passed && r.Error != nil {
			sb.WriteString(fmt.Sprintf("      Error: %v\n", r.Error))
		}
	}

	failures := 0
	for _, r := range results {
		if !r.Passed && !r.Skipped {
			failures++
		}
	}

	sb.WriteString("\n")
	if failures > 0 {
		sb.WriteString(fmt.Sprintf("Result: %d checks failed. See above for details.\n", failures))
	} else {
		sb.WriteString("Result: All systems operational. 🚀\n")
	}

	return sb.String()
}

// FormatJSON generates a JSON report.
func FormatJSON(results []CheckResult) string {
	type jsonResult struct {
		Name      string `json:"name"`
		Status    string `json:"status"`
		Message   string `json:"message"`
		Error     string `json:"error,omitempty"`
		Timestamp string `json:"timestamp"`
	}

	var jsonResults []jsonResult
	for _, r := range results {
		status := "PASS"
		if r.Skipped {
			status = "SKIP"
		} else if !r.Passed {
			status = "FAIL"
		}

		errMsg := ""
		if r.Error != nil {
			errMsg = r.Error.Error()
		}

		jsonResults = append(jsonResults, jsonResult{
			Name:      r.Name,
			Status:    status,
			Message:   r.Message,
			Error:     errMsg,
			Timestamp: r.Timestamp.Format(time.RFC3339),
		})
	}

	data, err := json.MarshalIndent(jsonResults, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal results: %v"}`, err)
	}
	return string(data)
}
