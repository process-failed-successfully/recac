package learn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Card struct {
	ID           string    `json:"id"`
	Question     string    `json:"question"`
	Answer       string    `json:"answer"`
	ContextFile  string    `json:"context_file"`
	Created      time.Time `json:"created"`
	NextReview   time.Time `json:"next_review"`
	Interval     float64   `json:"interval"` // in days
	Repetitions  int       `json:"repetitions"`
	EaseFactor   float64   `json:"ease_factor"`
	LastReviewed time.Time `json:"last_reviewed"`
}

type Deck struct {
	Cards []Card `json:"cards"`
}

func (d *Deck) Add(c Card) {
	d.Cards = append(d.Cards, c)
}

func (d *Deck) GetDueCards() []Card {
	var due []Card
	now := time.Now()
	for _, c := range d.Cards {
		if c.NextReview.Before(now) {
			due = append(due, c)
		}
	}
	return due
}

func (d *Deck) UpdateCard(updated Card) {
	for i, c := range d.Cards {
		if c.ID == updated.ID {
			d.Cards[i] = updated
			return
		}
	}
}

func GetDeckPath() (string, error) {
	// Try current directory .recac
	if _, err := os.Stat(".recac"); err == nil {
		return ".recac/learn.json", nil
	}
	// Try home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".recac", "learn.json"), nil
}

func LoadDeck() (*Deck, error) {
	path, err := GetDeckPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Deck{Cards: []Card{}}, nil
		}
		return nil, err
	}

	var deck Deck
	if err := json.Unmarshal(data, &deck); err != nil {
		return nil, err
	}
	return &deck, nil
}

func SaveDeck(deck *Deck) error {
	path, err := GetDeckPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(deck, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
