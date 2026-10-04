package cmd

import (
	"fmt"
	"os"

	"github.com/iamnikolie/oolog/internal/cache"
	"github.com/iamnikolie/oolog/internal/render"
	"github.com/spf13/cobra"
)

var streamsCmd = &cobra.Command{Use: "streams", Short: "Inspect log streams and their schema"}

var streamsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cached streams",
	RunE: func(cmd *cobra.Command, args []string) error {
		return outputJSON(streams, func() (string, error) {
			rows := make([][]string, 0, len(streams.List))
			for _, s := range streams.List {
				rows = append(rows, []string{s.Name, fmt.Sprintf("%d fields", len(s.Fields))})
			}
			return render.Table([]string{"stream", "schema"}, rows), nil
		})
	},
}

var streamsShowCmd = &cobra.Command{
	Use:   "show <stream>",
	Short: "Show a stream's fields",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		st, ok := streams.Get(args[0])
		if !ok {
			return fmt.Errorf("unknown stream %q; run 'oolog streams sync'", args[0])
		}
		return outputJSON(st, func() (string, error) {
			rows := make([][]string, 0, len(st.Fields))
			for _, f := range st.Fields {
				rows = append(rows, []string{f.Name, f.Type})
			}
			return render.Table([]string{"field", "type"}, rows), nil
		})
	},
}

var streamsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Refresh the streams cache from OpenObserve",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := cli.Streams(cmd.Context())
		if err != nil {
			return err
		}
		if err := cache.Save(s, account); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "synced %d streams\n", len(s.List))
		return nil
	},
}

func init() {
	streamsCmd.AddCommand(streamsListCmd, streamsShowCmd, streamsSyncCmd)
	rootCmd.AddCommand(streamsCmd)
}
