package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
)

type MemoGenerator struct {
	LLM           llm.Provider
	State         *state.Store
	Templates     *prompts.Templates
	SourceRepo    string
	OutputDir     string
	Period        string // "day" or "week"
	ReadmeContent string
	DryRun        bool
}

func (g *MemoGenerator) Generate(ctx context.Context) ([]GenerateResult, error) {
	lastHash, err := g.State.GetLastCommit(g.SourceRepo, "memo")
	if err != nil {
		return nil, fmt.Errorf("getting last commit: %w", err)
	}

	commits, err := git.GetCommits(g.SourceRepo, lastHash)
	if err != nil {
		return nil, fmt.Errorf("reading commits: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No new commits to process for memos.")
		return nil, nil
	}

	fmt.Printf("Found %d new commits for insight memos\n", len(commits))

	repoName := git.RepoName(g.SourceRepo)

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

	// Count existing memos for sequential numbering
	nextSeq := countExistingMemos(g.OutputDir) + 1

	// Determine the year from the most recent commit
	year := commits[0].Timestamp.Format("2006")

	var results []GenerateResult
	for _, key := range keys {
		groupCommits := groups[key]

		// Fetch diffs for all commits in this group
		diffs := make(map[string]string)
		for _, c := range groupCommits {
			diff, err := git.GetCommitDiff(g.SourceRepo, c.Hash)
			if err != nil {
				shortHash := c.Hash
				if len(shortHash) > 8 {
					shortHash = shortHash[:8]
				}
				return nil, fmt.Errorf("getting diff for %s: %w", shortHash, err)
			}
			diffs[c.Hash] = diff
		}

		userPrompt := buildMemoUserPrompt(repoName, key, groupCommits, diffs, g.ReadmeContent)

		if g.DryRun {
			fmt.Printf("[dry-run] Would analyze %s (%d commits) for insight memos\n", key, len(groupCommits))
			continue
		}

		fmt.Printf("Analyzing %s (%d commits) for insight memos...\n", key, len(groupCommits))

		content, err := g.LLM.Generate(ctx, llm.Request{
			SystemPrompt: g.Templates.Memo,
			UserPrompt:   userPrompt,
			MaxTokens:    4096,
			Temperature:  0.7,
		})
		if err != nil {
			return nil, fmt.Errorf("generating memos for %s: %w", key, err)
		}

		if strings.TrimSpace(content) == "NO_MEMO" {
			fmt.Printf("  No insight-worthy findings in %s.\n", key)
			continue
		}

		// Split on separator if multiple memos
		memos := strings.Split(content, "---MEMO_SEPARATOR---")

		allHashes := make([]string, len(groupCommits))
		for i, c := range groupCommits {
			allHashes[i] = c.Hash
		}

		for _, memo := range memos {
			memo = strings.TrimSpace(memo)
			if memo == "" || memo == "NO_MEMO" {
				continue
			}

			// Filename: 2025-mobile-agents-insight-memo-001.md
			filename := fmt.Sprintf("%s-%s-insight-memo-%03d.md", year, repoName, nextSeq)
			outPath := filepath.Join(g.OutputDir, filename)

			if err := os.WriteFile(outPath, []byte(memo), 0o644); err != nil {
				return nil, fmt.Errorf("writing memo: %w", err)
			}

			if err := g.State.RecordGeneration(g.SourceRepo, "memo", outPath, allHashes); err != nil {
				return nil, fmt.Errorf("recording generation: %w", err)
			}

			results = append(results, GenerateResult{
				OutputPath: outPath,
				Content:    memo,
				Commits:    allHashes,
			})

			fmt.Printf("  Written: %s\n", outPath)
			nextSeq++
		}
	}

	// Update last processed commit
	if len(commits) > 0 && !g.DryRun {
		newest := commits[0]
		if err := g.State.SetLastCommit(g.SourceRepo, "memo", newest.Hash, newest.Timestamp); err != nil {
			return nil, fmt.Errorf("updating state: %w", err)
		}
	}

	return results, nil
}

func buildMemoUserPrompt(repoName, period string, commits []git.Commit, diffs map[string]string, readmeContent string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\n", repoName)
	fmt.Fprintf(&b, "Period: %s\n", period)

	if readmeContent != "" {
		fmt.Fprintf(&b, "\nProject description (from README):\n%s\n", readmeContent)
	}

	fmt.Fprintf(&b, "\nCommits (%d total):\n\n", len(commits))

	// Budget for diff chars across all commits in the group
	maxDiffPerCommit := 15000 / len(commits)
	if maxDiffPerCommit < 2000 {
		maxDiffPerCommit = 2000
	}

	for _, c := range commits {
		shortHash := c.Hash
		if len(shortHash) > 8 {
			shortHash = shortHash[:8]
		}

		fmt.Fprintf(&b, "---\n")
		fmt.Fprintf(&b, "Hash: %s\n", shortHash)
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

		if diff, ok := diffs[c.Hash]; ok && diff != "" {
			truncatedDiff := diff
			if len(truncatedDiff) > maxDiffPerCommit {
				truncatedDiff = truncatedDiff[:maxDiffPerCommit] + "\n... (diff truncated)"
			}
			fmt.Fprintf(&b, "Code changes (diff):\n```\n%s\n```\n", truncatedDiff)
		}
		fmt.Fprintf(&b, "---\n\n")
	}

	b.WriteString("Based on these commits, identify any durable insights worth documenting as an insight memo.\n")
	b.WriteString("If there are multiple distinct insights, generate multiple memos separated by \"---MEMO_SEPARATOR---\".\n")
	b.WriteString("If there are no insight-worthy findings, respond with exactly \"NO_MEMO\".")
	return b.String()
}

// countExistingMemos counts .md files in a directory to determine the next sequence number.
func countExistingMemos(dir string) int {
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
