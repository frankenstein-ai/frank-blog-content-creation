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

var notesCmd = &cobra.Command{
	Use:   "notes",
	Short: "Generate notebooks and insight memos from git commits",
	RunE:  runNotes,
}

func init() {
	notesCmd.Flags().String("source-repo", "", "Path to source git repository (env: FRANK_SOURCE_REPO)")
	notesCmd.Flags().String("notebooks-dir", "", "Output directory for notebooks (env: FRANK_NOTEBOOKS_DIR)")
	notesCmd.Flags().String("memos-dir", "", "Output directory for memos (env: FRANK_MEMOS_DIR)")
	notesCmd.Flags().String("period", "week", "Grouping period for notebooks: day or week")
}

func runNotes(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cmd)
	if err != nil {
		return err
	}

	store, err := state.Open(cfg.StateDB)
	if err != nil {
		return err
	}
	defer store.Close()

	// Resolve source repo: flag/env → state DB → error
	sourceRepo := cfg.SourceRepo
	if sourceRepo == "" {
		sourceRepo, err = store.GetSourceRepo("notebook")
		if err != nil {
			return fmt.Errorf("looking up source repo from state: %w", err)
		}
	}
	if sourceRepo == "" {
		return fmt.Errorf("--source-repo is required (or run 'frank init' first to set it)")
	}

	notebooksDir := cfg.NotebooksDir
	if notebooksDir == "" {
		return fmt.Errorf("--notebooks-dir or FRANK_NOTEBOOKS_DIR is required")
	}

	memosDir := cfg.MemosDir
	if memosDir == "" {
		return fmt.Errorf("--memos-dir or FRANK_MEMOS_DIR is required")
	}

	tmpls, err := prompts.Load()
	if err != nil {
		return err
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

	// Generate notebooks
	notebookGen := &generator.NotebookGenerator{
		LLM:        provider,
		State:      store,
		Templates:  tmpls,
		SourceRepo: sourceRepo,
		OutputDir:  notebooksDir,
		Period:     cfg.Period,
		DryRun:     cfg.DryRun,
	}

	notebooks, err := notebookGen.Generate(context.Background())
	if err != nil {
		return fmt.Errorf("generating notebooks: %w", err)
	}
	fmt.Printf("Generated %d notebooks\n", len(notebooks))

	// Generate memos
	memoGen := &generator.MemoGenerator{
		LLM:        provider,
		State:      store,
		Templates:  tmpls,
		SourceRepo: sourceRepo,
		OutputDir:  memosDir,
		DryRun:     cfg.DryRun,
	}

	memos, err := memoGen.Generate(context.Background())
	if err != nil {
		return fmt.Errorf("generating memos: %w", err)
	}
	fmt.Printf("Generated %d insight memos\n", len(memos))

	return nil
}
