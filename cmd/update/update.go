package update

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "update",
	Short: "Update Hugo site configuration",
}

func init() {
	Cmd.AddCommand(menuCmd)
	Cmd.AddCommand(homeCmd)
	Cmd.AddCommand(skillCmd)
}
