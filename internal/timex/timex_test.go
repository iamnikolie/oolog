package timex

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"90s": 90 * time.Second,
		"15m": 15 * time.Minute,
		"2h":  2 * time.Hour,
		"3d":  3 * 24 * time.Hour,
	}
	for in, want := range cases {
		got, err := ParseDuration(in)
		if err != nil || got != want {
			t.Fatalf("ParseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseDuration("nonsense"); err == nil {
		t.Fatal("expected error for bad duration")
	}
}

func TestResolveRangeSince(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	start, end, err := ResolveRange("15m", "", "", now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if end != ToMicros(now) {
		t.Fatalf("end = %d, want %d", end, ToMicros(now))
	}
	if start != ToMicros(now.Add(-15*time.Minute)) {
		t.Fatalf("start = %d, want %d", start, ToMicros(now.Add(-15*time.Minute)))
	}
}

func TestResolveRangeDefault(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	start, _, err := ResolveRange("", "", "", now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if start != ToMicros(now.Add(-time.Hour)) {
		t.Fatalf("start = %d, want default 1h back", start)
	}
}

func TestResolveRangeFromTo(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	start, end, err := ResolveRange("", "2026-06-22T10:00:00Z", "2026-06-22T11:00:00Z", now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if start != ToMicros(time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("start wrong: %d", start)
	}
	if end != ToMicros(time.Date(2026, 6, 22, 11, 0, 0, 0, time.UTC)) {
		t.Fatalf("end wrong: %d", end)
	}
}
