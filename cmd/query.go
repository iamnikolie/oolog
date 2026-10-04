package cmd

import (
	"time"

	"github.com/spf13/cobra"
)

var (
	qSince, qFrom, qTo string
	qLimit             int
)

var queryCmd = &cobra.Command{
	Use:   "query <SQL>",
	Short: "Run a raw OpenObserve SQL query",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSQL(cmd, args[0], qSince, qFrom, qTo, qLimit, time.Hour)
	},
}

func init() {
	f := queryCmd.Flags()
	f.StringVar(&qSince, "since", "", "relative window (default 1h)")
	f.StringVar(&qFrom, "from", "", "RFC3339 start")
	f.StringVar(&qTo, "to", "", "RFC3339 end")
	f.IntVar(&qLimit, "limit", 100, "max rows")
	rootCmd.AddCommand(queryCmd)
}
