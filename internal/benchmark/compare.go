package benchmark

import "fmt"

type Comparison struct {
	Name           string
	NsPerOpDiff    float64 // Percentage change
	BytesPerOpDiff float64 // Percentage change
	AllocsPerOpDiff float64 // Percentage change
	Prev           Result
	Curr           Result
}

// Compare runs comparison between two results.
// It returns a list of comparisons for benchmarks present in both runs.
func Compare(prev, curr Run) []Comparison {
	prevMap := make(map[string]Result)
	for _, r := range prev.Results {
		prevMap[r.Name] = r
	}

	var comparisons []Comparison
	for _, c := range curr.Results {
		if p, ok := prevMap[c.Name]; ok {
			comp := Comparison{
				Name: c.Name,
				Prev: p,
				Curr: c,
			}

			if p.NsPerOp > 0 {
				comp.NsPerOpDiff = ((c.NsPerOp - p.NsPerOp) / p.NsPerOp) * 100
			}
			if p.BytesPerOp > 0 {
				comp.BytesPerOpDiff = float64(c.BytesPerOp-p.BytesPerOp) / float64(p.BytesPerOp) * 100
			}
			if p.AllocsPerOp > 0 {
				comp.AllocsPerOpDiff = float64(c.AllocsPerOp-p.AllocsPerOp) / float64(p.AllocsPerOp) * 100
			}

			comparisons = append(comparisons, comp)
		}
	}
	return comparisons
}

// FormatDiff returns a colored string for the diff
// Negative diff (faster) is green, Positive (slower) is red.
// This function returns plain text, color handling should be in the CLI part or use a library here.
// I'll stick to plain text logic or simple indicators here.
func (c Comparison) String() string {
	return fmt.Sprintf("%s: %.2f%% ns/op", c.Name, c.NsPerOpDiff)
}
