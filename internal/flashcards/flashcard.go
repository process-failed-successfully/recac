package flashcards

import (
	"time"

	"github.com/google/uuid"
)

// State represents the learning state of a card
type State int

const (
	StateNew State = iota
	StateLearning
	StateReview
	StateRelearning
)

// Flashcard represents a single flashcard in the spaced repetition system.
type Flashcard struct {
	ID          string    `json:"id"`
	Question    string    `json:"question"`
	Answer      string    `json:"answer"`
	Context     string    `json:"context,omitempty"`      // Optional source file or context
	Topic       string    `json:"topic,omitempty"`        // Optional topic tag
	EaseFactor  float64   `json:"ease_factor"`            // SM-2 Ease Factor (default 2.5)
	Interval    int       `json:"interval"`               // Current interval in days
	Repetitions int       `json:"repetitions"`            // Number of successful repetitions
	DueDate     time.Time `json:"due_date"`               // Next review date
	State       State     `json:"state"`                  // Current learning state
	LastReview  time.Time `json:"last_review,omitempty"`  // Last review timestamp
}

// NewFlashcard creates a new flashcard with default SM-2 settings.
func NewFlashcard(question, answer, context, topic string) Flashcard {
	return Flashcard{
		ID:          uuid.New().String(),
		Question:    question,
		Answer:      answer,
		Context:     context,
		Topic:       topic,
		EaseFactor:  2.5,
		Interval:    0,
		Repetitions: 0,
		DueDate:     time.Now(), // Due immediately
		State:       StateNew,
	}
}
