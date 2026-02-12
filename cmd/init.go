package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set starting commit point for content generation",
	Long:  "Initialize the processing state so that future generation runs only process commits after the specified one.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().String("source-repo", "", "Path to source git repository (env: FRANK_SOURCE_REPO)")
	initCmd.Flags().String("commit", "", "Git commit hash to mark as already processed")
	initCmd.Flags().String("content-type", "", "Target a specific content type: notebook, memo, or blog-post (default: all)")
	rootCmd.AddCommand(initCmd)
}

var allContentTypes = []string{"notebook", "memo", "blog-post"}

func runInit(cmd *cobra.Command, args []string) error {
	sourceRepo := flagOrEnvInit(cmd, "source-repo", "FRANK_SOURCE_REPO")
	commitHash, _ := cmd.Flags().GetString("commit")
	contentType, _ := cmd.Flags().GetString("content-type")
	dbPath, _ := cmd.Flags().GetString("state-db")

	if sourceRepo == "" {
		return fmt.Errorf("--source-repo or FRANK_SOURCE_REPO is required")
	}
	if commitHash == "" {
		return fmt.Errorf("--commit is required")
	}

	// Validate content-type if provided
	if contentType != "" {
		valid := false
		for _, ct := range allContentTypes {
			if ct == contentType {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("--content-type must be one of: notebook, memo, blog-post (got %q)", contentType)
		}
	}

	// Resolve source repo to absolute path
	absRepo, err := filepath.Abs(sourceRepo)
	if err != nil {
		return fmt.Errorf("resolving source repo path: %w", err)
	}

	// Validate the commit exists
	commit, err := git.GetCommit(absRepo, commitHash)
	if err != nil {
		return err
	}

	// Open state DB
	store, err := state.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	// Determine which content types to initialize
	types := allContentTypes
	if contentType != "" {
		types = []string{contentType}
	}

	shortHash := commit.Hash
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}

	for _, ct := range types {
		if err := store.SetLastCommit(absRepo, ct, commit.Hash, commit.Timestamp); err != nil {
			return fmt.Errorf("setting state for %s: %w", ct, err)
		}
		fmt.Printf("Initialized %s → commit %s (%s)\n", ct, shortHash, commit.Timestamp.Format("2006-01-02"))
	}

	return nil
}

func flagOrEnvInit(cmd *cobra.Command, flag, env string) string {
	if cmd.Flags().Changed(flag) {
		v, _ := cmd.Flags().GetString(flag)
		return v
	}
	return os.Getenv(env)
}
