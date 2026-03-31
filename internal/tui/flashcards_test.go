package tui

import (
	"bytes"
	"recac/internal/flashcards"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// MockStore implements flashcards.Store for testing
type MockStore struct {
	cards   map[string]flashcards.Flashcard
	updates []flashcards.Flashcard
}

func NewMockStore() *MockStore {
	return &MockStore{
		cards:   make(map[string]flashcards.Flashcard),
		updates: []flashcards.Flashcard{},
	}
}

func (s *MockStore) Load() error                { return nil }
func (s *MockStore) Save() error                { return nil }
func (s *MockStore) Add(c flashcards.Flashcard) { s.cards[c.ID] = c }
func (s *MockStore) List() []flashcards.Flashcard {
	var list []flashcards.Flashcard
	for _, c := range s.cards {
		list = append(list, c)
	}
	return list
}
func (s *MockStore) GetDue() []flashcards.Flashcard {
	var due []flashcards.Flashcard
	now := time.Now()
	for _, c := range s.cards {
		if c.DueDate.Before(now) || c.State == flashcards.StateNew || c.State == flashcards.StateRelearning {
			due = append(due, c)
		}
	}
	return due
}
func (s *MockStore) Update(c flashcards.Flashcard) {
	s.cards[c.ID] = c
	s.updates = append(s.updates, c)
}
func (s *MockStore) Delete(id string) { delete(s.cards, id) }

func TestFlashcardsModel_Init(t *testing.T) {
	store := NewMockStore()
	model := initialFlashcardsModel(store, []flashcards.Flashcard{})
	cmd := model.Init()
	assert.Nil(t, cmd)
}

func TestFlashcardsModel_Update_QuestionToAnswer(t *testing.T) {
	store := NewMockStore()
	card := flashcards.NewFlashcard("Q1", "A1", "ctx", "topic")
	queue := []flashcards.Flashcard{card}
	model := initialFlashcardsModel(store, queue)
	model.ready = true

	// Initial state: Question
	assert.Equal(t, StateQuestion, model.state)

	// Send space key
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	m := updatedModel.(FlashcardsModel)

	assert.Nil(t, cmd)
	assert.Equal(t, StateAnswer, m.state)
}

func TestFlashcardsModel_Update_AnswerToNext(t *testing.T) {
	store := NewMockStore()
	card1 := flashcards.NewFlashcard("Q1", "A1", "ctx", "topic")
	card2 := flashcards.NewFlashcard("Q2", "A2", "ctx", "topic")
	queue := []flashcards.Flashcard{card1, card2}
	model := initialFlashcardsModel(store, queue)
	model.ready = true
	model.state = StateAnswer // Start at answer phase

	// Rate as Good (3)
	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m := updatedModel.(FlashcardsModel)

	assert.Nil(t, cmd)
	assert.Equal(t, StateQuestion, m.state)
	assert.Equal(t, 1, m.current) // Moved to next card
	assert.Equal(t, 1, m.reviewed)
	assert.Equal(t, 1, m.learned) // Good (4) >= Hard (3) -> Learned++

	// Check store update
	assert.Len(t, store.updates, 1)
	assert.Equal(t, "Q1", store.updates[0].Question)
}

func TestFlashcardsModel_Update_Finish(t *testing.T) {
	store := NewMockStore()
	card := flashcards.NewFlashcard("Q1", "A1", "ctx", "topic")
	queue := []flashcards.Flashcard{card}
	model := initialFlashcardsModel(store, queue)
	model.ready = true
	model.state = StateAnswer

	// Rate as Easy (4)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m := updatedModel.(FlashcardsModel)

	assert.Equal(t, StateFinished, m.state)
	assert.Equal(t, 1, m.reviewed)
	assert.Equal(t, 1, m.learned) // Easy >= Hard -> Learned++
}

func TestFlashcardsModel_View(t *testing.T) {
	store := NewMockStore()
	card := flashcards.NewFlashcard("TestQuestion", "TestAnswer", "TestContext", "TestTopic")
	queue := []flashcards.Flashcard{card}
	model := initialFlashcardsModel(store, queue)
	model.ready = true
	model.width = 80
	model.height = 24

	// View Question
	view := model.View()
	assert.Contains(t, view, "TestQuestion")
	assert.NotContains(t, view, "TestAnswer")
	assert.Contains(t, view, "Flashcard 1/1")

	// Switch to Answer
	model.state = StateAnswer
	view = model.View()
	assert.Contains(t, view, "TestQuestion")
	assert.Contains(t, view, "TestAnswer")
	assert.Contains(t, view, "TestContext")
	assert.Contains(t, view, "[1] Again")

	// Finished
	model.state = StateFinished
	view = model.View()
	assert.Contains(t, view, "Session Complete!")
}

func TestStartFlashcardsSession_NoCards(t *testing.T) {
	store := NewMockStore()
	// No cards added
	err := StartFlashcardsSession(store)
	assert.NoError(t, err) // Should just print and return nil
}

func TestFlashcardsModel_Update_QuitKeys(t *testing.T) {
	store := NewMockStore()
	card := flashcards.NewFlashcard("Q1", "A1", "ctx", "topic")
	queue := []flashcards.Flashcard{card}
	model := initialFlashcardsModel(store, queue)
	model.ready = true

	// Test q
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.NotNil(t, cmd)

	// Test ctrl+c
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.NotNil(t, cmd)

	// Test esc
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.NotNil(t, cmd)
}

func TestFlashcardsModel_Update_Rate1(t *testing.T) {
	store := NewMockStore()
	card := flashcards.NewFlashcard("Q1", "A1", "ctx", "topic")
	queue := []flashcards.Flashcard{card}
	model := initialFlashcardsModel(store, queue)
	model.ready = true
	model.state = StateAnswer

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m := updatedModel.(FlashcardsModel)
	assert.Nil(t, cmd)
	assert.Equal(t, StateFinished, m.state)
	assert.Equal(t, 1, m.reviewed)
	assert.Equal(t, 0, m.learned) // Rate 1 < Rate 2 (Hard)
}

func TestFlashcardsModel_Update_Rate2(t *testing.T) {
	store := NewMockStore()
	card := flashcards.NewFlashcard("Q1", "A1", "ctx", "topic")
	queue := []flashcards.Flashcard{card}
	model := initialFlashcardsModel(store, queue)
	model.ready = true
	model.state = StateAnswer

	updatedModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m := updatedModel.(FlashcardsModel)
	assert.Nil(t, cmd)
	assert.Equal(t, StateFinished, m.state)
	assert.Equal(t, 1, m.reviewed)
	assert.Equal(t, 1, m.learned) // Rate 2 == Hard
}

func TestFlashcardsModel_Update_WindowSize(t *testing.T) {
	store := NewMockStore()
	card := flashcards.NewFlashcard("Q1", "A1", "ctx", "topic")
	queue := []flashcards.Flashcard{card}
	model := initialFlashcardsModel(store, queue)

	updatedModel, cmd := model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m := updatedModel.(FlashcardsModel)
	assert.Nil(t, cmd)
	assert.True(t, m.ready)
	assert.Equal(t, 100, m.width)
	assert.Equal(t, 50, m.height)
}

func TestFlashcardsModel_Update_TableDriven(t *testing.T) {
	card := flashcards.NewFlashcard("Q1", "A1", "ctx", "topic")
	queue := []flashcards.Flashcard{card}

	tests := []struct {
		name          string
		initialState  FlashcardsState
		msg           tea.Msg
		expectedState FlashcardsState
		expectedRev   int
		expectedLearn int
		expectCmd     bool
	}{
		{
			name:          "quit with q",
			initialState:  StateQuestion,
			msg:           tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
			expectedState: StateQuestion,
			expectCmd:     true,
		},
		{
			name:          "quit with ctrl+c",
			initialState:  StateQuestion,
			msg:           tea.KeyMsg{Type: tea.KeyCtrlC},
			expectedState: StateQuestion,
			expectCmd:     true,
		},
		{
			name:          "quit with esc",
			initialState:  StateQuestion,
			msg:           tea.KeyMsg{Type: tea.KeyEsc},
			expectedState: StateQuestion,
			expectCmd:     true,
		},
		{
			name:          "rate 1 (Again)",
			initialState:  StateAnswer,
			msg:           tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}},
			expectedState: StateFinished,
			expectedRev:   1,
			expectedLearn: 0,
		},
		{
			name:          "rate 2 (Hard)",
			initialState:  StateAnswer,
			msg:           tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}},
			expectedState: StateFinished,
			expectedRev:   1,
			expectedLearn: 1,
		},
		{
			name:          "window resize",
			initialState:  StateQuestion,
			msg:           tea.WindowSizeMsg{Width: 100, Height: 50},
			expectedState: StateQuestion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMockStore()
			model := initialFlashcardsModel(store, queue)
			model.ready = true
			model.state = tt.initialState

			updatedModel, cmd := model.Update(tt.msg)
			m := updatedModel.(FlashcardsModel)

			if tt.expectCmd {
				assert.NotNil(t, cmd)
			} else {
				assert.Nil(t, cmd)
				assert.Equal(t, tt.expectedState, m.state)
				if _, ok := tt.msg.(tea.KeyMsg); ok {
					assert.Equal(t, tt.expectedRev, m.reviewed)
					assert.Equal(t, tt.expectedLearn, m.learned)
				}
				if _, ok := tt.msg.(tea.WindowSizeMsg); ok {
					assert.True(t, m.ready)
					assert.Equal(t, 100, m.width)
					assert.Equal(t, 50, m.height)
				}
			}
		})
	}
}

func TestStartFlashcardsSession_Run(t *testing.T) {
	store := NewMockStore()
	store.Add(flashcards.NewFlashcard("Q", "A", "ctx", "topic"))

	var in bytes.Buffer
	in.WriteString("q")
	var out bytes.Buffer

	err := StartFlashcardsSession(store, tea.WithInput(&in), tea.WithOutput(&out))
	assert.NoError(t, err)
}
