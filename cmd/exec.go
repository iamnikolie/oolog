package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec <json-body>",
	Short: "POST a raw search body to /api/<org>/_search",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := refuseScoped("exec"); err != nil {
			return err
		}
		b, err := cli.Exec(cmd.Context(), []byte(args[0]))
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(b))
		return nil
	},
}

func init() { rootCmd.AddCommand(execCmd) }
