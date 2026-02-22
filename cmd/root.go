package cmd

import (
	"github.com/frankenstein-ai/frank-blog-content-generator/cmd/generate"
	"github.com/frankenstein-ai/frank-blog-content-generator/cmd/update"
	"github.com/spf13/cobra"
)

// version is set via ldflags at build time (e.g. -X cmd.version=1.0.0).
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "frank",
	Short:   "Generate blog posts from git commits",
	Long:    "CLI tool that generates blog posts from your project's git history using LLMs.",
	Version: version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().String("llm-provider", "", "LLM provider: openai or anthropic (env: FRANK_LLM_PROVIDER)")
	rootCmd.PersistentFlags().String("llm-model", "", "LLM model name (env: FRANK_LLM_MODEL)")
	rootCmd.PersistentFlags().String("state-db", ".frank-state.db", "Path to SQLite state file (env: FRANK_STATE_DB)")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Print what would be generated without writing files")
	rootCmd.PersistentFlags().String("hugo-dir", "", "Path to Hugo site directory (env: FRANK_HUGO_DIR)")

	rootCmd.AddCommand(generate.Cmd)
	rootCmd.AddCommand(update.Cmd)
}
