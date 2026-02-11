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

var homepageCmd = &cobra.Command{
	Use:   "homepage",
	Short: "Generate homepage from notebooks and memos",
	RunE:  runHomepage,
}

func init() {
	homepageCmd.Flags().String("notebooks-dir", "", "Directory containing notebooks")
	homepageCmd.Flags().String("memos-dir", "", "Directory containing insight memos")
	homepageCmd.Flags().String("output-file", "", "Output file for homepage")
}

func runHomepage(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cmd)
	if err != nil {
		return err
	}

	notebooksDir := cfg.NotebooksDir
	memosDir := cfg.MemosDir

	outputFile, _ := cmd.Flags().GetString("output-file")
	if outputFile == "" {
		return fmt.Errorf("--output-file is required")
	}

	if notebooksDir == "" && memosDir == "" {
		return fmt.Errorf("at least one of --notebooks-dir or --memos-dir is required")
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

	gen := &generator.HomepageGenerator{
		LLM:          provider,
		State:        store,
		Templates:    tmpls,
		NotebooksDir: notebooksDir,
		MemosDir:     memosDir,
		OutputFile:   outputFile,
		DryRun:       cfg.DryRun,
	}

	result, err := gen.Generate(context.Background())
	if err != nil {
		return err
	}

	if result != nil {
		fmt.Println("Homepage updated")
	}
	return nil
}
