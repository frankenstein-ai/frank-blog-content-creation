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
	"github.com/spf13/cobra"
)

var homeCmd = &cobra.Command{
	Use:   "home",
	Short: "Update homepage with latest published blog posts",
	RunE:  runHome,
}

const homeSystemPrompt = `You are editing the homepage of a blog. Your job is to UPDATE it — not rewrite it.

You will receive the current homepage markdown body and a list of published blog posts. Some posts may already be linked on the homepage; others are new.

## How to work

1. **Analyze the homepage structure.** Read every section and understand its purpose. Sections may include overviews of research areas, detailed project descriptions, mission statements, contact info, etc. Do not assume fixed section names — learn them from the content.

2. **Identify new blog posts.** Compare the blog post list against existing links on the homepage. A post is "new" if its slug does not already appear as a link anywhere on the page.

3. **Update every relevant section.** For each new blog post, evaluate ALL sections:
   - Overview/summary sections that list research areas or capabilities: add a brief mention if the post introduces a genuinely new area of work not already covered.
   - Detailed project sections: add the post under the matching project subsection, or create a new subsection if no existing one fits. Follow the exact pattern of existing subsections (heading level, description style, bullet format, link format).
   - Introductory/mission/contact sections: leave untouched unless a new post fundamentally changes the scope described.

4. **Preserve everything else.** Do not rephrase, reorganize, or rewrite existing content. Only add new text where the new blog posts warrant it. Existing links, descriptions, bullet points, and formatting must remain exactly as they are.

## Output rules

- Blog post links use the format: [Post Title](/posts/slug)
- Match existing heading levels (## for top-level sections, ### for subsections, etc.)
- No emoji
- Do NOT add sections like "Published blog posts" or "New posts" — integrate into existing structure
- Do NOT add conversational text like "Here is the updated content"
- Output ONLY the markdown body — no frontmatter, no +++ delimiters, no code fences, no backticks wrapping`

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

	// Read current homepage and split frontmatter from body
	homepagePath := filepath.Join(cfg.HugoDir, "content", "_index.md")
	currentHomepage, _ := os.ReadFile(homepagePath)
	frontmatter, currentBody := hugo.SplitFrontmatter(string(currentHomepage))

	// Read published blog posts
	posts, err := hugo.ReadAllPosts(cfg.HugoDir)
	if err != nil {
		return err
	}

	// Build post summaries with slug for link generation
	var postSummaries strings.Builder
	for _, p := range posts {
		postSummaries.WriteString(fmt.Sprintf("- Title: %s\n  Date: %s\n  Slug: %s\n  Link: /posts/%s\n",
			p.Title, p.Date.Format("2006-01-02"), p.Slug, p.Slug))
		body := hugo.StripFrontmatter(p.Content)
		words := strings.Fields(body)
		if len(words) > 200 {
			words = words[:200]
		}
		postSummaries.WriteString(fmt.Sprintf("  Summary: %s...\n\n", strings.Join(words, " ")))
	}

	// Read README for project context
	readmeContent := git.ReadREADME(".")

	// Build user prompt
	var userPrompt strings.Builder
	userPrompt.WriteString("Here is the CURRENT homepage body (this is your base — update it, do not rewrite):\n\n")
	userPrompt.WriteString(currentBody)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString("Here are ALL published blog posts (some may already be linked on the homepage):\n\n")
	userPrompt.WriteString(postSummaries.String())
	if readmeContent != "" {
		userPrompt.WriteString("---\n\nProject README for additional context:\n\n")
		userPrompt.WriteString(readmeContent)
	}
	userPrompt.WriteString("\n\nUpdate the homepage body to include any new blog posts that are not already linked. Output the full updated body.")

	homeTemp := 0.7
	if cfg.Temperature != 0 {
		homeTemp = cfg.Temperature
	}

	result, err := provider.Generate(context.Background(), llm.Request{
		SystemPrompt: homeSystemPrompt,
		UserPrompt:   userPrompt.String(),
		MaxTokens:    16384,
		Temperature:  homeTemp,
	})
	if err != nil {
		return err
	}

	body := hugo.SanitizeLLMOutput(result)
	// Strip any frontmatter the LLM may have included despite instructions
	_, strippedBody := hugo.SplitFrontmatter(body)
	if strippedBody != "" {
		body = strippedBody
	}

	// Reassemble: original frontmatter + new body
	output := frontmatter + "\n" + strings.TrimSpace(body) + "\n"

	if err := os.WriteFile(homepagePath, []byte(output), 0644); err != nil {
		return err
	}

	fmt.Printf("Homepage updated: %s\n", homepagePath)
	return nil
}

