// Package render formats OpenObserve hits for humans, agents, and spreadsheets.
package render

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// messageKeys are field names that may carry the log line, in priority order.
// fluent-bit writes the message to "log"; other shippers use "message"/"msg"/"body".
var messageKeys = []string{"message", "log", "msg", "body"}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// toInt64 reports a numeric value as int64 (JSON numbers decode to float64).
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case string:
		if i, err := strconv.ParseInt(n, 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}

// tsString renders a hit's timestamp. OpenObserve's _timestamp is microseconds
// since the epoch; an already-formatted string passes through, and a "time"
// field is the fallback.
func tsString(h map[string]any) string {
	if v, ok := h["_timestamp"]; ok {
		if us, ok := toInt64(v); ok {
			return time.UnixMicro(us).UTC().Format("2006-01-02T15:04:05.000Z")
		}
		return str(v)
	}
	return str(h["time"])
}

// Fields names the stream fields render should read. Empty values use the
// Kubernetes-collector names (namespace/pod/container) or "level".
type Fields struct {
	Namespace, Pod, Container, Level, Message string
}

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func message(h map[string]any, configured string) string {
	if configured != "" {
		if s := str(h[configured]); s != "" {
			return s
		}
	}
	for _, k := range messageKeys {
		if s := str(h[k]); s != "" {
			return s
		}
	}
	return ""
}

// LogLines renders one concise, prioritized line per hit:
//
//	<timestamp>  LEVEL  <ns>/<pod>[<container>]  <message>
//
// Only these fields are shown — use --fields or --json to see the rest.
func LogLines(hits []map[string]any, f Fields) string {
	var b strings.Builder
	for _, h := range hits {
		loc := str(h[orDefault(f.Namespace, "kubernetes_namespace_name")])
		if pod := str(h[orDefault(f.Pod, "kubernetes_pod_name")]); pod != "" {
			loc += "/" + pod
		}
		if cont := str(h[orDefault(f.Container, "kubernetes_container_name")]); cont != "" {
			loc += "[" + cont + "]"
		}

		var parts []string
		if ts := tsString(h); ts != "" {
			parts = append(parts, ts)
		}
		if level := str(h[orDefault(f.Level, "level")]); level != "" {
			parts = append(parts, strings.ToUpper(level))
		}
		if loc != "" {
			parts = append(parts, loc)
		}
		if msg := message(h, f.Message); msg != "" {
			parts = append(parts, msg)
		}
		b.WriteString(strings.Join(parts, "  "))
		b.WriteByte('\n')
	}
	return b.String()
}

func keys(hits []map[string]any) []string {
	set := map[string]bool{}
	for _, h := range hits {
		for k := range h {
			set[k] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func delimited(hits []map[string]any, comma rune) (string, error) {
	cols := keys(hits)
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	w.Comma = comma
	if err := w.Write(cols); err != nil {
		return "", err
	}
	for _, h := range hits {
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = str(h[c])
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}
	w.Flush()
	return sb.String(), w.Error()
}

// CSV renders hits as comma-separated values.
func CSV(hits []map[string]any) (string, error) { return delimited(hits, ',') }

// TSV renders hits as tab-separated values.
func TSV(hits []map[string]any) (string, error) { return delimited(hits, '\t') }

// Table renders space-aligned columns.
func Table(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if i < len(widths) && len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}
	var b strings.Builder
	writeRow := func(cells []string) {
		for i, c := range cells {
			b.WriteString(c)
			if i < len(cells)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-len(c)+2))
			}
		}
		b.WriteByte('\n')
	}
	writeRow(headers)
	for _, r := range rows {
		writeRow(r)
	}
	return b.String()
}
