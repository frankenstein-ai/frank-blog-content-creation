package generate

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate content from git commits",
}

func init() {
	Cmd.AddCommand(notebooksCmd)
	Cmd.AddCommand(memosCmd)
	Cmd.AddCommand(notesCmd)
	Cmd.AddCommand(blogPostsCmd)
	Cmd.AddCommand(homepageCmd)
}
