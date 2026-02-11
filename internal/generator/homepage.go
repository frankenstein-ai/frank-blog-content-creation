package generator

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
)

type HomepageGenerator struct {
	LLM          llm.Provider
	State        *state.Store
	Templates    *prompts.Templates
	NotebooksDir string
	MemosDir     string
	OutputFile   string
	DryRun       bool
}

func (g *HomepageGenerator) Generate(ctx context.Context) (*GenerateResult, error) {
	notebooks, err := readMarkdownFiles(g.NotebooksDir)
	if err != nil {
		return nil, fmt.Errorf("reading notebooks: %w", err)
	}

	memos, err := readMarkdownFiles(g.MemosDir)
	if err != nil {
		return nil, fmt.Errorf("reading memos: %w", err)
	}

	if len(notebooks) == 0 && len(memos) == 0 {
		fmt.Println("No notebooks or memos found to generate homepage from.")
		return nil, nil
	}

	// Read current homepage if it exists
	var currentHomepage string
	if data, err := os.ReadFile(g.OutputFile); err == nil {
		currentHomepage = string(data)
	}

	userPrompt := buildHomepageUserPrompt(currentHomepage, notebooks, memos)

	if g.DryRun {
		fmt.Printf("[dry-run] Would update homepage from %d notebooks and %d memos\n", len(notebooks), len(memos))
		return nil, nil
	}

	fmt.Println("Generating homepage...")

	content, err := g.LLM.Generate(ctx, llm.Request{
		SystemPrompt: g.Templates.Homepage,
		UserPrompt:   userPrompt,
		MaxTokens:    4096,
		Temperature:  0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("generating homepage: %w", err)
	}

	if err := os.WriteFile(g.OutputFile, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("writing homepage: %w", err)
	}

	if err := g.State.RecordGeneration("homepage", "homepage", g.OutputFile, nil); err != nil {
		return nil, fmt.Errorf("recording generation: %w", err)
	}

	fmt.Printf("  Written: %s\n", g.OutputFile)

	return &GenerateResult{
		OutputPath: g.OutputFile,
		Content:    content,
	}, nil
}

func buildHomepageUserPrompt(currentHomepage string, notebooks, memos map[string]string) string {
	var b strings.Builder

	if currentHomepage != "" {
		fmt.Fprintf(&b, "Current homepage:\n%s\n\n", currentHomepage)
	}

	if len(notebooks) > 0 {
		b.WriteString("Recent notebooks:\n\n")
		for name, content := range notebooks {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", name, content)
		}
	}

	if len(memos) > 0 {
		b.WriteString("Recent insight memos:\n\n")
		for name, content := range memos {
			fmt.Fprintf(&b, "--- %s ---\n%s\n\n", name, content)
		}
	}

	b.WriteString("Update the homepage to reflect the latest research and work in progress.")
	return b.String()
}
