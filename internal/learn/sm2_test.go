package learn

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateNextReview(t *testing.T) {
	// 1. Test New Card (First Review)
	card := Card{
		ID:         "test",
		EaseFactor: 0,
		Repetitions: 0,
		Interval:    0,
	}

	// 1a. Fail (Rating 1)
	next := CalculateNextReview(card, 1)
	assert.Equal(t, 0, next.Repetitions)
	assert.Equal(t, float64(1), next.Interval)
	assert.Equal(t, 2.5, next.EaseFactor) // Default initialized
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), next.NextReview, 5*time.Second)

	// 1b. Good (Rating 3)
	next = CalculateNextReview(card, 3)
	assert.Equal(t, 1, next.Repetitions)
	assert.Equal(t, float64(1), next.Interval)
	assert.Equal(t, 2.5, next.EaseFactor) // Default initialized, no change for rating 4 (SM2: 4 is Quality 4. Wait. Rating 3 -> Quality 4)
	// EF' = 2.5 + (0.1 - (5-4)*(0.08+(5-4)*0.02)) = 2.5 + (0.1 - 1*0.1) = 2.5
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), next.NextReview, 5*time.Second)

	// 2. Second Repetition
	card.Repetitions = 1
	card.Interval = 1
	card.EaseFactor = 2.5

	// 2a. Good (Rating 3 -> Quality 4)
	next = CalculateNextReview(card, 3)
	assert.Equal(t, 2, next.Repetitions)
	assert.Equal(t, float64(6), next.Interval)
	assert.Equal(t, 2.5, next.EaseFactor)

	// 3. Third Repetition
	card.Repetitions = 2
	card.Interval = 6
	card.EaseFactor = 2.5

	// 3a. Good (Rating 3 -> Quality 4)
	next = CalculateNextReview(card, 3)
	assert.Equal(t, 3, next.Repetitions)
	// Interval = 6 * 2.5 = 15
	assert.Equal(t, float64(15), next.Interval)

	// 4. Test Ease Factor Change
	// Rating 4 (Easy) -> Quality 5
	// EF' = 2.5 + (0.1 - (5-5)*(...)) = 2.6
	next = CalculateNextReview(card, 4)
	assert.InDelta(t, 2.6, next.EaseFactor, 0.001)

	// Rating 2 (Hard) -> Quality 3
	// EF' = 2.5 + (0.1 - (2)*(0.08 + 2*0.02)) = 2.5 + (0.1 - 2*0.12) = 2.36
	next = CalculateNextReview(card, 2)
	assert.InDelta(t, 2.36, next.EaseFactor, 0.001)
}
