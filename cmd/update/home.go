package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/config"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/hugo"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/prompts"
	"github.com/spf13/cobra"
)

var homeCmd = &cobra.Command{
	Use:   "home",
	Short: "Regenerate homepage from published blog posts",
	RunE:  runHome,
}

func runHome(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cmd)
	if err != nil {
		return err
	}

	if cfg.HugoDir == "" {
		return fmt.Errorf("--hugo-dir, FRANK_HUGO_DIR, or hugo_dir in .frank.toml is required")
	}

	if err := cfg.ValidateForGenerate(); err != nil {
		return err
	}

	provider, err := llm.New(cfg.LLMProvider, cfg.LLMModel, cfg.APIKey)
	if err != nil {
		return err
	}

	tmpls, err := prompts.Load()
	if err != nil {
		return err
	}

	// Read current homepage
	homepagePath := filepath.Join(cfg.HugoDir, "content", "_index.md")
	currentHomepage, _ := os.ReadFile(homepagePath)

	// Read published blog posts
	posts, err := hugo.ReadAllPosts(cfg.HugoDir)
	if err != nil {
		return err
	}

	// Build post summaries (title + first ~200 words)
	var postSummaries strings.Builder
	for _, p := range posts {
		postSummaries.WriteString(fmt.Sprintf("## %s (%s)\n", p.Title, p.Date.Format("2006-01-02")))
		body := stripFrontmatter(p.Content)
		words := strings.Fields(body)
		if len(words) > 200 {
			words = words[:200]
		}
		postSummaries.WriteString(strings.Join(words, " "))
		postSummaries.WriteString("\n\n")
	}

	// Read README for project context
	var readmeContent string
	if cfg.SourceRepo != "" {
		readmeContent = git.ReadREADME(cfg.SourceRepo)
	}

	// Build user prompt
	var userPrompt strings.Builder
	if len(currentHomepage) > 0 {
		userPrompt.WriteString("Current homepage:\n\n")
		userPrompt.WriteString(string(currentHomepage))
		userPrompt.WriteString("\n\n---\n\n")
	}
	userPrompt.WriteString("Published blog posts:\n\n")
	userPrompt.WriteString(postSummaries.String())
	if readmeContent != "" {
		userPrompt.WriteString("---\n\nProject README:\n\n")
		userPrompt.WriteString(readmeContent)
	}

	result, err := provider.Generate(context.Background(), llm.Request{
		SystemPrompt: tmpls.Homepage,
		UserPrompt:   userPrompt.String(),
		MaxTokens:    4096,
		Temperature:  0.7,
	})
	if err != nil {
		return err
	}

	if err := os.WriteFile(homepagePath, []byte(result), 0644); err != nil {
		return err
	}

	fmt.Printf("Homepage updated: %s\n", homepagePath)
	return nil
}

// stripFrontmatter removes +++ delimited TOML frontmatter from content.
func stripFrontmatter(content string) string {
	parts := strings.SplitN(content, "+++", 3)
	if len(parts) == 3 {
		return strings.TrimSpace(parts[2])
	}
	return content
}
