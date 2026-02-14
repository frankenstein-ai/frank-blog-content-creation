package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
)

type BlogPostGenerator struct {
	LLM          llm.Provider
	State        *state.Store
	Templates    *prompts.Templates
	SourceRepo   string
	NotebooksDir string
	MemosDir     string
	OutputDir    string
	DryRun       bool
}

func (g *BlogPostGenerator) Generate(ctx context.Context) ([]GenerateResult, error) {
	var notebooks, memos map[string]string
	var commits []git.Commit
	var err error

	if g.SourceRepo != "" {
		// Commit-based discovery: get checkpoint, read new commits, find new notebooks/memos
		lastHash, err := g.State.GetLastCommit(g.SourceRepo, "blog-post")
		if err != nil {
			return nil, fmt.Errorf("getting last commit: %w", err)
		}

		commits, err = git.GetCommits(g.SourceRepo, lastHash)
		if err != nil {
			return nil, fmt.Errorf("getting commits from source repo: %w", err)
		}

		if len(commits) == 0 {
			fmt.Println("No new commits since last run.")
			return nil, nil
		}

		fmt.Printf("Found %d new commits in source repo\n", len(commits))

		notebooks, memos, err = g.discoverNewFiles(commits)
		if err != nil {
			return nil, fmt.Errorf("discovering new files: %w", err)
		}
	} else {
		// Fallback: read all files from directories
		notebooks, err = readMarkdownFiles(g.NotebooksDir)
		if err != nil {
			return nil, fmt.Errorf("reading notebooks: %w", err)
		}

		memos, err = readMarkdownFiles(g.MemosDir)
		if err != nil {
			return nil, fmt.Errorf("reading memos: %w", err)
		}
	}

	if len(notebooks) == 0 && len(memos) == 0 {
		fmt.Println("No notebooks or memos found to generate blog posts from.")
		return nil, nil
	}

	fmt.Printf("Found %d notebooks and %d memos\n", len(notebooks), len(memos))

	userPrompt := buildBlogPostUserPrompt(notebooks, memos)

	if g.DryRun {
		fmt.Printf("[dry-run] Would generate blog post from %d notebooks and %d memos\n", len(notebooks), len(memos))
		if g.SourceRepo != "" && len(commits) > 0 {
			fmt.Printf("[dry-run] Would update blog-post state to commit %s\n", commits[0].Hash[:8])
		}
		return nil, nil
	}

	fmt.Println("Generating blog post...")

	content, err := g.LLM.Generate(ctx, llm.Request{
		SystemPrompt: g.Templates.BlogPost,
		UserPrompt:   userPrompt,
		MaxTokens:    4096,
		Temperature:  0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("generating blog post: %w", err)
	}

	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	slug := extractSlug(content)
	filename := fmt.Sprintf("%s-%s.md", time.Now().Format("2006-01-02"), slug)
	outPath := filepath.Join(g.OutputDir, filename)

	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("writing blog post: %w", err)
	}

	if err := g.State.RecordGeneration(g.SourceRepo, "blog-post", outPath, nil); err != nil {
		return nil, fmt.Errorf("recording generation: %w", err)
	}

	// Update state to newest commit
	if g.SourceRepo != "" && len(commits) > 0 {
		newest := commits[0]
		if err := g.State.SetLastCommit(g.SourceRepo, "blog-post", newest.Hash, newest.Timestamp); err != nil {
			return nil, fmt.Errorf("updating blog-post state: %w", err)
		}
	}

	fmt.Printf("  Written: %s\n", outPath)

	return []GenerateResult{{
		OutputPath: outPath,
		Content:    content,
	}}, nil
}

func (g *BlogPostGenerator) discoverNewFiles(commits []git.Commit) (notebooks, memos map[string]string, err error) {
	absRepo, err := filepath.Abs(g.SourceRepo)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving source repo path: %w", err)
	}
	absNotebooks, _ := filepath.Abs(g.NotebooksDir)
	absMemos, _ := filepath.Abs(g.MemosDir)

	seen := make(map[string]bool)
	notebooks = make(map[string]string)
	memos = make(map[string]string)

	for _, c := range commits {
		for _, f := range c.Files {
			if f.Status != "A" && f.Status != "M" {
				continue
			}
			if !strings.HasSuffix(f.Path, ".md") || seen[f.Path] {
				continue
			}
			seen[f.Path] = true

			absPath := filepath.Join(absRepo, f.Path)
			content, err := os.ReadFile(absPath)
			if err != nil {
				continue // file might have been deleted in a later commit
			}

			name := filepath.Base(f.Path)
			if absNotebooks != "" && strings.HasPrefix(absPath, absNotebooks) {
				notebooks[name] = string(content)
			} else if absMemos != "" && strings.HasPrefix(absPath, absMemos) {
				memos[name] = string(content)
			}
		}
	}
	return notebooks, memos, nil
}

func buildBlogPostUserPrompt(notebooks, memos map[string]string) string {
	var b strings.Builder

	if len(notebooks) > 0 {
		b.WriteString("Source notebooks:\n\n")
		for name, content := range notebooks {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", name, content)
		}
	}

	if len(memos) > 0 {
		b.WriteString("Source insight memos:\n\n")
		for name, content := range memos {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", name, content)
		}
	}

	b.WriteString("Write a blog post covering this research. Focus on what makes it interesting and useful for other developers.")
	return b.String()
}

func readMarkdownFiles(dir string) (map[string]string, error) {
	if dir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	files := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		files[e.Name()] = string(content)
	}
	return files, nil
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
