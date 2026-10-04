package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the active account and verify connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := cli.Streams(cmd.Context())
		if err != nil {
			return fmt.Errorf("auth/connectivity failed: %w", err)
		}
		fmt.Fprintf(os.Stdout, "account: %s\nurl:     %s\norg:     %s\nemail:   %s\nstreams: %d\n",
			accountName(), cfg.URL, cfg.Org, cfg.Email, len(s.List))
		return nil
	},
}

func init() { rootCmd.AddCommand(whoamiCmd) }
