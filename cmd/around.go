package cmd

import (
	"fmt"
	"time"

	"github.com/iamnikolie/oolog/internal/sqlbuild"
	"github.com/iamnikolie/oolog/internal/timex"
	"github.com/spf13/cobra"
)

var (
	aStream         string
	aBefore, aAfter int
	aWindow         string
	aFilter         filterFlags
)

var aroundCmd = &cobra.Command{
	Use:   "around <rfc3339-timestamp>",
	Short: "Show log lines surrounding a moment in time",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		at, err := time.Parse(time.RFC3339, args[0])
		if err != nil {
			return fmt.Errorf("bad timestamp %q (want RFC3339): %w", args[0], err)
		}
		win, err := timex.ParseDuration(aWindow)
		if err != nil {
			return err
		}
		stream, err := streamOrDefault(aStream)
		if err != nil {
			return err
		}
		conds, err := aFilter.conds(stream)
		if err != nil {
			return err
		}
		sqlDesc := sqlbuild.Search(stream, nil, conds)
		sqlAsc := sqlbuild.SearchAsc(stream, nil, conds)

		// after: the aAfter rows immediately following the pivot. Ascending order
		// makes the LIMIT keep the rows NEAREST the pivot (not the window edge),
		// already chronological. +1µs excludes the pivot row (it belongs to the
		// before half).
		after, err := cli.Search(cmd.Context(), sqlAsc, timex.ToMicros(at)+1, timex.ToMicros(at.Add(win)), aAfter)
		if err != nil {
			return err
		}
		// before: the aBefore rows up to and including the pivot (end_time is
		// exclusive, hence +1µs). Descending +
		// LIMIT keeps the nearest rows; reverse to chronological.
		before, err := cli.Search(cmd.Context(), sqlDesc, timex.ToMicros(at.Add(-win)), timex.ToMicros(at)+1, aBefore)
		if err != nil {
			return err
		}
		reverse(before.Hits)
		hits := append(before.Hits, after.Hits...)
		return outputJSON(hits, func() (string, error) { return renderHits(hits) })
	},
}

func init() {
	f := aroundCmd.Flags()
	f.StringVar(&aStream, "stream", "", "stream name (default: config `stream:`, or the only stream)")
	f.IntVar(&aBefore, "before", 5, "rows before the moment")
	f.IntVar(&aAfter, "after", 5, "rows after the moment")
	f.StringVar(&aWindow, "window", "5m", "max lookaround window each side")
	aFilter.register(aroundCmd, false)
	rootCmd.AddCommand(aroundCmd)
}

// reverse flips a slice of hits in place, turning OpenObserve's newest-first
// (DESC) order into chronological order for the surrounding-context view.
func reverse(hits []map[string]any) {
	for i, j := 0, len(hits)-1; i < j; i, j = i+1, j-1 {
		hits[i], hits[j] = hits[j], hits[i]
	}
}
