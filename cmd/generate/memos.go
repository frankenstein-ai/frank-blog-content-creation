package generate

import (
	"context"
	"fmt"

	"github.com/frankenstein-ai/frank-blog-content/internal/config"
	"github.com/frankenstein-ai/frank-blog-content/internal/generator"
	"github.com/frankenstein-ai/frank-blog-content/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content/internal/state"
	"github.com/spf13/cobra"
)

var memosCmd = &cobra.Command{
	Use:   "memos",
	Short: "Generate insight memos from git commits",
	RunE:  runMemos,
}

func init() {
	memosCmd.Flags().String("source-repo", "", "Path to source git repository (env: FRANK_SOURCE_REPO)")
	memosCmd.Flags().String("output-dir", "", "Output directory for memos (env: FRANK_MEMOS_DIR)")
}

func runMemos(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cmd)
	if err != nil {
		return err
	}

	if cfg.SourceRepo == "" {
		return fmt.Errorf("--source-repo or FRANK_SOURCE_REPO is required")
	}
	outputDir := cfg.MemosDir
	if outputDir == "" {
		outputDir = cfg.OutputDir
	}
	if outputDir == "" {
		return fmt.Errorf("--output-dir or FRANK_MEMOS_DIR is required")
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

	gen := &generator.MemoGenerator{
		LLM:        provider,
		State:      store,
		Templates:  tmpls,
		SourceRepo: cfg.SourceRepo,
		OutputDir:  outputDir,
		DryRun:     cfg.DryRun,
	}

	results, err := gen.Generate(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("Generated %d insight memos\n", len(results))
	return nil
}
