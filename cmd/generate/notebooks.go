package generate

import (
	"context"
	"fmt"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/config"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/generator"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
	"github.com/spf13/cobra"
)

var notebooksCmd = &cobra.Command{
	Use:   "notebooks",
	Short: "Generate research notebooks from git commits",
	RunE:  runNotebooks,
}

func init() {
	notebooksCmd.Flags().String("source-repo", "", "Path to source git repository (env: FRANK_SOURCE_REPO)")
	notebooksCmd.Flags().String("output-dir", "", "Output directory for notebooks (env: FRANK_NOTEBOOKS_DIR)")
	notebooksCmd.Flags().String("period", "week", "Grouping period: day or week")
}

func runNotebooks(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cmd)
	if err != nil {
		return err
	}

	if cfg.SourceRepo == "" {
		return fmt.Errorf("--source-repo or FRANK_SOURCE_REPO is required")
	}
	outputDir := cfg.NotebooksDir
	if outputDir == "" {
		outputDir = cfg.OutputDir
	}
	if outputDir == "" {
		return fmt.Errorf("--output-dir or FRANK_NOTEBOOKS_DIR is required")
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

	gen := &generator.NotebookGenerator{
		LLM:           provider,
		State:         store,
		Templates:     tmpls,
		SourceRepo:    cfg.SourceRepo,
		OutputDir:     outputDir,
		Period:        cfg.Period,
		ReadmeContent: git.ReadREADME(cfg.SourceRepo),
		DryRun:        cfg.DryRun,
	}

	results, err := gen.Generate(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("Generated %d notebooks\n", len(results))
	return nil
}
