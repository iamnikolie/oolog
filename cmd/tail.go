package cmd

import (
	"time"

	"github.com/iamnikolie/oolog/internal/sqlbuild"
	"github.com/spf13/cobra"
)

var (
	tStream, tSince string
	tN              int
	tFilter         filterFlags
)

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Show the most recent log lines (newest first)",
	RunE: func(cmd *cobra.Command, args []string) error {
		stream, err := streamOrDefault(tStream)
		if err != nil {
			return err
		}
		conds, err := tFilter.conds(stream)
		if err != nil {
			return err
		}
		sql := sqlbuild.Search(stream, nil, conds)
		return runSQL(cmd, sql, tSince, "", "", tN, 15*time.Minute)
	},
}

func init() {
	f := tailCmd.Flags()
	f.StringVar(&tStream, "stream", "", "stream name (default: config `stream:`, or the only stream)")
	f.StringVar(&tSince, "since", "", "relative window (default 15m)")
	f.IntVarP(&tN, "lines", "n", 50, "number of lines")
	tFilter.register(tailCmd, false)
	rootCmd.AddCommand(tailCmd)
}
