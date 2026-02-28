package tui

import (
	"fmt"
	"recac/internal/flashcards"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FlashcardsState int

const (
	StateQuestion FlashcardsState = iota
	StateAnswer
	StateFinished
)

type FlashcardsModel struct {
	store    flashcards.Store
	queue    []flashcards.Flashcard
	current  int
	state    FlashcardsState
	viewport viewport.Model
	width    int
	height   int
	ready    bool

	// Stats for this session
	reviewed int
	learned  int
}

func StartFlashcardsSession(store flashcards.Store) error {
	queue := store.GetDue()
	if len(queue) == 0 {
		fmt.Println("No cards due for review! Great job.")
		return nil
	}

	p := tea.NewProgram(initialFlashcardsModel(store, queue), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func initialFlashcardsModel(store flashcards.Store, queue []flashcards.Flashcard) FlashcardsModel {
	return FlashcardsModel{
		store: store,
		queue: queue,
		state: StateQuestion,
	}
}

func (m FlashcardsModel) Init() tea.Cmd {
	return nil
}

func (m FlashcardsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		}

		if m.state == StateFinished {
			return m, tea.Quit
		}

		if m.state == StateQuestion {
			if msg.String() == " " || msg.String() == "enter" {
				m.state = StateAnswer
				return m, nil
			}
		} else if m.state == StateAnswer {
			var rating flashcards.Rating
			rated := false

			switch msg.String() {
			case "1":
				rating = flashcards.RatingAgain
				rated = true
			case "2":
				rating = flashcards.RatingHard
				rated = true
			case "3":
				rating = flashcards.RatingGood
				rated = true
			case "4":
				rating = flashcards.RatingEasy
				rated = true
			case " ": // Space also triggers Good by default? No, let's force a choice.
			}

			if rated {
				// Update card
				card := m.queue[m.current]
				updatedCard := flashcards.Review(card, rating)
				m.store.Update(updatedCard)
				m.store.Save() // Save immediately

				m.reviewed++
				if rating >= flashcards.RatingHard {
					m.learned++
				}

				// Move to next
				m.current++
				if m.current >= len(m.queue) {
					m.state = StateFinished
				} else {
					m.state = StateQuestion
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height
		}
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m FlashcardsModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.state == StateFinished {
		return m.finishedView()
	}

	if len(m.queue) == 0 {
		return "No cards."
	}

	card := m.queue[m.current]

	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render(
		fmt.Sprintf("Flashcard %d/%d | Topic: %s", m.current+1, len(m.queue), card.Topic))

	var content string

	// Question
	qStyle := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Width(m.width - 4)
	content += qStyle.Render(fmt.Sprintf("Q: %s", card.Question)) + "\n\n"

	if m.state == StateAnswer {
		// Answer
		aStyle := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("205")).Width(m.width - 4)
		content += aStyle.Render(fmt.Sprintf("A: %s", card.Answer)) + "\n\n"

		// Context if available
		if card.Context != "" {
			content += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf("Context: %s", card.Context)) + "\n\n"
		}

		// Controls
		controls := lipgloss.NewStyle().Bold(true).Render("Rate this card:") + "\n" +
			"[1] Again (Fail)  [2] Hard  [3] Good  [4] Easy"
		content += controls
	} else {
		// Instructions
		content += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Press Space or Enter to flip...")
	}

	// Center usage
	return fmt.Sprintf("%s\n\n%s", header, content)
}

func (m FlashcardsModel) finishedView() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("Session Complete!")
	stats := fmt.Sprintf("Reviewed: %d\nMastered: %d", m.reviewed, m.learned)
	return fmt.Sprintf("%s\n\n%s\n\nPress q to quit.", title, stats)
}
