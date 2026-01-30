package learn

import (
	"math"
	"time"
)

// CalculateReview updates the card's schedule based on the user rating (1-4).
// 1: Again (Fail)
// 2: Hard (Pass)
// 3: Good (Pass)
// 4: Easy (Pass)
func CalculateReview(card Flashcard, rating int) Flashcard {
	// Initialize if fresh
	if card.EaseFactor < 1.3 {
		card.EaseFactor = 2.5
	}

	if rating == 1 {
		// Fail: Reset progress
		card.Repetitions = 0
		card.Interval = 1
	} else {
		// Pass
		// Map 1-4 to standard SM-2 0-5 scale roughly
		// 1 -> 0 (Fail)
		// 2 -> 3 (Hard)
		// 3 -> 4 (Good)
		// 4 -> 5 (Easy)

		var q float64
		switch rating {
		case 1: q = 0
		case 2: q = 3
		case 3: q = 4
		case 4: q = 5
		default: q = 3
		}

		// EF' = EF + (0.1 - (5-q)*(0.08+(5-q)*0.02))
		card.EaseFactor = card.EaseFactor + (0.1 - (5-q)*(0.08+(5-q)*0.02))
		if card.EaseFactor < 1.3 {
			card.EaseFactor = 1.3
		}

		card.Repetitions++

		if card.Repetitions == 1 {
			card.Interval = 1
		} else if card.Repetitions == 2 {
			card.Interval = 6
		} else {
			card.Interval = int(math.Round(float64(card.Interval) * card.EaseFactor))
		}
	}

	card.NextReview = time.Now().AddDate(0, 0, card.Interval)
	return card
}
