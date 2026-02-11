package cmd

import (
	"fmt"

	"github.com/frankenstein-ai/frank-blog-content/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show last processed commit per source repo",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	dbPath, _ := cmd.Flags().GetString("state-db")

	store, err := state.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	rows, err := store.GetAllState()
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		fmt.Println("No processing state found. Run 'frank generate' first.")
		return nil
	}

	fmt.Printf("%-40s %-12s %-10s %-20s %-20s\n", "SOURCE REPO", "TYPE", "COMMIT", "TIMESTAMP", "UPDATED")
	fmt.Println("-----------------------------------------------------------------------------------------------------------")
	for _, r := range rows {
		hash := r["last_commit"]
		if len(hash) > 8 {
			hash = hash[:8]
		}
		fmt.Printf("%-40s %-12s %-10s %-20s %-20s\n",
			r["source_repo"], r["content_type"], hash, r["timestamp"], r["updated_at"])
	}

	return nil
}
