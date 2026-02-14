package update

import (
	"fmt"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/config"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/hugo"
	"github.com/spf13/cobra"
)

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Update Hugo menu with the latest blog post",
	RunE:  runMenu,
}

func runMenu(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cmd)
	if err != nil {
		return err
	}

	if cfg.HugoDir == "" {
		return fmt.Errorf("--hugo-dir, FRANK_HUGO_DIR, or hugo_dir in .frank.toml is required")
	}

	post, err := hugo.FindLatestPost(cfg.HugoDir)
	if err != nil {
		return err
	}

	name := "Latest: " + post.Title
	pageRef := "/posts/" + post.Slug

	if err := hugo.UpdateMenuEntry(cfg.HugoDir, name, pageRef); err != nil {
		return err
	}

	fmt.Printf("Menu updated: %s → %s\n", name, pageRef)
	return nil
}
