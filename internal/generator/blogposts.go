package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/hugo"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/skills"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
)

type GenerateResult struct {
	OutputPath string
	Content    string
	Commits    []string
}

type BlogPostGenerator struct {
	LLM           llm.Provider
	State         *state.Store
	Templates     *prompts.Templates
	Skills        []skills.Skill
	SourceRepo    string
	OutputDir     string
	Period        string // "day" or "week"
	ReadmeContent string
	DryRun        bool
}

func (g *BlogPostGenerator) Generate(ctx context.Context) ([]GenerateResult, error) {
	repoName := git.RepoName(g.SourceRepo)

	lastHash, err := g.State.GetLastCommit(g.SourceRepo, "blog-post")
	if err != nil {
		return nil, fmt.Errorf("getting last commit: %w", err)
	}

	commits, err := git.GetCommits(g.SourceRepo, lastHash)
	if err != nil {
		return nil, fmt.Errorf("reading commits: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No new commits to process for blog posts.")
		return nil, nil
	}

	fmt.Printf("Found %d new commits for blog posts\n", len(commits))

	var groups map[string][]git.Commit
	switch g.Period {
	case "day":
		groups = git.GroupByDay(commits)
	default:
		groups = git.GroupByWeek(commits)
	}

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

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

		// Date from first commit in group for accurate filenames
		date := groupCommits[0].Timestamp.Format("2006-01-02T15:04:05-07:00")

		userPrompt := buildUserPrompt(repoName, key, groupCommits, diffs, g.ReadmeContent)

		if g.DryRun {
			fmt.Printf("[dry-run] Would generate blog post for %s (%d commits)\n", key, len(groupCommits))
			continue
		}

		fmt.Printf("Generating blog post for %s (%d commits)...\n", key, len(groupCommits))

		systemPrompt := strings.Replace(g.Templates.BlogPost, "{{.Date}}", date, 1)

		content, err := g.LLM.Generate(ctx, llm.Request{
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
			MaxTokens:    4096,
			Temperature:  0.7,
		})
		if err != nil {
			return nil, fmt.Errorf("generating blog post for %s: %w", key, err)
		}

		// Run post-processing skills
		for _, skill := range g.Skills {
			fmt.Printf("  Running skill '%s' on blog post for %s...\n", skill.Name, key)
			frontmatter, body := hugo.SplitFrontmatter(content)
			processed, err := g.LLM.Generate(ctx, llm.Request{
				SystemPrompt: skill.Prompt,
				UserPrompt:   body,
				MaxTokens:    4096,
				Temperature:  0.4,
			})
			if err != nil {
				return nil, fmt.Errorf("skill '%s' for %s: %w", skill.Name, key, err)
			}
			processed = hugo.SanitizeLLMOutput(processed)
			content = frontmatter + "\n" + strings.TrimSpace(processed) + "\n"
		}

		slug := extractSlug(content)
		datePrefix := groupCommits[0].Timestamp.Format("2006-01-02")
		filename := fmt.Sprintf("%s-%s.md", datePrefix, slug)
		outPath := filepath.Join(g.OutputDir, filename)

		if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("writing blog post: %w", err)
		}

		hashes := make([]string, len(groupCommits))
		for j, c := range groupCommits {
			hashes[j] = c.Hash
		}

		if err := g.State.RecordGeneration(g.SourceRepo, "blog-post", outPath, hashes); err != nil {
			return nil, fmt.Errorf("recording generation: %w", err)
		}

		results = append(results, GenerateResult{
			OutputPath: outPath,
			Content:    content,
			Commits:    hashes,
		})

		fmt.Printf("  Written: %s\n", outPath)
	}

	// Update last processed commit (use the most recent commit)
	if len(commits) > 0 && !g.DryRun {
		newest := commits[0] // git log returns newest first
		if err := g.State.SetLastCommit(g.SourceRepo, "blog-post", newest.Hash, newest.Timestamp); err != nil {
			return nil, fmt.Errorf("updating state: %w", err)
		}
	}

	return results, nil
}

func buildUserPrompt(repoName, period string, commits []git.Commit, diffs map[string]string, readmeContent string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\n", repoName)
	fmt.Fprintf(&b, "Period: %s\n", period)

	if readmeContent != "" {
		fmt.Fprintf(&b, "\nProject description (from README):\n%s\n", readmeContent)
	}

	fmt.Fprintf(&b, "\nCommits (%d total):\n\n", len(commits))

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

	b.WriteString("Write a blog post covering this work. Focus on what makes it interesting and useful for other developers.")
	return b.String()
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func extractSlug(content string) string {
	// Try to extract title from frontmatter
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "title") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				title := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
				slug := strings.ToLower(title)
				slug = nonAlphanumeric.ReplaceAllString(slug, "-")
				slug = strings.Trim(slug, "-")
				if len(slug) > 60 {
					slug = slug[:60]
				}
				return slug
			}
		}
	}
	return "blog-post"
}
