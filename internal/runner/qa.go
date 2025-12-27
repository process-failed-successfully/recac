package runner

import (
	"fmt"
	"strings"
)

// QAReport summarizes the status of the feature list.
type QAReport struct {
	TotalFeatures   int
	PassedFeatures  int
	FailedFeatures  int
	FailedList      []Feature
	CompletionRatio float64
}

// RunQA analyzes the feature list and generates a report.
func RunQA(features []Feature) QAReport {
	report := QAReport{
		TotalFeatures: len(features),
		FailedList:    []Feature{},
	}

	for _, f := range features {
		if f.Passes {
			report.PassedFeatures++
		} else {
			report.FailedFeatures++
			report.FailedList = append(report.FailedList, f)
		}
	}

	if report.TotalFeatures > 0 {
		report.CompletionRatio = float64(report.PassedFeatures) / float64(report.TotalFeatures)
	}

	return report
}

// GenerateQASummary creates a human-readable string of the report.
func (r QAReport) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("QA Report: %d/%d features passing (%.1f%%)\n", r.PassedFeatures, r.TotalFeatures, r.CompletionRatio*100))
	
	if r.FailedFeatures > 0 {
		sb.WriteString("\nFailed Features:\n")
		for _, f := range r.FailedList {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", f.Category, f.Description))
		}
	} else {
		sb.WriteString("\nAll systems operational.\n")
	}
	
	return sb.String()
}
