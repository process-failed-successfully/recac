package learn

import (
	"math"
	"time"
)

// CalculateNextReview implements a modified SuperMemo-2 (SM-2) algorithm.
// rating (input from UI):
// 1 - Again (Fail) -> equivalent to SM-2 rating 0-2
// 2 - Hard         -> equivalent to SM-2 rating 3
// 3 - Good         -> equivalent to SM-2 rating 4
// 4 - Easy         -> equivalent to SM-2 rating 5
func CalculateNextReview(card Card, uiRating int) Card {
	// Initialize defaults if new card
	if card.EaseFactor == 0 {
		card.EaseFactor = 2.5
	}

	// Map UI rating to SM-2 quality (0-5)
	// We map 1 -> 1 (Fail)
	// 2 -> 3
	// 3 -> 4
	// 4 -> 5
	quality := 0
	switch uiRating {
	case 1:
		quality = 1
	case 2:
		quality = 3
	case 3:
		quality = 4
	case 4:
		quality = 5
	default:
		quality = 0 // Should not happen
	}

	if quality < 3 {
		// Failed, reset repetitions
		card.Repetitions = 0
		card.Interval = 1
	} else {
		// Passed
		if card.Repetitions == 0 {
			card.Interval = 1
		} else if card.Repetitions == 1 {
			card.Interval = 6
		} else {
			card.Interval = math.Round(card.Interval * card.EaseFactor)
		}
		card.Repetitions++

		// Update Ease Factor only on success
		// EF' = EF + (0.1 - (5-q) * (0.08 + (5-q) * 0.02))
		q := float64(quality)
		newEF := card.EaseFactor + (0.1 - (5.0-q)*(0.08+(5.0-q)*0.02))
		if newEF < 1.3 {
			newEF = 1.3
		}
		card.EaseFactor = newEF
	}

	card.LastReviewed = time.Now()
	// Next review is Now + Interval days
	card.NextReview = card.LastReviewed.Add(time.Duration(card.Interval*24) * time.Hour)

	return card
}
