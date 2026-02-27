package tui

import (
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

func (s *MockStore) Load() error           { return nil }
func (s *MockStore) Save() error           { return nil }
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
