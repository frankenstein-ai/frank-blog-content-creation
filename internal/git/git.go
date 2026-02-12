package git

import (
	"fmt"
	"os"
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

	// Use null bytes as field separators with start/end markers to cleanly
	// separate commit metadata from --name-status file listings
	format := "START_COMMIT%x00%H%x00%s%x00%b%x00%an%x00%aI%x00END_COMMIT%x00"
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

	// Split by START marker to isolate each commit record
	records := strings.Split(output, "START_COMMIT\x00")
	var commits []Commit

	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		// Split by END marker to separate metadata from file status
		endParts := strings.SplitN(record, "END_COMMIT\x00", 2)
		commitData := endParts[0]

		var fileStatusData string
		if len(endParts) > 1 {
			fileStatusData = endParts[1]
		}

		// Parse commit metadata: hash\0subject\0body\0author\0date\0
		parts := strings.SplitN(commitData, "\x00", 6)
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

		// Parse file changes from content after END marker
		var files []FileChange
		if fileStatusData != "" {
			files = parseFileChanges(fileStatusData)
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

	format := "START_COMMIT%x00%H%x00%s%x00%b%x00%an%x00%aI%x00END_COMMIT%x00"
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

// GetParentHash returns the parent hash of the given commit.
// Returns "" if the commit is a root commit (no parent).
func GetParentHash(repoPath, hash string) (string, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("resolving repo path: %w", err)
	}

	cmd := exec.Command("git", "-C", absPath, "rev-parse", hash+"~1")
	output, err := cmd.Output()
	if err != nil {
		// Root commit has no parent — return empty
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCommitDiff returns the unified diff for a single commit.
func GetCommitDiff(repoPath, hash string) (string, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("resolving repo path: %w", err)
	}

	cmd := exec.Command("git", "-C", absPath, "show", "--format=", "--patch", hash)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git show --patch failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git show --patch failed: %w", err)
	}
	return string(output), nil
}

// ReadREADME reads the README.md from a repository root.
// Returns empty string if the file doesn't exist.
func ReadREADME(repoPath string) string {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(absPath, "README.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
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
