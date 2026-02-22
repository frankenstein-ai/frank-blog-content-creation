package generate

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate blog posts from git commits",
}

func init() {
	Cmd.AddCommand(blogPostsCmd)
}
