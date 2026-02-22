package cmd

import (
	"fmt"
	"os"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize frank for this project",
	Long: `Initialize the processing state so that future generation runs only process
commits after the specified one. Also generates a .frank.toml config file.

Run this from the root of the project whose commits you want to turn into blog posts.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().String("commit", "", "Starting commit hash (required)")
	initCmd.Flags().String("hugo-dir", "", "Path to Hugo site directory (env: FRANK_HUGO_DIR)")
	initCmd.MarkFlagRequired("commit")
	initCmd.MarkFlagRequired("hugo-dir")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	commitHash, _ := cmd.Flags().GetString("commit")
	dbPath, _ := cmd.Flags().GetString("state-db")

	sourceRepo := "."

	commit, err := git.GetCommit(sourceRepo, commitHash)
	if err != nil {
		return err
	}

	short := commit.Hash
	if len(short) > 8 {
		short = short[:8]
	}

	// Store parent hash so the exclusive range (parent..HEAD) includes the target commit
	parentHash, err := git.GetParentHash(sourceRepo, commit.Hash)
	if err != nil {
		return fmt.Errorf("resolving parent commit: %w", err)
	}

	// Open state DB
	store, err := state.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.SetLastCommit(sourceRepo, "blog-post", parentHash, commit.Timestamp); err != nil {
		return fmt.Errorf("setting state: %w", err)
	}
	fmt.Printf("Initialized blog-post → commit %s (%s)\n", short, commit.Timestamp.Format("2006-01-02"))

	if err := writeConfig(cmd); err != nil {
		return err
	}

	return nil
}

func writeConfig(cmd *cobra.Command) error {
	const configFile = ".frank.toml"

	if _, err := os.Stat(configFile); err == nil {
		fmt.Printf("%s already exists, skipping\n", configFile)
		return nil
	}

	hugoDir := flagOrEnvInit(cmd, "hugo-dir", "FRANK_HUGO_DIR")
	llmProvider := flagOrEnvInit(cmd, "llm-provider", "FRANK_LLM_PROVIDER")
	llmModel := flagOrEnvInit(cmd, "llm-model", "FRANK_LLM_MODEL")
	stateDB, _ := cmd.Flags().GetString("state-db")
	period := flagOrEnvInit(cmd, "period", "")
	if period == "" {
		period = "week"
	}

	type entry struct {
		key     string
		value   string
		comment string
	}

	entries := []entry{
		{"hugo_dir", hugoDir, ""},
		{"state_db", stateDB, ""},
		{"llm_provider", llmProvider, ""},
		{"llm_model", llmModel, ""},
		{"period", period, "day or week"},
	}

	var content string
	content += "# .frank.toml — persistent config for frank CLI\n"
	content += "# Resolution order: CLI flags > env vars > .frank.toml > defaults\n\n"

	for _, e := range entries {
		if e.comment != "" {
			content += fmt.Sprintf("# %s\n", e.comment)
		}
		if e.value != "" {
			content += fmt.Sprintf("%s = \"%s\"\n", e.key, e.value)
		} else {
			content += fmt.Sprintf("# %s = \"\"\n", e.key)
		}
	}

	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", configFile, err)
	}

	fmt.Printf("Generated %s\n", configFile)
	return nil
}

func flagOrEnvInit(cmd *cobra.Command, flag, env string) string {
	if cmd.Flags().Changed(flag) {
		v, _ := cmd.Flags().GetString(flag)
		return v
	}
	if env != "" {
		return os.Getenv(env)
	}
	return ""
}
