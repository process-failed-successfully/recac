package learn

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"recac/internal/utils"
)

type AgentClient interface {
	Send(ctx context.Context, prompt string) (string, error)
}

func GenerateCards(ctx context.Context, ag AgentClient, root string, count int) ([]Flashcard, error) {
	// Find candidates
	files, err := findGoFiles(root)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no Go files found in %s", root)
	}

	var newCards []Flashcard

	// Limit to random selection if too many
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })

	if len(files) > count {
		files = files[:count]
	}

	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Truncate
		sContent := string(content)
		if len(sContent) > 3000 {
			sContent = sContent[:3000] + "\n...(truncated)"
		}

		prompt := fmt.Sprintf(`Create a single Flashcard for learning this code.
Focus on "Why this exists" or "How it works".
Return JSON:
{
  "question": "...",
  "answer": "The main concept...",
  "options": ["Wrong A", "Wrong B", "Correct Answer", "Wrong C"],
  "explanation": "..."
}
Code:
%s`, sContent)

		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			continue
		}

		clean := utils.CleanJSONBlock(resp)
		var card Flashcard
		if err := json.Unmarshal([]byte(clean), &card); err != nil {
			continue
		}

		// Initialize fields
		card.ID = fmt.Sprintf("%s-%d", filepath.Base(path), time.Now().UnixNano())
		card.FilePath = path
		card.EaseFactor = 2.5
		card.Interval = 0
		card.NextReview = time.Now() // Due immediately

		newCards = append(newCards, card)
	}

	return newCards, nil
}

func findGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
