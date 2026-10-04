package render

import (
	"strings"
	"testing"
)

func TestLogLinesPrioritizesFieldsAndStaysConcise(t *testing.T) {
	hits := []map[string]any{{
		"_timestamp":                "2026-06-22T09:50:01Z",
		"kubernetes_namespace_name": "demo-ns",
		"kubernetes_pod_name":       "api-7c",
		"kubernetes_container_name": "api",
		"message":                   "connection refused",
		"stream":                    "stderr",
		"kubernetes_pod_id":         "noise",
	}}
	out := LogLines(hits, Fields{})
	if !strings.Contains(out, "demo-ns/api-7c[api]") || !strings.Contains(out, "connection refused") {
		t.Fatalf("missing prioritized parts:\n%s", out)
	}
	// Non-priority fields must NOT be dumped onto the line.
	if strings.Contains(out, "stream=stderr") || strings.Contains(out, "kubernetes_pod_id") {
		t.Fatalf("extra fields should not be appended:\n%s", out)
	}
}

func TestLogLinesMessageFallsBackToLog(t *testing.T) {
	out := LogLines([]map[string]any{{
		"_timestamp": "2026-06-22T09:50:01Z",
		"log":        "GET / 200",
	}}, Fields{})
	if !strings.Contains(out, "GET / 200") {
		t.Fatalf("expected log field as message:\n%s", out)
	}
}

func TestLogLinesFormatsMicrosecondTimestamp(t *testing.T) {
	// 1782129668591283 µs = 2026-06-22T12:01:08.591Z
	out := LogLines([]map[string]any{{
		"_timestamp": float64(1782129668591283),
		"log":        "hi",
	}}, Fields{})
	if !strings.Contains(out, "2026-06-22T12:01:08.591Z") {
		t.Fatalf("micros not formatted to RFC3339:\n%s", out)
	}
}

func TestCSVHeaderAndRow(t *testing.T) {
	out, err := CSV([]map[string]any{{"a": "1", "b": "2"}})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if lines[0] != "a,b" || lines[1] != "1,2" {
		t.Fatalf("csv wrong:\n%s", out)
	}
}

func TestTableAligns(t *testing.T) {
	out := Table([]string{"ts", "count"}, [][]string{{"09:50", "12"}})
	if !strings.Contains(out, "ts") || !strings.Contains(out, "count") || !strings.Contains(out, "09:50") {
		t.Fatalf("table wrong:\n%s", out)
	}
}

func TestLogLinesUsesMapping(t *testing.T) {
	hits := []map[string]any{{
		"_timestamp": "T", "svc": "billing", "host": "h1", "lvl": "warn",
		"msg": "mapped", "message": "other", "log": "x",
	}}
	out := LogLines(hits, Fields{Namespace: "svc", Pod: "host", Level: "lvl", Message: "msg"})
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "billing/h1") || !strings.HasSuffix(strings.TrimSpace(out), "mapped") {
		t.Fatalf("mapping ignored:\n%s", out)
	}
	// Mapped message field missing: falls back to the default list.
	out = LogLines([]map[string]any{{"message": "fallback"}}, Fields{Message: "nope"})
	if !strings.Contains(out, "fallback") {
		t.Fatalf("no fallback:\n%s", out)
	}
}
