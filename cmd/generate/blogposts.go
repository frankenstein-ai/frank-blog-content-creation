package generate

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/config"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/generator"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/skills"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
	"github.com/spf13/cobra"
)

var blogPostsCmd = &cobra.Command{
	Use:   "blog-posts",
	Short: "Generate blog posts from git commits",
	RunE:  runBlogPosts,
}

func init() {
	blogPostsCmd.Flags().String("period", "week", "Grouping period for commits: 'day' or 'week'")
}

func runBlogPosts(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cmd)
	if err != nil {
		return err
	}

	sourceRepo := "."

	if cfg.HugoDir == "" {
		return fmt.Errorf("--hugo-dir, FRANK_HUGO_DIR, or hugo_dir in .frank.toml is required")
	}
	outputDir := filepath.Join(cfg.HugoDir, "content", "posts")

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

	readmeContent := git.ReadREADME(sourceRepo)

	var loadedSkills []skills.Skill
	if !cfg.DryRun && len(cfg.Skills) > 0 {
		loadedSkills, err = skills.Load("skills", cfg.Skills)
		if err != nil {
			return err
		}
		fmt.Printf("Loaded %d skill(s): %v\n", len(loadedSkills), cfg.Skills)
	}

	gen := &generator.BlogPostGenerator{
		LLM:           provider,
		State:         store,
		Templates:     tmpls,
		Skills:        loadedSkills,
		SourceRepo:    sourceRepo,
		OutputDir:     outputDir,
		Period:        cfg.Period,
		ReadmeContent: readmeContent,
		DryRun:        cfg.DryRun,
	}

	results, err := gen.Generate(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("Generated %d blog posts\n", len(results))
	return nil
}
