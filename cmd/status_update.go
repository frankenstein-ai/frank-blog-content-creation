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
	statusUpdateCmd.Flags().String("commit", "", "Set the last processed commit to this hash")
	statusUpdateCmd.Flags().Bool("reset", false, "Clear state entirely so next run processes all commits")
}

func runStatusUpdate(cmd *cobra.Command, args []string) error {
	hash, _ := cmd.Flags().GetString("commit")
	reset, _ := cmd.Flags().GetBool("reset")

	if !reset && hash == "" {
		return fmt.Errorf("must specify either --reset or --commit <hash>")
	}
	if reset && hash != "" {
		return fmt.Errorf("--reset and --commit are mutually exclusive")
	}

	dbPath, _ := cmd.Flags().GetString("state-db")

	store, err := state.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	if reset {
		if err := store.ClearState(".", "blog-post"); err != nil {
			return fmt.Errorf("clearing state: %w", err)
		}
		fmt.Println("State cleared. Next generation will process all commits.")
		return nil
	}

	// Validate the commit exists
	commit, err := git.GetCommit(".", hash)
	if err != nil {
		return fmt.Errorf("invalid commit: %w", err)
	}

	if err := store.SetLastCommit(".", "blog-post", commit.Hash, commit.Timestamp); err != nil {
		return fmt.Errorf("updating state: %w", err)
	}

	fmt.Printf("Updated last processed commit to %s (%s)\n", commit.Hash[:8], commit.Subject)
	return nil
}
