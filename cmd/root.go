package cmd

import (
	"github.com/frankenstein-ai/frank-blog-content/cmd/generate"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "frank",
	Short: "Frankenstein AI Lab content generator",
	Long:  "Automatically generate research notebooks, insight memos, and blog posts from R&D git commits.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().String("llm-provider", "", "LLM provider: openai or anthropic (env: FRANK_LLM_PROVIDER)")
	rootCmd.PersistentFlags().String("llm-model", "", "LLM model name (env: FRANK_LLM_MODEL)")
	rootCmd.PersistentFlags().String("state-db", ".frank-state.db", "Path to SQLite state file (env: FRANK_STATE_DB)")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Print what would be generated without writing files")

	rootCmd.AddCommand(generate.Cmd)
}
