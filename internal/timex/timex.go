// Package timex parses relative ("15m", "3d") and RFC3339 time inputs into
// OpenObserve's microsecond timestamps.
package timex

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseDuration extends time.ParseDuration with a "d" (day) unit.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		if err != nil {
			return 0, fmt.Errorf("bad duration %q: %w", s, err)
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("bad duration %q: %w", s, err)
	}
	return d, nil
}

// ToMicros converts a time to microseconds since the Unix epoch.
func ToMicros(t time.Time) int64 { return t.UnixMicro() }

// ResolveRange returns [start, end] in microseconds. Explicit from/to (RFC3339)
// win; otherwise end=now and start=now-since (or now-def when since is empty).
func ResolveRange(since, from, to string, now time.Time, def time.Duration) (int64, int64, error) {
	if from != "" || to != "" {
		start, end := now.Add(-def), now
		if from != "" {
			t, err := time.Parse(time.RFC3339, from)
			if err != nil {
				return 0, 0, fmt.Errorf("bad --from %q: %w", from, err)
			}
			start = t
		}
		if to != "" {
			t, err := time.Parse(time.RFC3339, to)
			if err != nil {
				return 0, 0, fmt.Errorf("bad --to %q: %w", to, err)
			}
			end = t
		}
		return ToMicros(start), ToMicros(end), nil
	}
	window := def
	if since != "" {
		d, err := ParseDuration(since)
		if err != nil {
			return 0, 0, err
		}
		window = d
	}
	return ToMicros(now.Add(-window)), ToMicros(now), nil
}
