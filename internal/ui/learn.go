package ui

import (
	"fmt"
	"recac/internal/learn"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LearnState int

const (
	LearnStateQuestion LearnState = iota
	LearnStateAnswer
	LearnStateDone
)

type LearnModel struct {
	Cards      []learn.Flashcard
	CurrentIdx int
	State      LearnState
	WindowSize tea.WindowSizeMsg

	// UpdatedCards to return
	UpdatedCards []learn.Flashcard

	// Styles
	width int
	height int
}

func NewLearnModel(cards []learn.Flashcard) LearnModel {
	return LearnModel{
		Cards: cards,
		State: LearnStateQuestion,
	}
}

func (m LearnModel) Init() tea.Cmd {
	return nil
}

func (m LearnModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.State = LearnStateDone // Ensure we signal done or just quit
			return m, tea.Quit
		}

		if m.State == LearnStateQuestion {
			if msg.String() == " " || msg.String() == "enter" {
				m.State = LearnStateAnswer
				return m, nil
			}
		} else if m.State == LearnStateAnswer {
			switch msg.String() {
			case "1", "2", "3", "4":
				// Rate
				rating := 0
				switch msg.String() {
				case "1": rating = 1
				case "2": rating = 2
				case "3": rating = 3
				case "4": rating = 4
				}

				// Update Card
				currentCard := m.Cards[m.CurrentIdx]
				updatedCard := learn.CalculateReview(currentCard, rating)
				m.UpdatedCards = append(m.UpdatedCards, updatedCard)

				// Next
				m.CurrentIdx++
				if m.CurrentIdx >= len(m.Cards) {
					m.State = LearnStateDone
					return m, tea.Quit
				} else {
					m.State = LearnStateQuestion
				}
			}
		}

	case tea.WindowSizeMsg:
		m.WindowSize = msg
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m LearnModel) View() string {
	if m.State == LearnStateDone {
		return "Session Complete! Press q to exit."
	}

	if len(m.Cards) == 0 {
		return "No cards due! Great job."
	}

	if m.CurrentIdx >= len(m.Cards) {
		return "Session Complete!"
	}

	card := m.Cards[m.CurrentIdx]

	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("Card %d/%d\n\n", m.CurrentIdx+1, len(m.Cards)))

	// Question
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Question:"))
	b.WriteString("\n")
	b.WriteString(card.Question)
	b.WriteString("\n\n")

	if len(card.Options) > 0 {
		for _, opt := range card.Options {
			b.WriteString(fmt.Sprintf("- %s\n", opt))
		}
		b.WriteString("\n")
	}

	if m.State == LearnStateAnswer {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("green")).Render("Answer:"))
		b.WriteString("\n")
		b.WriteString(card.Answer)
		b.WriteString("\n\n")

		if card.Explanation != "" {
			b.WriteString(lipgloss.NewStyle().Italic(true).Render("Explanation:"))
			b.WriteString("\n")
			b.WriteString(card.Explanation)
			b.WriteString("\n\n")
		}

		b.WriteString("Rate recall quality:\n")
		b.WriteString("[1] Again (Fail)  [2] Hard  [3] Good  [4] Easy")
	} else {
		b.WriteString("\n[Space] Show Answer")
	}

	return b.String()
}
