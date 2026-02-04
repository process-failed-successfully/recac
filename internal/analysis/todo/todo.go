package todo

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"recac/internal/utils"
	"regexp"
	"strings"
)

// Item represents a scanned technical debt marker (TODO, FIXME, etc.).
type Item struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Keyword string `json:"type"`    // TODO, FIXME, BUG, etc.
	Content string `json:"message"` // The content of the comment
	Raw     string `json:"-"`       // The raw string for TODO.md (optional)
}

// Regex to catch TODOs.
// Matches: (//|#|<!--|--|/*|%|;) [whitespace] (TODO|FIXME|BUG|HACK|NOTE|XXX) [optional: (stuff)] [whitespace|:] (content)
var strictRegex = regexp.MustCompile(`(?i)(\/\/|#|<!--|--|\/\*|%|;)\s*(TODO|FIXME|BUG|HACK|NOTE|XXX)(?:\((.*)\))?[:\s]+(.*)`)

// Scan scans the root directory for TODO items.
func Scan(root string, ignoreMap map[string]bool) ([]Item, error) {
	var items []Item

	if ignoreMap == nil {
		ignoreMap = utils.DefaultIgnoreMap()
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Don't skip the root itself if it's the start
		if path != root && path != "." {
			if d.IsDir() {
				if ignoreMap[d.Name()] {
					return filepath.SkipDir
				}
				if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." {
					return filepath.SkipDir
				}
				return nil
			}
		} else if d.IsDir() && path != "." {
			// If root is a dir, we generally want to scan it, but check ignores
			if ignoreMap[d.Name()] {
				return filepath.SkipDir
			}
		}

		if !d.IsDir() {
			if ignoreMap[d.Name()] {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if utils.IsBinaryExt(ext) {
				return nil
			}
		} else {
			return nil // Already handled dir logic above
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		// Read first 512 bytes to check for binary
		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		if n > 0 && utils.IsBinaryContent(buf[:n]) {
			return nil
		}

		// Reset file pointer
		f.Seek(0, 0)

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			matches := strictRegex.FindStringSubmatch(strings.TrimSpace(line))
			if len(matches) > 4 {
				// matches[1] is comment starter
				// matches[2] is keyword
				// matches[3] is author/context (optional)
				// matches[4] is content

				keyword := strings.ToUpper(matches[2])
				author := strings.TrimSpace(matches[3])
				content := strings.TrimSpace(matches[4])

				// Remove trailing comment closers like */ or -->
				content = strings.TrimSuffix(content, "*/")
				content = strings.TrimSuffix(content, "-->")
				content = strings.TrimSpace(content)

				if content == "" {
					continue
				}

				if author != "" {
					content = fmt.Sprintf("[%s] %s", author, content)
				}

				displayPath := path
				if cwd, err := os.Getwd(); err == nil {
					if rel, err := filepath.Rel(cwd, path); err == nil {
						displayPath = rel
					}
				}

				// Format: [File:Line] Keyword: Content
				raw := fmt.Sprintf("[%s:%d] %s: %s", displayPath, lineNum, keyword, content)

				items = append(items, Item{
					File:    displayPath,
					Line:    lineNum,
					Keyword: keyword,
					Content: content,
					Raw:     raw,
				})
			}
		}

		return nil
	})

	return items, err
}
