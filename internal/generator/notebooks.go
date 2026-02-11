package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/frankenstein-ai/frank-blog-content/internal/git"
	"github.com/frankenstein-ai/frank-blog-content/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content/internal/state"
)

type NotebookGenerator struct {
	LLM        llm.Provider
	State      *state.Store
	Templates  *prompts.Templates
	SourceRepo string
	OutputDir  string
	Period     string // "day" or "week"
	DryRun     bool
}

type GenerateResult struct {
	OutputPath string
	Content    string
	Commits    []string // hashes used
}

func (g *NotebookGenerator) Generate(ctx context.Context) ([]GenerateResult, error) {
	repoName := git.RepoName(g.SourceRepo)

	lastHash, err := g.State.GetLastCommit(g.SourceRepo, "notebook")
	if err != nil {
		return nil, fmt.Errorf("getting last commit: %w", err)
	}

	commits, err := git.GetCommits(g.SourceRepo, lastHash)
	if err != nil {
		return nil, fmt.Errorf("reading commits: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No new commits to process for notebooks.")
		return nil, nil
	}

	fmt.Printf("Found %d new commits for notebooks\n", len(commits))

	var groups map[string][]git.Commit
	switch g.Period {
	case "day":
		groups = git.GroupByDay(commits)
	default:
		groups = git.GroupByWeek(commits)
	}

	// Sort group keys for deterministic output
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	// Count existing notebooks to determine next sequence number
	nextSeq := countExistingNotebooks(g.OutputDir) + 1

	var results []GenerateResult
	for _, key := range keys {
		groupCommits := groups[key]

		// Determine the month from the first commit in this group
		month := groupCommits[0].Timestamp.Format("01")
		year := groupCommits[0].Timestamp.Format("2006")

		userPrompt := buildNotebookUserPrompt(repoName, key, groupCommits)

		if g.DryRun {
			fmt.Printf("[dry-run] Would generate notebook for %s (%d commits)\n", key, len(groupCommits))
			continue
		}

		fmt.Printf("Generating notebook for %s (%d commits)...\n", key, len(groupCommits))

		content, err := g.LLM.Generate(ctx, llm.Request{
			SystemPrompt: g.Templates.Notebook,
			UserPrompt:   userPrompt,
			MaxTokens:    4096,
			Temperature:  0.7,
		})
		if err != nil {
			return nil, fmt.Errorf("generating notebook for %s: %w", key, err)
		}

		// Extract topic slug from the first line of LLM output
		topicSlug, body := extractTopicSlug(content)

		// Build filename: 2025-02-LLM-Reasoning-01.md
		filename := fmt.Sprintf("%s-%s-%s-%02d.md", year, month, topicSlug, nextSeq)

		// Replace placeholders in the heading
		heading := fmt.Sprintf("# %s-%s-%s-%02d", year, month, topicSlug, nextSeq)
		body = replaceFirstHeading(body, heading)

		outPath := filepath.Join(g.OutputDir, filename)

		if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
			return nil, fmt.Errorf("writing notebook: %w", err)
		}

		hashes := make([]string, len(groupCommits))
		for j, c := range groupCommits {
			hashes[j] = c.Hash
		}

		if err := g.State.RecordGeneration(g.SourceRepo, "notebook", outPath, hashes); err != nil {
			return nil, fmt.Errorf("recording generation: %w", err)
		}

		results = append(results, GenerateResult{
			OutputPath: outPath,
			Content:    body,
			Commits:    hashes,
		})

		fmt.Printf("  Written: %s\n", outPath)
		nextSeq++
	}

	// Update last processed commit (use the most recent commit)
	if len(commits) > 0 && !g.DryRun {
		newest := commits[0] // git log returns newest first
		if err := g.State.SetLastCommit(g.SourceRepo, "notebook", newest.Hash, newest.Timestamp); err != nil {
			return nil, fmt.Errorf("updating state: %w", err)
		}
	}

	return results, nil
}

func buildNotebookUserPrompt(repoName, period string, commits []git.Commit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\n", repoName)
	fmt.Fprintf(&b, "Period: %s\n", period)
	fmt.Fprintf(&b, "\nCommits (%d total):\n\n", len(commits))

	for _, c := range commits {
		fmt.Fprintf(&b, "---\n")
		fmt.Fprintf(&b, "Hash: %s\n", c.Hash[:8])
		fmt.Fprintf(&b, "Date: %s\n", c.Timestamp.Format("2006-01-02 15:04"))
		fmt.Fprintf(&b, "Author: %s\n", c.Author)
		fmt.Fprintf(&b, "Subject: %s\n", c.Subject)
		if c.Body != "" {
			fmt.Fprintf(&b, "Description:\n%s\n", c.Body)
		}
		if len(c.Files) > 0 {
			fmt.Fprintf(&b, "Files changed:\n")
			for _, f := range c.Files {
				fmt.Fprintf(&b, "  %s %s\n", f.Status, f.Path)
			}
		}
		fmt.Fprintf(&b, "---\n\n")
	}

	b.WriteString("Generate a research notebook for this work period.")
	return b.String()
}

// extractTopicSlug pulls the topic slug from the first line of LLM output.
// Returns the slug and the remaining content.
func extractTopicSlug(content string) (string, string) {
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) == 0 {
		return "Research", content
	}

	slug := strings.TrimSpace(lines[0])
	// Clean up any quotes or markdown the LLM might have added
	slug = strings.Trim(slug, "\"'`# ")

	if slug == "" || strings.Contains(slug, " ") || len(slug) > 40 {
		return "Research", content
	}

	body := ""
	if len(lines) > 1 {
		body = strings.TrimLeft(lines[1], "\n")
	}

	return slug, body
}

// replaceFirstHeading replaces the first markdown h1 heading with the given heading.
func replaceFirstHeading(content, heading string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			lines[i] = heading
			return strings.Join(lines, "\n")
		}
	}
	// No heading found — prepend it
	return heading + "\n\n" + content
}

// countExistingNotebooks counts .md files in a directory to determine the next sequence number.
func countExistingNotebooks(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}
	return count
}
