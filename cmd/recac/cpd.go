package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
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

var (
	cpdMinLines int
	cpdIgnore   []string
	cpdJSON     bool
	cpdFail     bool
)

var cpdCmd = &cobra.Command{
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
		for _, p := range cpdIgnore {
			if p != "[]" {
				cleanIgnore = append(cleanIgnore, p)
			}
		}
		cpdIgnore = cleanIgnore

		duplicates, err := runCPD(root, cpdMinLines, cpdIgnore)
		if err != nil {
			return err
		}

		if cpdJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(duplicates)
		}

		if len(duplicates) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No duplicates found. Great job!")
			return nil
		}

		printCPDTable(cmd, duplicates)

		if cpdFail {
			return fmt.Errorf("found %d duplicated blocks", len(duplicates))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cpdCmd)
	cpdCmd.Flags().IntVar(&cpdMinLines, "min-lines", 10, "Minimum lines to match")
	cpdCmd.Flags().StringSliceVar(&cpdIgnore, "ignore", []string{}, "Ignore patterns (glob)")
	cpdCmd.Flags().BoolVar(&cpdJSON, "json", false, "Output results as JSON")
	cpdCmd.Flags().BoolVar(&cpdFail, "fail", false, "Exit with error code if duplicates found")
}

func hashLineBytes(b []byte) uint64 {
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64()
}

// readAndHashFile reads a file and returns a slice of hashes for each line (trimmed).
// This avoids allocating strings for every line in the file.
func readAndHashFile(path string) ([]uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hashes []uint64
	// Use a reasonable buffer size for scanner if lines are long, but default is usually fine
	scanner := bufio.NewScanner(file)

	// Increase max token size if needed? Default 64k is usually enough for source code lines.

	for scanner.Scan() {
		// scanner.Bytes() returns a slice that may be overwritten by next Scan call.
		// We use it immediately to hash.
		// We trim space from the bytes.
		lineBytes := scanner.Bytes()
		trimmed := bytes.TrimSpace(lineBytes)
		hashes = append(hashes, hashLineBytes(trimmed))
	}

	return hashes, scanner.Err()
}

func runCPD(root string, minLines int, ignorePatterns []string) ([]Duplication, error) {
	hashes, err := collectFileHashes(root, minLines, ignorePatterns)
	if err != nil {
		return nil, err
	}

	pairs := findPairs(hashes)
	merged := mergePairs(pairs)
	results := pairsToDuplications(merged)

	return results, nil
}

func collectFileHashes(root string, minLines int, ignorePatterns []string) (map[string][]Location, error) {
	hashes := make(map[string][]Location)
	windowBuf := make([]byte, minLines*8)
	defaultIgnores := DefaultIgnoreMap()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if defaultIgnores[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		for _, p := range ignorePatterns {
			matched, _ := filepath.Match(p, info.Name())
			if matched {
				return nil
			}
		}

		ext := filepath.Ext(path)
		allowedExts := map[string]bool{
			".go": true, ".js": true, ".ts": true, ".py": true, ".java": true, ".c": true, ".cpp": true, ".h": true, ".rs": true,
		}
		if !allowedExts[ext] {
			return nil
		}

		lineHashes, err := readAndHashFile(path)
		if err != nil {
			return nil
		}

		if len(lineHashes) < minLines {
			return nil
		}

		for i := 0; i <= len(lineHashes)-minLines; i++ {
			for j := 0; j < minLines; j++ {
				binary.BigEndian.PutUint64(windowBuf[j*8:], lineHashes[i+j])
			}

			h := sha256.Sum256(windowBuf)
			hashStr := string(h[:])

			hashes[hashStr] = append(hashes[hashStr], Location{
				File:      path,
				StartLine: i + 1,
				EndLine:   i + minLines,
			})
		}

		return nil
	})

	return hashes, err
}

func findPairs(hashes map[string][]Location) []MatchPair {
	var pairs []MatchPair
	for _, locs := range hashes {
		if len(locs) < 2 {
			continue
		}
		for i := 0; i < len(locs); i++ {
			for j := i + 1; j < len(locs); j++ {
				l1, l2 := locs[i], locs[j]

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
	return pairs
}

func mergePairs(pairs []MatchPair) []MatchPair {
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

	var merged []MatchPair
	if len(pairs) > 0 {
		current := pairs[0]
		for i := 1; i < len(pairs); i++ {
			next := pairs[i]

			if current.FileA == next.FileA && current.FileB == next.FileB {
				diffCurrent := current.StartB - current.StartA
				diffNext := next.StartB - next.StartA

				if diffCurrent == diffNext {
					if next.StartA <= current.EndA+1 {
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
	return merged
}

func pairsToDuplications(merged []MatchPair) []Duplication {
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

	sort.Slice(results, func(i, j int) bool {
		return results[i].LineCount > results[j].LineCount
	})
	return results
}

func printCPDTable(cmd *cobra.Command, dups []Duplication) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "LINES\tFILES")

	for _, d := range dups {
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
