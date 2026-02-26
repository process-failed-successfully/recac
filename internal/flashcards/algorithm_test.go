package flashcards

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReview(t *testing.T) {
	// 1. Test New Card (First Review - Good)
	card := NewFlashcard("Q", "A", "", "test")
	assert.Equal(t, StateNew, card.State)
	assert.Equal(t, 0, card.Interval)
	assert.Equal(t, 0, card.Repetitions)

	updated := Review(card, RatingGood)
	assert.Equal(t, StateReview, updated.State)
	assert.Equal(t, 1, updated.Interval)
	assert.Equal(t, 1, updated.Repetitions)
	assert.True(t, updated.DueDate.After(time.Now()))

	// 2. Test Second Review (Good)
	updated = Review(updated, RatingGood)
	assert.Equal(t, 6, updated.Interval)
	assert.Equal(t, 2, updated.Repetitions)

	// 3. Test Third Review (Good)
	// Interval = 6 * 2.5 = 15
	updated = Review(updated, RatingGood)
	assert.Equal(t, 15, updated.Interval)
	assert.Equal(t, 3, updated.Repetitions)

	// 4. Test Failure (Again)
	failed := Review(updated, RatingAgain)
	assert.Equal(t, StateRelearning, failed.State)
	assert.Equal(t, 1, failed.Interval)
	assert.Equal(t, 0, failed.Repetitions)
}

func TestEaseUpdate(t *testing.T) {
	card := NewFlashcard("Q", "A", "", "test")
	// Default EF = 2.5

	// Easy (5) -> EF increases
	// 2.5 + (0.1 - (0) * ...) = 2.6
	updated := Review(card, RatingEasy)
	assert.InDelta(t, 2.6, updated.EaseFactor, 0.01)

	// Hard (3) -> EF decreases
	// 2.6 + (0.1 - (2)*(0.08 + 0.04))
	// 2.6 + (0.1 - 0.24) = 2.46
	updated = Review(updated, RatingHard)
	assert.InDelta(t, 2.46, updated.EaseFactor, 0.01)

	// Fail (0) -> EF decreases but not below 1.3
	// Strict SM-2 logic might differ, but our implementation updates it.
	// 2.46 + (0.1 - (5)*(0.08 + 0.1)) = 2.46 + (0.1 - 0.9) = 1.66
	updated = Review(updated, RatingAgain)
	assert.Less(t, updated.EaseFactor, 2.46)
}
