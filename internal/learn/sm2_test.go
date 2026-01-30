package learn

import (
	"testing"
)

func TestCalculateReview(t *testing.T) {
	card := Flashcard{
		EaseFactor: 2.5,
		Interval:   0,
		Repetitions: 0,
	}

	// First Review: Good (3)
	card = CalculateReview(card, 3)
	if card.Interval != 1 {
		t.Errorf("Expected Interval 1, got %d", card.Interval)
	}
	if card.Repetitions != 1 {
		t.Errorf("Expected Repetitions 1, got %d", card.Repetitions)
	}

	// Check EF calculation:
	// q=4. 5-q=1. 0.08+1*0.02=0.1. 1*0.1=0.1. EF = 2.5 + (0.1 - 0.1) = 2.5
	if card.EaseFactor != 2.5 {
		t.Errorf("Expected EF 2.5, got %f", card.EaseFactor)
	}

	// Second Review: Good (3)
	card = CalculateReview(card, 3)
	if card.Interval != 6 {
		t.Errorf("Expected Interval 6, got %d", card.Interval)
	}

	// Third Review: Easy (4) -> q=5
	// 5-q=0 -> EF + 0.1
	oldEF := card.EaseFactor
	card = CalculateReview(card, 4)
	if card.EaseFactor <= oldEF {
		t.Errorf("Expected EF to increase, got %f vs %f", card.EaseFactor, oldEF)
	}
	// Interval = 6 * 2.6 = 15.6 -> 16
	if card.Interval < 15 {
		t.Errorf("Expected Interval > 15, got %d", card.Interval)
	}

	// Fail
	card = CalculateReview(card, 1)
	if card.Interval != 1 {
		t.Errorf("Expected Interval 1 after fail, got %d", card.Interval)
	}
	if card.Repetitions != 0 {
		t.Errorf("Expected Repetitions 0 after fail, got %d", card.Repetitions)
	}
}
