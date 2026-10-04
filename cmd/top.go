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
	pStream, pSince, pFrom, pTo string
	pN                          int
	pFilter                     filterFlags
)

var topCmd = &cobra.Command{
	Use:   "top <field>",
	Short: "Top-N values of a field by count",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		stream, err := streamOrDefault(pStream)
		if err != nil {
			return err
		}
		conds, err := pFilter.conds(stream)
		if err != nil {
			return err
		}
		sql := sqlbuild.Top(stream, args[0], conds, pN)
		start, end, err := timex.ResolveRange(pSince, pFrom, pTo, time.Now(), time.Hour)
		if err != nil {
			return err
		}
		res, err := cli.Search(cmd.Context(), sql, start, end, pN)
		if err != nil {
			return err
		}
		return outputJSON(res.Hits, func() (string, error) {
			rows := make([][]string, 0, len(res.Hits))
			for _, h := range res.Hits {
				rows = append(rows, []string{fmt.Sprintf("%v", h["value"]), fmt.Sprintf("%v", h["count"])})
			}
			return render.Table([]string{"value", "count"}, rows), nil
		})
	},
}

func init() {
	f := topCmd.Flags()
	f.StringVar(&pStream, "stream", "", "stream name (default: config `stream:`, or the only stream)")
	f.StringVar(&pSince, "since", "", "relative window (default 1h)")
	f.StringVar(&pFrom, "from", "", "RFC3339 start")
	f.StringVar(&pTo, "to", "", "RFC3339 end")
	f.IntVarP(&pN, "limit", "n", 20, "number of values")
	pFilter.register(topCmd, false)
	rootCmd.AddCommand(topCmd)
}
