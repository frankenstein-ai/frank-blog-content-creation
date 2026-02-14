package hugo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Post holds metadata parsed from a Hugo post's TOML frontmatter.
type Post struct {
	Title string
	Date  time.Time
	Slug  string // filename without .md
}

// FindLatestPost finds the most recently dated post in hugoDir/content/posts/.
func FindLatestPost(hugoDir string) (*Post, error) {
	postsDir := filepath.Join(hugoDir, "content", "posts")
	entries, err := os.ReadDir(postsDir)
	if err != nil {
		return nil, fmt.Errorf("reading posts directory: %w", err)
	}

	var latest *Post
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(postsDir, e.Name()))
		if err != nil {
			continue
		}

		title, date := parseTOMLFrontmatter(string(data))
		if title == "" || date.IsZero() {
			continue
		}

		slug := strings.TrimSuffix(e.Name(), ".md")
		p := &Post{Title: title, Date: date, Slug: slug}
		if latest == nil || p.Date.After(latest.Date) {
			latest = p
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no posts found in %s", postsDir)
	}
	return latest, nil
}

// parseTOMLFrontmatter extracts title and date from +++ delimited TOML frontmatter.
func parseTOMLFrontmatter(content string) (string, time.Time) {
	var title string
	var date time.Time

	lines := strings.Split(content, "\n")
	inFrontmatter := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "+++" {
			if inFrontmatter {
				break // closing delimiter
			}
			inFrontmatter = true
			continue
		}
		if !inFrontmatter {
			continue
		}

		idx := strings.Index(trimmed, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])

		// Strip quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		switch key {
		case "title":
			title = val
		case "date":
			// Try RFC3339 variants
			for _, layout := range []string{
				time.RFC3339,
				"2006-01-02T15:04:05-07:00",
				"2006-01-02",
			} {
				if t, err := time.Parse(layout, val); err == nil {
					date = t
					break
				}
			}
		}
	}

	return title, date
}

// UpdateMenuEntry adds or replaces a "Latest:" menu entry in hugo.toml.
func UpdateMenuEntry(hugoDir, name, pageRef string) error {
	tomlPath := filepath.Join(hugoDir, "hugo.toml")
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return fmt.Errorf("reading hugo.toml: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	newBlock := []string{
		"[[menu.main]]",
		fmt.Sprintf("name = %q", name),
		fmt.Sprintf("pageRef = %q", pageRef),
		"weight = 3",
	}

	// Find existing "Latest:" block
	latestStart := -1
	latestEnd := -1
	lastMenuEnd := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[[menu.main]]" {
			// Track end of last menu block
			lastMenuEnd = findBlockEnd(lines, i)

			// Check if the next name line starts with "Latest:"
			for j := i + 1; j < len(lines) && j <= i+4; j++ {
				t := strings.TrimSpace(lines[j])
				if strings.HasPrefix(t, "name") {
					idx := strings.Index(t, "=")
					if idx >= 0 {
						val := strings.TrimSpace(t[idx+1:])
						val = strings.Trim(val, `"'`)
						if strings.HasPrefix(val, "Latest:") {
							latestStart = i
							latestEnd = findBlockEnd(lines, i)
							break
						}
					}
				}
			}
		}
	}

	var result []string
	if latestStart >= 0 {
		// Replace existing Latest: block
		result = append(result, lines[:latestStart]...)
		result = append(result, newBlock...)
		result = append(result, lines[latestEnd+1:]...)
	} else if lastMenuEnd >= 0 {
		// Append after last menu block
		insertAt := lastMenuEnd + 1
		result = append(result, lines[:insertAt]...)
		result = append(result, "")
		result = append(result, newBlock...)
		result = append(result, lines[insertAt:]...)
	} else {
		// No menu blocks at all — append at end
		result = append(result, lines...)
		result = append(result, "")
		result = append(result, newBlock...)
	}

	output := strings.Join(result, "\n")
	return os.WriteFile(tomlPath, []byte(output), 0644)
}

// findBlockEnd returns the index of the last non-empty line belonging to a
// [[menu.main]] block starting at startIdx.
func findBlockEnd(lines []string, startIdx int) int {
	end := startIdx
	for i := startIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			break
		}
		// Another section/table header means this block ended
		if strings.HasPrefix(trimmed, "[") {
			break
		}
		end = i
	}
	return end
}
