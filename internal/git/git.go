package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Commit struct {
	Hash      string
	Subject   string
	Body      string
	Author    string
	Timestamp time.Time
	Files     []FileChange
}

type FileChange struct {
	Status string // "A", "M", "D", "R", etc.
	Path   string
}

// GetCommits returns commits from repoPath since sinceHash (exclusive).
// If sinceHash is empty, returns all commits.
func GetCommits(repoPath string, sinceHash string) ([]Commit, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolving repo path: %w", err)
	}

	// Use null bytes as field separators and a special end-of-record marker
	format := "%H%x00%s%x00%b%x00%an%x00%aI%x00END_COMMIT%x00"
	args := []string{"-C", absPath, "log", "--format=" + format, "--name-status"}

	if sinceHash != "" {
		args = append(args, sinceHash+"..HEAD")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git log failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	return parseCommits(string(output))
}

// GetDiffStat returns the file change summary for a commit.
func GetDiffStat(repoPath string, hash string) (string, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("resolving repo path: %w", err)
	}

	cmd := exec.Command("git", "-C", absPath, "show", "--stat", "--no-patch", hash)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git show --stat failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// RepoName extracts the repository name from a path.
func RepoName(repoPath string) string {
	absPath, _ := filepath.Abs(repoPath)
	return filepath.Base(absPath)
}

func parseCommits(output string) ([]Commit, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	// Split by the end-of-record marker
	records := strings.Split(output, "END_COMMIT\x00")
	var commits []Commit

	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		// Each record has: hash\0subject\0body\0author\0date\0\nfile_status_lines
		// The body may contain newlines, so we split by \0 carefully
		parts := strings.SplitN(record, "\x00", 6)
		if len(parts) < 5 {
			continue
		}

		hash := strings.TrimSpace(parts[0])
		subject := parts[1]
		body := strings.TrimSpace(parts[2])
		author := parts[3]
		dateStr := parts[4]

		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(dateStr))
		if err != nil {
			// Try without timezone offset
			ts, _ = time.Parse("2006-01-02T15:04:05-07:00", strings.TrimSpace(dateStr))
		}

		// Parse file changes from remaining content
		var files []FileChange
		if len(parts) > 5 {
			files = parseFileChanges(parts[5])
		}

		commits = append(commits, Commit{
			Hash:      hash,
			Subject:   subject,
			Body:      body,
			Author:    author,
			Timestamp: ts,
			Files:     files,
		})
	}

	return commits, nil
}

func parseFileChanges(raw string) []FileChange {
	var files []FileChange
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "M\tpath/to/file" or "R100\told\tnew"
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) >= 2 {
			files = append(files, FileChange{
				Status: parts[0],
				Path:   parts[len(parts)-1], // Use last part (handles renames)
			})
		}
	}
	return files
}

// GetCommit looks up a single commit by hash, validating it exists.
func GetCommit(repoPath string, hash string) (*Commit, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolving repo path: %w", err)
	}

	format := "%H%x00%s%x00%b%x00%an%x00%aI%x00END_COMMIT%x00"
	cmd := exec.Command("git", "-C", absPath, "log", "-1", "--format="+format, hash)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("commit %s not found: %s", hash, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	commits, err := parseCommits(string(output))
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("commit %s not found", hash)
	}
	return &commits[0], nil
}

// GroupByWeek groups commits by ISO year-week.
func GroupByWeek(commits []Commit) map[string][]Commit {
	groups := make(map[string][]Commit)
	for _, c := range commits {
		year, week := c.Timestamp.ISOWeek()
		key := fmt.Sprintf("%d-W%02d", year, week)
		groups[key] = append(groups[key], c)
	}
	return groups
}

// GroupByDay groups commits by date.
func GroupByDay(commits []Commit) map[string][]Commit {
	groups := make(map[string][]Commit)
	for _, c := range commits {
		key := c.Timestamp.Format("2006-01-02")
		groups[key] = append(groups[key], c)
	}
	return groups
}
