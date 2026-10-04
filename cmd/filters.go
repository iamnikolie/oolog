package cmd

import (
	"fmt"

	"github.com/iamnikolie/oolog/internal/sqlbuild"
	"github.com/spf13/cobra"
)

// filterFlags are the log-filter flags shared by the read commands
// (search/tail/hist/top/around). --ns/--pod/--container/--level resolve to
// stream fields through the profile's `fields:` mapping, so every read command
// filters the same way on any instance.
type filterFlags struct {
	ns, pod, container, level, grep, match, where string
}

// register wires the filter flags onto a command. When withMatch is true the
// command also exposes --match (an explicit full-text term); --grep is the
// short alias every command gets.
func (f *filterFlags) register(c *cobra.Command, withMatch bool) {
	fl := c.Flags()
	fl.StringVar(&f.ns, "ns", "", "namespace filter (field: fields.namespace)")
	fl.StringVar(&f.pod, "pod", "", "pod filter (field: fields.pod)")
	fl.StringVar(&f.container, "container", "", "container filter (field: fields.container)")
	fl.StringVar(&f.level, "level", "", "level filter (field: fields.level, else full-text match_all)")
	fl.StringVar(&f.grep, "grep", "", "full-text term (match_all)")
	fl.StringVar(&f.where, "where", "", "raw SQL WHERE condition")
	if withMatch {
		fl.StringVar(&f.match, "match", "", "full-text match_all() term")
	}
}

// conds builds the SQL WHERE conditions from the set flags. It errors when a
// used flag has no mapped field, or the mapped field is absent from the
// (cached) schema of stream, instead of producing a silently empty query.
func (f *filterFlags) conds(stream string) ([]string, error) {
	fm := cfg.Fields.Resolved()
	used := []struct{ flag, key, field, val string }{
		{"ns", "namespace", fm.Namespace, f.ns},
		{"pod", "pod", fm.Pod, f.pod},
		{"container", "container", fm.Container, f.container},
		{"level", "level", fm.Level, f.level},
	}
	if _, known := streams.Get(stream); known {
		for _, u := range used {
			if u.val != "" && u.field != "" && !streams.HasField(stream, u.field) {
				return nil, fmt.Errorf("--%s: field %q not found in stream %q; set fields.%s in config (see 'oolog streams show %s', 'oolog streams sync' if stale)",
					u.flag, u.field, stream, u.key, stream)
			}
		}
	}
	m := sqlbuild.Mapping{Namespace: fm.Namespace, Pod: fm.Pod, Container: fm.Container, Level: fm.Level}
	return sqlbuild.Conds(m, f.ns, f.pod, f.container, f.level, f.grep, f.match, f.where)
}
