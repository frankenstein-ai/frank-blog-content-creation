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
	Long: `Initialize the processing state so that future generation runs only process
commits after the specified ones.

Two independent tracks can be initialized together or separately:
  - Source track (--source-repo + --commit): sets starting point for notebooks and memos
  - Blog track (--blog-repo + --blog-commit): sets starting point for blog posts`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().String("source-repo", "", "Path to source git repository (env: FRANK_SOURCE_REPO)")
	initCmd.Flags().String("commit", "", "Starting commit for notebook + memo generation")
	initCmd.Flags().String("blog-repo", "", "Path to blog content git repository (env: FRANK_BLOG_REPO)")
	initCmd.Flags().String("blog-commit", "", "Starting commit for blog-post generation")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	sourceRepo := flagOrEnvInit(cmd, "source-repo", "FRANK_SOURCE_REPO")
	commitHash, _ := cmd.Flags().GetString("commit")
	blogRepo := flagOrEnvInit(cmd, "blog-repo", "FRANK_BLOG_REPO")
	blogCommit, _ := cmd.Flags().GetString("blog-commit")
	dbPath, _ := cmd.Flags().GetString("state-db")

	hasSource := sourceRepo != "" || commitHash != ""
	hasBlog := blogRepo != "" || blogCommit != ""

	if !hasSource && !hasBlog {
		return fmt.Errorf("at least one track required: --source-repo + --commit, or --blog-repo + --blog-commit")
	}

	// Validate flags come in pairs
	if hasSource {
		if sourceRepo == "" {
			return fmt.Errorf("--source-repo (or FRANK_SOURCE_REPO) is required when --commit is provided")
		}
		if commitHash == "" {
			return fmt.Errorf("--commit is required when --source-repo is provided")
		}
	}
	if hasBlog {
		if blogRepo == "" {
			return fmt.Errorf("--blog-repo (or FRANK_BLOG_REPO) is required when --blog-commit is provided")
		}
		if blogCommit == "" {
			return fmt.Errorf("--blog-commit is required when --blog-repo is provided")
		}
	}

	// Open state DB
	store, err := state.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	// Source track: initialize notebook + memo
	if hasSource {
		absRepo, err := filepath.Abs(sourceRepo)
		if err != nil {
			return fmt.Errorf("resolving source repo path: %w", err)
		}
		commit, err := git.GetCommit(absRepo, commitHash)
		if err != nil {
			return err
		}
		short := commit.Hash
		if len(short) > 8 {
			short = short[:8]
		}
		for _, ct := range []string{"notebook", "memo"} {
			if err := store.SetLastCommit(absRepo, ct, commit.Hash, commit.Timestamp); err != nil {
				return fmt.Errorf("setting state for %s: %w", ct, err)
			}
			fmt.Printf("Initialized %s → commit %s (%s)\n", ct, short, commit.Timestamp.Format("2006-01-02"))
		}
	}

	// Blog track: initialize blog-post
	if hasBlog {
		absRepo, err := filepath.Abs(blogRepo)
		if err != nil {
			return fmt.Errorf("resolving blog repo path: %w", err)
		}
		commit, err := git.GetCommit(absRepo, blogCommit)
		if err != nil {
			return err
		}
		short := commit.Hash
		if len(short) > 8 {
			short = short[:8]
		}
		if err := store.SetLastCommit(absRepo, "blog-post", commit.Hash, commit.Timestamp); err != nil {
			return fmt.Errorf("setting state for blog-post: %w", err)
		}
		fmt.Printf("Initialized blog-post → commit %s (%s)\n", short, commit.Timestamp.Format("2006-01-02"))
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
