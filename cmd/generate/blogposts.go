package generate

import (
	"context"
	"fmt"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/config"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/generator"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
	"github.com/spf13/cobra"
)

var blogPostsCmd = &cobra.Command{
	Use:   "blog-posts",
	Short: "Generate blog posts from notebooks and memos",
	RunE:  runBlogPosts,
}

func init() {
	blogPostsCmd.Flags().String("notebooks-dir", "", "Directory containing notebooks")
	blogPostsCmd.Flags().String("memos-dir", "", "Directory containing insight memos")
	blogPostsCmd.Flags().String("output-dir", "", "Output directory for blog posts (env: FRANK_BLOG_DIR)")
}

func runBlogPosts(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cmd)
	if err != nil {
		return err
	}

	notebooksDir := cfg.NotebooksDir
	memosDir := cfg.MemosDir
	outputDir := cfg.BlogDir
	if outputDir == "" {
		outputDir = cfg.OutputDir
	}

	if notebooksDir == "" && memosDir == "" {
		return fmt.Errorf("at least one of --notebooks-dir or --memos-dir is required")
	}
	if outputDir == "" {
		return fmt.Errorf("--output-dir or FRANK_BLOG_DIR is required")
	}

	var provider llm.Provider
	if !cfg.DryRun {
		if err := cfg.ValidateForGenerate(); err != nil {
			return err
		}
		provider, err = llm.New(cfg.LLMProvider, cfg.LLMModel, cfg.APIKey)
		if err != nil {
			return err
		}
	}

	store, err := state.Open(cfg.StateDB)
	if err != nil {
		return err
	}
	defer store.Close()

	tmpls, err := prompts.Load()
	if err != nil {
		return err
	}

	gen := &generator.BlogPostGenerator{
		LLM:          provider,
		State:        store,
		Templates:    tmpls,
		NotebooksDir: notebooksDir,
		MemosDir:     memosDir,
		OutputDir:    outputDir,
		DryRun:       cfg.DryRun,
	}

	results, err := gen.Generate(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("Generated %d blog posts\n", len(results))
	return nil
}
