package cmd

import (
	"time"

	"github.com/iamnikolie/oolog/internal/sqlbuild"
	"github.com/iamnikolie/oolog/internal/timex"
	"github.com/spf13/cobra"
)

var (
	sStream, sSince, sFrom, sTo string
	sFields                     []string
	sLimit                      int
	sFilter                     filterFlags
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search logs with filters and a time range",
	RunE: func(cmd *cobra.Command, args []string) error {
		stream, err := streamOrDefault(sStream)
		if err != nil {
			return err
		}
		conds, err := sFilter.conds(stream)
		if err != nil {
			return err
		}
		sql := sqlbuild.Search(stream, sFields, conds)
		return runSQL(cmd, sql, sSince, sFrom, sTo, sLimit, 15*time.Minute)
	},
}

// streamOrDefault resolves the stream: --stream, else config `stream:`, else
// the only stream on the instance, else an error listing the streams.
func streamOrDefault(s string) (string, error) {
	return streams.Resolve(s, cfg.Stream)
}

// runSQL resolves the time range, runs the search, and renders hits.
func runSQL(cmd *cobra.Command, sql, since, from, to string, limit int, def time.Duration) error {
	start, end, err := timex.ResolveRange(since, from, to, time.Now(), def)
	if err != nil {
		return err
	}
	res, err := cli.Search(cmd.Context(), sql, start, end, limit)
	if err != nil {
		return err
	}
	return outputJSON(res.Hits, func() (string, error) { return renderHits(res.Hits) })
}

func init() {
	f := searchCmd.Flags()
	f.StringVar(&sStream, "stream", "", "stream name (default: config `stream:`, or the only stream)")
	f.StringVar(&sSince, "since", "", "relative window, e.g. 15m, 2h, 3d (default 15m)")
	f.StringVar(&sFrom, "from", "", "RFC3339 start (overrides --since)")
	f.StringVar(&sTo, "to", "", "RFC3339 end")
	f.StringSliceVar(&sFields, "fields", nil, "explicit columns to select")
	f.IntVar(&sLimit, "limit", 100, "max rows")
	sFilter.register(searchCmd, true)
	rootCmd.AddCommand(searchCmd)
}
