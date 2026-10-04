// Package sqlbuild assembles OpenObserve SQL strings from command flags.
// Time bounds are NOT part of the SQL — they travel as the API's
// start_time/end_time params, which keeps these builders time-independent
// and fully unit-testable.
package sqlbuild

import (
	"fmt"
	"strings"
)

// Quote wraps an identifier in double quotes, escaping embedded quotes.
func Quote(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func quoteLit(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

// Mapping names the stream fields behind the shared filter flags. An empty
// Level means --level falls back to a full-text match.
type Mapping struct {
	Namespace, Pod, Container, Level string
}

// Conds builds WHERE conditions from flag sugar, skipping empty inputs. A flag
// whose field is unmapped (empty in m) is an error, never a silent no-op.
func Conds(m Mapping, ns, pod, container, level, grep, match, where string) ([]string, error) {
	var c []string
	eq := func(flag, field, val string) error {
		if val == "" {
			return nil
		}
		if field == "" {
			return fmt.Errorf("--%s: no field mapped; set fields.%s in config", flag, flag)
		}
		c = append(c, Quote(field)+" = "+quoteLit(val))
		return nil
	}
	if err := eq("ns", m.Namespace, ns); err != nil {
		return nil, err
	}
	if err := eq("pod", m.Pod, pod); err != nil {
		return nil, err
	}
	if err := eq("container", m.Container, container); err != nil {
		return nil, err
	}
	if level != "" {
		if m.Level != "" {
			c = append(c, Quote(m.Level)+" = "+quoteLit(level))
		} else {
			c = append(c, "match_all("+quoteLit(level)+")")
		}
	}
	if grep != "" {
		c = append(c, "match_all("+quoteLit(grep)+")")
	}
	if match != "" {
		c = append(c, "match_all("+quoteLit(match)+")")
	}
	if where != "" {
		c = append(c, where)
	}
	return c, nil
}

func whereClause(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

// Search builds a newest-first (DESC) row query.
func Search(stream string, fields, conds []string) string {
	return searchOrdered(stream, fields, conds, "DESC")
}

// SearchAsc builds an oldest-first (ASC) row query — used by `around` so a
// LIMIT keeps the rows nearest the pivot rather than the window edge.
func SearchAsc(stream string, fields, conds []string) string {
	return searchOrdered(stream, fields, conds, "ASC")
}

func searchOrdered(stream string, fields, conds []string, dir string) string {
	sel := "*"
	if len(fields) > 0 {
		quoted := make([]string, len(fields))
		for i, f := range fields {
			quoted[i] = Quote(f)
		}
		sel = strings.Join(quoted, ", ")
	}
	return fmt.Sprintf("SELECT %s FROM %s%s ORDER BY _timestamp %s",
		sel, Quote(stream), whereClause(conds), dir)
}

// Hist builds a time-bucketed count query. GROUP BY/ORDER BY are positional:
// OpenObserve auto-injects _timestamp for type=logs queries, and grouping by the
// "ts" alias then trips DataFusion's GROUP-BY check — positional 1 avoids it.
func Hist(stream string, conds []string, bucket string) string {
	return fmt.Sprintf(
		"SELECT histogram(_timestamp, '%s') AS ts, count(*) AS count FROM %s%s GROUP BY 1 ORDER BY 1",
		bucket, Quote(stream), whereClause(conds))
}

// Top builds a GROUP BY count query for the most frequent values of a field.
func Top(stream, field string, conds []string, limit int) string {
	return fmt.Sprintf(
		"SELECT %s AS value, count(*) AS count FROM %s%s GROUP BY value ORDER BY count DESC LIMIT %d",
		Quote(field), Quote(stream), whereClause(conds), limit)
}
