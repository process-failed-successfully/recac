package main

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(costCmd)
	costCmd.Flags().Int("limit", 10, "Limit the number of sessions displayed in the 'Top Sessions by Cost' list")
	costCmd.Flags().Bool("watch", false, "Enter live-monitoring mode")
}

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Analyze and display session costs",
	Long:  `Provides a detailed breakdown of costs associated with all sessions, grouped by model and sorted by expense.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
		}

		watch, _ := cmd.Flags().GetBool("watch")
		if watch {
			model := newTopModel(sm)
			p := tea.NewProgram(model, tea.WithAltScreen())

			if err := p.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
				return err
			}
			return nil
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("could not list sessions: %w", err)
		}

		if len(sessions) == 0 {
			cmd.Println("No sessions found to analyze.")
			return nil
		}

		limit, _ := cmd.Flags().GetInt("limit")

		analysis, err := analyzeSessionCosts(sessions, limit)
		if err != nil {
			return fmt.Errorf("error analyzing session costs: %w", err)
		}

		displayCostAnalysis(cmd, analysis)

		return nil
	},
}

// CostAnalysis holds the aggregated cost data.
type CostAnalysis struct {
	TotalCost         float64
	TotalTokens       int
	Models            []*ModelCost
	TopSessionsByCost []*SessionCost
}

// ModelCost aggregates cost and token data for a specific model.
type ModelCost struct {
	Name              string
	TotalTokens       int
	TotalPromptTokens int
	TotalResponseTokens int
	TotalCost         float64
}

// SessionCost holds cost data for a single session.
type SessionCost struct {
	Name      string
	Model     string
	Cost      float64
	TotalTokens int
}

func analyzeSessionCosts(sessions []*runner.SessionState, limit int) (*CostAnalysis, error) {
	modelCosts := make(map[string]*ModelCost)
	var sessionCosts []*SessionCost
	var totalCost float64
	var totalTokens int

	for _, session := range sessions {
		if session.AgentStateFile == "" {
			continue
		}

		agentState, err := loadAgentState(session.AgentStateFile)
		// Skip sessions where agent state can't be loaded (e.g., still running, no agent yet)
		if err != nil {
			continue
		}

		// Ensure model name is not empty
		if agentState.Model == "" {
			agentState.Model = "unknown"
		}

		cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)

		// Aggregate total stats
		totalCost += cost
		totalTokens += agentState.TokenUsage.TotalTokens

		// Aggregate by model
		if _, ok := modelCosts[agentState.Model]; !ok {
			modelCosts[agentState.Model] = &ModelCost{Name: agentState.Model}
		}
		model := modelCosts[agentState.Model]
		model.TotalTokens += agentState.TokenUsage.TotalTokens
		model.TotalPromptTokens += agentState.TokenUsage.TotalPromptTokens
		model.TotalResponseTokens += agentState.TokenUsage.TotalResponseTokens
		model.TotalCost += cost

		// Store session cost for sorting later
		sessionCosts = append(sessionCosts, &SessionCost{
			Name:      session.Name,
			Model:     agentState.Model,
			Cost:      cost,
			TotalTokens: agentState.TokenUsage.TotalTokens,
		})
	}

	// Sort models by cost (high to low)
	sortedModels := make([]*ModelCost, 0, len(modelCosts))
	for _, mc := range modelCosts {
		sortedModels = append(sortedModels, mc)
	}
	sort.Slice(sortedModels, func(i, j int) bool {
		return sortedModels[i].TotalCost > sortedModels[j].TotalCost
	})

	// Sort sessions by cost (high to low)
	sort.Slice(sessionCosts, func(i, j int) bool {
		return sessionCosts[i].Cost > sessionCosts[j].Cost
	})

	// Apply limit to top sessions
	if limit > 0 && len(sessionCosts) > limit {
		sessionCosts = sessionCosts[:limit]
	}

	return &CostAnalysis{
		TotalCost:         totalCost,
		TotalTokens:       totalTokens,
		Models:            sortedModels,
		TopSessionsByCost: sessionCosts,
	}, nil
}

func displayCostAnalysis(cmd *cobra.Command, analysis *CostAnalysis) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)

	// --- Cost By Model ---
	fmt.Fprintln(w, "COST BY MODEL")
	fmt.Fprintln(w, "-------------")
	fmt.Fprintln(w, "MODEL\tCOST\tTOTAL TOKENS\tPROMPT TOKENS\tRESPONSE TOKENS")
	for _, model := range analysis.Models {
		fmt.Fprintf(w, "%s\t$%.4f\t%d\t%d\t%d\n",
			model.Name, model.TotalCost, model.TotalTokens, model.TotalPromptTokens, model.TotalResponseTokens)
	}
	fmt.Fprintln(w)

	// --- Top Sessions by Cost ---
	fmt.Fprintln(w, "TOP SESSIONS BY COST")
	fmt.Fprintln(w, "--------------------")
	fmt.Fprintln(w, "SESSION NAME\tMODEL\tCOST\tTOTAL TOKENS")
	for _, session := range analysis.TopSessionsByCost {
		fmt.Fprintf(w, "%s\t%s\t$%.6f\t%d\n",
			session.Name, session.Model, session.Cost, session.TotalTokens)
	}
	fmt.Fprintln(w)

	// --- Totals ---
	fmt.Fprintln(w, "TOTALS")
	fmt.Fprintln(w, "------")
	fmt.Fprintf(w, "Total Estimated Cost:\t$%.4f\n", analysis.TotalCost)
	fmt.Fprintf(w, "Total Tokens:\t%d\n", analysis.TotalTokens)

	w.Flush()
}

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type topModel struct {
	table    table.Model
	sm       ISessionManager
	sessions []*runner.SessionState
	err      error
}

type updateMsg []*runner.SessionState
type errMsg struct{ err error }

func newTopModel(sm ISessionManager) *topModel {
	columns := []table.Column{
		{Title: "NAME", Width: 20},
		{Title: "STATUS", Width: 10},
		{Title: "STARTED", Width: 20},
		{Title: "DURATION", Width: 15},
		{Title: "PROMPT", Width: 10},
		{Title: "COMPLETION", Width: 10},
		{Title: "TOTAL", Width: 10},
		{Title: "COST", Width: 15},
	}

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return &topModel{
		table: t,
		sm:    sm,
	}
}

func (m *topModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), fetchSessions(m.sm))
}

func (m *topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tickMsg:
		return m, tea.Batch(tickCmd(), fetchSessions(m.sm))
	case updateMsg:
		m.sessions = msg
		m.updateTable()
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *topModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	return baseStyle.Render(m.table.View()) + "\n"
}

func (m *topModel) updateTable() {
	rows := make([]table.Row, len(m.sessions))
	for i, session := range m.sessions {
		started := session.StartTime.Format("2006-01-02 15:04:05")
		var duration string
		if session.EndTime.IsZero() {
			duration = time.Since(session.StartTime).Round(time.Second).String()
		} else {
			duration = session.EndTime.Sub(session.StartTime).Round(time.Second).String()
		}

		agentState, err := loadAgentState(session.AgentStateFile)
		if err != nil {
			rows[i] = table.Row{
				session.Name,
				session.Status,
				started,
				duration,
				"N/A",
				"N/A",
				"N/A",
				"N/A",
			}
		} else {
			cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
			rows[i] = table.Row{
				session.Name,
				session.Status,
				started,
				duration,
				fmt.Sprintf("%d", agentState.TokenUsage.TotalPromptTokens),
				fmt.Sprintf("%d", agentState.TokenUsage.TotalResponseTokens),
				fmt.Sprintf("%d", agentState.TokenUsage.TotalTokens),
				fmt.Sprintf("$%.6f", cost),
			}
		}
	}
	m.table.SetRows(rows)
}


type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchSessions(sm ISessionManager) tea.Cmd {
	return func() tea.Msg {
		sessions, err := sm.ListSessions()
		if err != nil {
			return errMsg{err}
		}
		return updateMsg(sessions)
	}
}
