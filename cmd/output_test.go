package cmd

import (
	"strings"
	"testing"
)

func TestRenderHitsCSV(t *testing.T) {
	outFormat = "csv"
	defer func() { outFormat = "" }()
	s, err := renderHits([]map[string]any{{"a": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "a") || !strings.Contains(s, "1") {
		t.Fatalf("csv render wrong: %q", s)
	}
}

func TestRenderHitsDefaultIsLogLines(t *testing.T) {
	outFormat = ""
	s, err := renderHits([]map[string]any{{"_timestamp": "T", "message": "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "hello") {
		t.Fatalf("default render wrong: %q", s)
	}
}
