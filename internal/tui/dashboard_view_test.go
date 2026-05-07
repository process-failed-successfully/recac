package tui

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestDashboardView(t *testing.T) {
	m := NewDashboardModel("http://localhost")

	states := []viewState{
		viewMain,
		viewLogs,
		viewDepsInput,
		viewEnvInput,
		viewRenameInput,
		viewMaxRetriesInput,
		viewTagsInput,
		viewAgentInput,
		viewDeletePendingGroupInput,
		viewSearchJobsInput,
		viewSearchLogsContextInput,
		viewSearchLogsInput,
		viewConfirmation,
		viewAnalyzeFailures,
		viewAnalyzeDurations,
		viewAnalyzeReliability,
		viewAnalyzeCosts,
		viewAnalyzeAgents,
		viewSummary,
		viewAnalytics,
		viewCriticalPath,
		viewTree,
		viewDetails,
		viewSimulate,
		viewCompare,
		viewBlockers,
		viewDependents,
		viewDeletePendingTagInput,
		viewDeletePendingMatchInput,
		viewPauseGroupInput,
		viewResumeGroupInput,
		viewExplain,
	}

	for _, state := range states {
		m.viewState = state
		view := m.View()
		assert.NotEmpty(t, view, "View for state %d should not be empty", state)
	}
}
