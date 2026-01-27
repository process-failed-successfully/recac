package gamify

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GitClient defines the interface needed for gamification analysis.
type GitClient interface {
	Log(directory string, args ...string) ([]string, error)
}

// Player represents a contributor.
type Player struct {
	Name        string
	Commits     int
	LinesAdded  int
	LinesDel    int
	BugFixes    int
	DocEdits    int
	TestEdits   int
	XP          int
	Badges      []string
	Languages   map[string]int
	LastCommit  time.Time
}

// Leaderboard holds all players.
type Leaderboard struct {
	Players []*Player
}

// AnalyzeRepo analyzes the git history and returns a leaderboard.
func AnalyzeRepo(client GitClient, dir string) (*Leaderboard, error) {
	// Request numstat and formatted header
	// Format: COMMIT|Hash|Author|Date|Message
	// Followed by numstat lines
	args := []string{
		"--numstat",
		"--date=iso",
		"--pretty=format:COMMIT|%h|%an|%ad|%s",
	}

	lines, err := client.Log(dir, args...)
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	players := make(map[string]*Player)
	var currentPlayer *Player

	// Regex for bug fixes
	bugFixRe := regexp.MustCompile(`(?i)(fix|resolve|close|bug|issue)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "COMMIT|") {
			parts := strings.SplitN(line, "|", 5)
			if len(parts) < 5 {
				continue
			}
			author := parts[2]
			dateStr := parts[3]
			msg := parts[4]

			// Parse date (Git ISO format: 2006-01-02 15:04:05 -0700)
			// Go's time.Parse might need adjustment or use specific layout
			// Git ISO output: "2023-10-25 14:30:00 +0000"
			parsedDate, _ := time.Parse("2006-01-02 15:04:05 -0700", dateStr)

			if _, exists := players[author]; !exists {
				players[author] = &Player{
					Name:      author,
					Languages: make(map[string]int),
				}
			}
			currentPlayer = players[author]
			currentPlayer.Commits++
			if parsedDate.After(currentPlayer.LastCommit) {
				currentPlayer.LastCommit = parsedDate
			}

			// Base XP
			currentPlayer.XP += 10

			// Bug Fix XP
			if bugFixRe.MatchString(msg) {
				currentPlayer.BugFixes++
				currentPlayer.XP += 20
			}

			// Night Owl Check (between 00:00 and 05:00 local time of the committer)
			// Note: parsedDate includes offset, so .Hour() returns hour in that offset.
			hour := parsedDate.Hour()
			if hour >= 0 && hour < 5 {
				// We'll award badge logic later, but maybe track "NightCommits" if we want
				// For now, let's just use a flag or check commits
			}

		} else {
			// Numstat line: "added	deleted	path"
			// 10	5	src/main.go
			// Note: git log --numstat uses tabs as delimiters
			parts := strings.Split(line, "\t")
			if len(parts) < 3 {
				// Fallback to Fields if split by tab fails (e.g. copied/pasted logs with spaces)
				parts = strings.Fields(line)
				if len(parts) < 3 {
					continue
				}
			}

			// Binary files have "-"
			added, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			deleted, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			path := strings.TrimSpace(parts[2])

			if currentPlayer != nil {
				currentPlayer.LinesAdded += added
				currentPlayer.LinesDel += deleted

				// Track Languages
				if idx := strings.LastIndex(path, "."); idx != -1 {
					ext := path[idx:] // e.g., ".go"
					if len(ext) > 1 {
						// Simple normalization
						lang := strings.ToLower(ext[1:]) // "go"
						currentPlayer.Languages[lang] += added
					}
				} else if strings.HasSuffix(strings.ToLower(path), "dockerfile") {
					currentPlayer.Languages["docker"] += added
				} else if strings.HasSuffix(strings.ToLower(path), "makefile") {
					currentPlayer.Languages["make"] += added
				}

				// XP for Lines (capped at 100 per commit to prevent massive generated files gaming)
				linesXP := added
				if linesXP > 100 {
					linesXP = 100
				}
				currentPlayer.XP += linesXP / 10 // 1 XP per 10 lines

				// File Type Bonuses
				if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".txt") {
					currentPlayer.DocEdits++
					currentPlayer.XP += 5
				}
				if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.js") {
					currentPlayer.TestEdits++
					currentPlayer.XP += 10
				}
			}
		}
	}

	// Calculate Badges and Sort
	var leaderboard Leaderboard
	for _, p := range players {
		awardBadges(p)
		leaderboard.Players = append(leaderboard.Players, p)
	}

	sort.Slice(leaderboard.Players, func(i, j int) bool {
		return leaderboard.Players[i].XP > leaderboard.Players[j].XP
	})

	return &leaderboard, nil
}

func awardBadges(p *Player) {
	badges := make([]string, 0)

	if p.Commits >= 50 {
		badges = append(badges, "🏅 Marathoner")
	} else if p.Commits >= 10 {
		badges = append(badges, "🏃 Runner")
	}

	if p.BugFixes >= 5 {
		badges = append(badges, "🐛 Hunter")
	}

	if p.DocEdits >= 10 {
		badges = append(badges, "📜 Scholar")
	}

	if p.TestEdits >= 10 {
		badges = append(badges, "🧪 Scientist")
	}

	if p.XP > 1000 {
		badges = append(badges, "🧙 Wizard")
	}

	// Night Owl logic requires tracking night commits, which we didn't store in struct.
	// We can add it if needed, but for MVP this is fine.

	p.Badges = badges
}
