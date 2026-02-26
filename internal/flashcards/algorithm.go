package flashcards

import (
	"math"
	"time"
)

// Rating represents the user's self-assessment of the answer.
type Rating int

const (
	RatingAgain Rating = 0 // Forgot / Incorrect
	RatingHard  Rating = 3 // Remembered with difficulty
	RatingGood  Rating = 4 // Remembered correctly
	RatingEasy  Rating = 5 // Remembered easily
)

// Review updates the flashcard's scheduling based on the user's rating (SM-2 Algorithm).
func Review(card Flashcard, rating Rating) Flashcard {
	card.LastReview = time.Now()

	if rating < RatingHard {
		// Incorrect answer: Reset repetitions and interval
		card.Repetitions = 0
		card.Interval = 1
		card.State = StateRelearning
		// Ease factor usually doesn't change on failure in strict SM-2,
		// but some variants decrease it. We'll keep it as is or implement standard SM-2 logic.
		// Standard SM-2 updates EF even on failure, but keeps it >= 1.3
		card.EaseFactor = updateEase(card.EaseFactor, float64(rating))
	} else {
		// Correct answer
		if card.Repetitions == 0 {
			card.Interval = 1
		} else if card.Repetitions == 1 {
			card.Interval = 6
		} else {
			card.Interval = int(math.Round(float64(card.Interval) * card.EaseFactor))
		}

		card.Repetitions++
		card.EaseFactor = updateEase(card.EaseFactor, float64(rating))
		card.State = StateReview
	}

	// Update Due Date
	card.DueDate = time.Now().AddDate(0, 0, card.Interval)

	return card
}

func updateEase(ease float64, quality float64) float64 {
	// EF' = EF + (0.1 - (5-q)*(0.08 + (5-q)*0.02))
	newEase := ease + (0.1 - (5.0-quality)*(0.08+(5.0-quality)*0.02))
	if newEase < 1.3 {
		return 1.3
	}
	return newEase
}
