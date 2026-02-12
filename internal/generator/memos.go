package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	userPrompt := buildMemoUserPrompt(repoName, commits, g.ReadmeContent)

	if g.DryRun {
		fmt.Printf("[dry-run] Would analyze %d commits for insight memos\n", len(commits))
		return nil, nil
	}

	fmt.Println("Analyzing commits for insight memos...")

	content, err := g.LLM.Generate(ctx, llm.Request{
		SystemPrompt: g.Templates.Memo,
		UserPrompt:   userPrompt,
		MaxTokens:    4096,
		Temperature:  0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("generating memos: %w", err)
	}

	if strings.TrimSpace(content) == "NO_MEMO" {
		fmt.Println("No insight-worthy findings in these commits.")
		if len(commits) > 0 {
			newest := commits[0]
			if err := g.State.SetLastCommit(g.SourceRepo, "memo", newest.Hash, newest.Timestamp); err != nil {
				return nil, fmt.Errorf("updating state: %w", err)
			}
		}
		return nil, nil
	}

	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	// Split on separator if multiple memos
	memos := strings.Split(content, "---MEMO_SEPARATOR---")

	allHashes := make([]string, len(commits))
	for i, c := range commits {
		allHashes[i] = c.Hash
	}

	// Determine the year from the most recent commit
	year := commits[0].Timestamp.Format("2006")

	// Count existing memos for sequential numbering
	nextSeq := countExistingMemos(g.OutputDir) + 1

	var results []GenerateResult
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

	// Update last processed commit
	if len(commits) > 0 {
		newest := commits[0]
		if err := g.State.SetLastCommit(g.SourceRepo, "memo", newest.Hash, newest.Timestamp); err != nil {
			return nil, fmt.Errorf("updating state: %w", err)
		}
	}

	return results, nil
}

func buildMemoUserPrompt(repoName string, commits []git.Commit, readmeContent string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\n", repoName)
	if readmeContent != "" {
		fmt.Fprintf(&b, "\nProject description (from README):\n%s\n", readmeContent)
	}
	fmt.Fprintf(&b, "\nRecent commits (%d total):\n\n", len(commits))

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
