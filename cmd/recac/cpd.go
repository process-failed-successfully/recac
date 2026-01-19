package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type Duplication struct {
	LineCount int        `json:"line_count"`
	Locations []Location `json:"locations"`
}

type Location struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type MatchPair struct {
	FileA  string
	StartA int
	EndA   int
	FileB  string
	StartB int
	EndB   int
}

type cpdOptions struct {
	minLines int
	ignore   []string
	json     bool
	fail     bool
}

func newCPDCmd() *cobra.Command {
	opts := cpdOptions{}
	cmd := &cobra.Command{
		Use:   "cpd [path]",
		Short: "Detect copy-pasted code",
		Long:  `Scans the codebase for duplicated code blocks (copy-paste detection). uses a sliding window algorithm to find identical sequences of lines.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}

			// Clean up ignore flags
			var cleanIgnore []string
			for _, p := range opts.ignore {
				if p != "[]" {
					cleanIgnore = append(cleanIgnore, p)
				}
			}
			opts.ignore = cleanIgnore

			duplicates, err := runCPD(root, opts.minLines, opts.ignore)
			if err != nil {
				return err
			}

			if opts.json {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(duplicates)
			}

			if len(duplicates) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No duplicates found. Great job!")
				return nil
			}

			printCPDTable(cmd, duplicates)

			if opts.fail {
				return fmt.Errorf("found %d duplicated blocks", len(duplicates))
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&opts.minLines, "min-lines", 10, "Minimum lines to match")
	cmd.Flags().StringSliceVar(&opts.ignore, "ignore", []string{}, "Ignore patterns (glob)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&opts.fail, "fail", false, "Exit with error code if duplicates found")

	return cmd
}

func init() {
	rootCmd.AddCommand(newCPDCmd())
}

func runCPD(root string, minLines int, ignorePatterns []string) ([]Duplication, error) {
	// 1. Collect all lines from all files
	fileLines := make(map[string][]string) // file -> lines

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			for _, p := range []string{"vendor", "node_modules", "dist", "build"} {
				if info.Name() == p {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check ignored patterns
		for _, p := range ignorePatterns {
			matched, _ := filepath.Match(p, info.Name())
			if matched {
				return nil
			}
		}

		// Only check source-like files (heuristic)
		ext := filepath.Ext(path)
		allowedExts := map[string]bool{
			".go": true, ".js": true, ".ts": true, ".py": true, ".java": true, ".c": true, ".cpp": true, ".h": true, ".rs": true,
		}
		if !allowedExts[ext] {
			return nil
		}

		lines, err := readLines(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		// Filter out short files
		if len(lines) < minLines {
			return nil
		}

		fileLines[path] = lines
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 2. Generate hashes for sliding windows
	// Map: hash -> []Location
	hashes := make(map[string][]Location)

	for path, lines := range fileLines {
		for i := 0; i <= len(lines)-minLines; i++ {
			// Create window content
			var window strings.Builder
			for j := 0; j < minLines; j++ {
				// Normalize: trim whitespace
				window.WriteString(strings.TrimSpace(lines[i+j]))
				window.WriteString("\n")
			}

			h := sha256.Sum256([]byte(window.String()))
			hashStr := hex.EncodeToString(h[:])

			hashes[hashStr] = append(hashes[hashStr], Location{
				File:      path,
				StartLine: i + 1,
				EndLine:   i + minLines,
			})
		}
	}

	// 3. Generate Pairwise Matches
	var pairs []MatchPair
	for _, locs := range hashes {
		if len(locs) < 2 {
			continue
		}
		for i := 0; i < len(locs); i++ {
			for j := i + 1; j < len(locs); j++ {
				l1, l2 := locs[i], locs[j]

				// Canonicalize order: FileA < FileB, or StartA < StartB
				if l1.File > l2.File || (l1.File == l2.File && l1.StartLine > l2.StartLine) {
					l1, l2 = l2, l1
				}

				pairs = append(pairs, MatchPair{
					FileA: l1.File, StartA: l1.StartLine, EndA: l1.EndLine,
					FileB: l2.File, StartB: l2.StartLine, EndB: l2.EndLine,
				})
			}
		}
	}

	// 4. Sort Pairs
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].FileA != pairs[j].FileA {
			return pairs[i].FileA < pairs[j].FileA
		}
		if pairs[i].FileB != pairs[j].FileB {
			return pairs[i].FileB < pairs[j].FileB
		}
		if pairs[i].StartA != pairs[j].StartA {
			return pairs[i].StartA < pairs[j].StartA
		}
		return pairs[i].StartB < pairs[j].StartB
	})

	// 5. Merge Overlapping/Adjacent Pairs
	var merged []MatchPair
	if len(pairs) > 0 {
		current := pairs[0]
		for i := 1; i < len(pairs); i++ {
			next := pairs[i]

			// Check if same file pair
			if current.FileA == next.FileA && current.FileB == next.FileB {
				// Check constant offset (diagonal)
				diffCurrent := current.StartB - current.StartA
				diffNext := next.StartB - next.StartA

				if diffCurrent == diffNext {
					// Check if next starts within or immediately after current
					// Since we sorted by StartA, next.StartA >= current.StartA
					// Overlap or adjacency: next.StartA <= current.EndA + 1
					if next.StartA <= current.EndA+1 {
						// Merge: extend ends
						if next.EndA > current.EndA {
							current.EndA = next.EndA
							current.EndB = next.EndB
						}
						continue
					}
				}
			}

			merged = append(merged, current)
			current = next
		}
		merged = append(merged, current)
	}

	// 6. Convert to Duplication Results
	var results []Duplication
	for _, m := range merged {
		results = append(results, Duplication{
			LineCount: m.EndA - m.StartA + 1,
			Locations: []Location{
				{File: m.FileA, StartLine: m.StartA, EndLine: m.EndA},
				{File: m.FileB, StartLine: m.StartB, EndLine: m.EndB},
			},
		})
	}

	// Sort results by line count (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].LineCount > results[j].LineCount
	})

	return results, nil
}

func printCPDTable(cmd *cobra.Command, dups []Duplication) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "LINES\tFILES")

	for _, d := range dups {
		// Because we now report pairs, it's always 2 locations.
		// But we can format it nicely.
		l1 := d.Locations[0]
		l2 := d.Locations[1]

		fmt.Fprintf(w, "%d\t%s:%d-%d <==> %s:%d-%d\n",
			d.LineCount,
			l1.File, l1.StartLine, l1.EndLine,
			l2.File, l2.StartLine, l2.EndLine,
		)
	}
	w.Flush()
}
