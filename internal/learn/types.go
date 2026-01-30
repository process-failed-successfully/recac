package learn

import "time"

type Flashcard struct {
	ID          string    `json:"id"`
	Question    string    `json:"question"`
	Answer      string    `json:"answer"` // Correct answer text
	Options     []string  `json:"options,omitempty"` // For multiple choice
	Explanation string    `json:"explanation,omitempty"`
	FilePath    string    `json:"file_path"`

	// SM-2 State
	NextReview  time.Time `json:"next_review"`
	Interval    int       `json:"interval"`     // Days
	EaseFactor  float64   `json:"ease_factor"`
	Repetitions int       `json:"repetitions"`
}

type Deck struct {
	Cards []Flashcard `json:"cards"`
}
