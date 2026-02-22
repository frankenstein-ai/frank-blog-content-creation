package cmd

import (
	"fmt"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
	"github.com/spf13/cobra"
)

var statusUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Move the last processed commit pointer",
	Long:  "Manually set the last processed commit hash in the state database. Use this to skip commits or reset the pointer without running a full generation.",
	RunE:  runStatusUpdate,
}

func init() {
	statusCmd.AddCommand(statusUpdateCmd)
	statusUpdateCmd.Flags().String("commit", "", "Commit hash to set as last processed (required)")
	statusUpdateCmd.MarkFlagRequired("commit")
}

func runStatusUpdate(cmd *cobra.Command, args []string) error {
	hash, _ := cmd.Flags().GetString("commit")
	dbPath, _ := cmd.Flags().GetString("state-db")

	// Validate the commit exists
	commit, err := git.GetCommit(".", hash)
	if err != nil {
		return fmt.Errorf("invalid commit: %w", err)
	}

	store, err := state.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.SetLastCommit(".", "blog-post", commit.Hash, commit.Timestamp); err != nil {
		return fmt.Errorf("updating state: %w", err)
	}

	fmt.Printf("Updated last processed commit to %s (%s)\n", commit.Hash[:8], commit.Subject)
	return nil
}
