package sqlbuild

import (
	"reflect"
	"strings"
	"testing"
)

func TestConds(t *testing.T) {
	m := Mapping{Namespace: "kubernetes_namespace_name", Pod: "kubernetes_pod_name", Container: "kubernetes_container_name"}
	got, err := Conds(m, "app-ns", "", "api", "warn", "", "timeout", "stream = 'stderr'")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`"kubernetes_namespace_name" = 'app-ns'`,
		`"kubernetes_container_name" = 'api'`,
		"match_all('warn')",
		"match_all('timeout')",
		"stream = 'stderr'",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Conds = %#v\nwant %#v", got, want)
	}
}

func TestCondsLevelMapped(t *testing.T) {
	got, err := Conds(Mapping{Level: "lvl"}, "", "", "", "error", "", "", "")
	if err != nil || !reflect.DeepEqual(got, []string{`"lvl" = 'error'`}) {
		t.Fatalf("got %#v err %v", got, err)
	}
}

func TestCondsUnmappedErrors(t *testing.T) {
	if _, err := Conds(Mapping{}, "x", "", "", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "fields.ns") {
		t.Fatalf("want unmapped error, got %v", err)
	}
}

func TestCondsQuotesLiteral(t *testing.T) {
	got, _ := Conds(Mapping{Pod: "p"}, "", "o'brien", "", "", "", "", "")
	if got[0] != `"p" = 'o''brien'` {
		t.Fatalf("got %q", got[0])
	}
}

func TestSearchNoConds(t *testing.T) {
	got := Search("k8s", nil, nil)
	want := `SELECT * FROM "k8s" ORDER BY _timestamp DESC`
	if got != want {
		t.Fatalf("Search = %q\nwant %q", got, want)
	}
}

func TestSearchWithFieldsAndConds(t *testing.T) {
	got := Search("k8s", []string{"_timestamp", "message"}, []string{"match_all('boom')"})
	want := `SELECT "_timestamp", "message" FROM "k8s" WHERE match_all('boom') ORDER BY _timestamp DESC`
	if got != want {
		t.Fatalf("Search = %q\nwant %q", got, want)
	}
}

func TestHist(t *testing.T) {
	got := Hist("k8s", []string{"match_all('error')"}, "1 minute")
	want := `SELECT histogram(_timestamp, '1 minute') AS ts, count(*) AS count FROM "k8s" WHERE match_all('error') GROUP BY 1 ORDER BY 1`
	if got != want {
		t.Fatalf("Hist = %q\nwant %q", got, want)
	}
}

func TestTop(t *testing.T) {
	got := Top("k8s", "kubernetes_namespace_name", nil, 20)
	want := `SELECT "kubernetes_namespace_name" AS value, count(*) AS count FROM "k8s" GROUP BY value ORDER BY count DESC LIMIT 20`
	if got != want {
		t.Fatalf("Top = %q\nwant %q", got, want)
	}
}

func TestSearchAsc(t *testing.T) {
	got := SearchAsc("k8s", nil, nil)
	want := `SELECT * FROM "k8s" ORDER BY _timestamp ASC`
	if got != want {
		t.Fatalf("SearchAsc = %q\nwant %q", got, want)
	}
}
