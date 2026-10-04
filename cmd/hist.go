package cmd

import (
	"fmt"
	"time"

	"github.com/iamnikolie/oolog/internal/render"
	"github.com/iamnikolie/oolog/internal/sqlbuild"
	"github.com/iamnikolie/oolog/internal/timex"
	"github.com/spf13/cobra"
)

var (
	hStream, hSince, hFrom, hTo, hBy string
	hFilter                          filterFlags
)

var histCmd = &cobra.Command{
	Use:   "hist",
	Short: "Count logs in time buckets (spot spikes)",
	RunE: func(cmd *cobra.Command, args []string) error {
		stream, err := streamOrDefault(hStream)
		if err != nil {
			return err
		}
		conds, err := hFilter.conds(stream)
		if err != nil {
			return err
		}
		sql := sqlbuild.Hist(stream, conds, hBy)
		start, end, err := timex.ResolveRange(hSince, hFrom, hTo, time.Now(), time.Hour)
		if err != nil {
			return err
		}
		res, err := cli.Search(cmd.Context(), sql, start, end, 1000)
		if err != nil {
			return err
		}
		return outputJSON(res.Hits, func() (string, error) {
			rows := make([][]string, 0, len(res.Hits))
			for _, h := range res.Hits {
				rows = append(rows, []string{fmt.Sprintf("%v", h["ts"]), fmt.Sprintf("%v", h["count"])})
			}
			return render.Table([]string{"bucket", "count"}, rows), nil
		})
	},
}

func init() {
	f := histCmd.Flags()
	f.StringVar(&hStream, "stream", "", "stream name (default: config `stream:`, or the only stream)")
	f.StringVar(&hSince, "since", "", "relative window (default 1h)")
	f.StringVar(&hFrom, "from", "", "RFC3339 start")
	f.StringVar(&hTo, "to", "", "RFC3339 end")
	f.StringVar(&hBy, "by", "1 minute", "bucket size, e.g. '30 second', '5 minute'")
	hFilter.register(histCmd, false)
	rootCmd.AddCommand(histCmd)
}
