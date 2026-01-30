package ui

import (
	"fmt"
	"recac/internal/learn"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LearnModel struct {
	deck        *learn.Deck
	currentCard learn.Card
	queue       []learn.Card
	showAnswer  bool
	finished    bool
	help        help.Model
	keys        LearnKeyMap
	width       int
	height      int
}

type LearnKeyMap struct {
	ShowAnswer key.Binding
	RateAgain  key.Binding
	RateHard   key.Binding
	RateGood   key.Binding
	RateEasy   key.Binding
	Quit       key.Binding
}

var learnKeys = LearnKeyMap{
	ShowAnswer: key.NewBinding(
		key.WithKeys(" ", "enter"),
		key.WithHelp("space/enter", "show answer"),
	),
	RateAgain: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "again (fail)"),
	),
	RateHard: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "hard"),
	),
	RateGood: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "good"),
	),
	RateEasy: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "easy"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// ShortHelp returns keybindings to be shown in the mini help view.
func (k LearnKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.ShowAnswer, k.Quit}
}

// FullHelp returns keybindings for the expanded help view.
func (k LearnKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.ShowAnswer, k.RateAgain, k.RateHard},
		{k.RateGood, k.RateEasy, k.Quit},
	}
}

func NewLearnModel(deck *learn.Deck) LearnModel {
	due := deck.GetDueCards()
	m := LearnModel{
		deck:  deck,
		queue: due,
		help:  help.New(),
		keys:  learnKeys,
	}
	m.nextCard()
	return m
}

func (m *LearnModel) nextCard() {
	if len(m.queue) == 0 {
		m.finished = true
		return
	}
	m.currentCard = m.queue[0]
	m.queue = m.queue[1:]
	m.showAnswer = false
}

func (m LearnModel) Init() tea.Cmd {
	return nil
}

func (m LearnModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		if m.finished {
			return m, tea.Quit
		}

		if !m.showAnswer {
			if key.Matches(msg, m.keys.ShowAnswer) {
				m.showAnswer = true
			}
		} else {
			rating := 0
			if key.Matches(msg, m.keys.RateAgain) {
				rating = 1
			} else if key.Matches(msg, m.keys.RateHard) {
				rating = 2
			} else if key.Matches(msg, m.keys.RateGood) {
				rating = 3
			} else if key.Matches(msg, m.keys.RateEasy) {
				rating = 4
			}

			if rating > 0 {
				// Update card
				updated := learn.CalculateNextReview(m.currentCard, rating)
				m.deck.UpdateCard(updated)

				// Save deck immediately
				_ = learn.SaveDeck(m.deck)

				m.nextCard()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
	}

	return m, nil
}

func (m LearnModel) View() string {
	if m.finished {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("42")).
			Padding(2).
			Render("🎉 You have completed all due cards! Come back later.")
	}

	s := ""

	// Question Style
	questionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Width(60).
		Align(lipgloss.Center)

	answerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(1).
		Width(60)

	s += fmt.Sprintf("File: %s\n\n", m.currentCard.ContextFile)
	s += questionStyle.Render(m.currentCard.Question) + "\n\n"

	if m.showAnswer {
		s += answerStyle.Render(m.currentCard.Answer) + "\n\n"
		s += "Rate difficulty:\n"
		s += "[1] Again  [2] Hard  [3] Good  [4] Easy\n"
	} else {
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("(Press Space/Enter to show answer)") + "\n"
	}

	s += "\n" + m.help.View(m.keys)

	return lipgloss.NewStyle().Padding(2).Render(s)
}
