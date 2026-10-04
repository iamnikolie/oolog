package cache

import (
	"strings"
	"testing"
)

func fixture() Streams {
	return Streams{List: []Stream{
		{Name: "app", Fields: []Field{{Name: "message", Type: "Utf8"}}},
		{Name: "k8s", Fields: []Field{
			{Name: "_timestamp", Type: "Int64"},
			{Name: "kubernetes_namespace_name", Type: "Utf8"},
		}},
	}}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("OOLOG_HOME", t.TempDir())
	if err := Save(fixture(), ""); err != nil {
		t.Fatal(err)
	}
	got, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.List) != 2 {
		t.Fatalf("want 2 streams, got %d", len(got.List))
	}
}

func TestResolveStream(t *testing.T) {
	two := fixture()
	one := Streams{List: two.List[:1]}
	if got, err := two.Resolve("flag", "cfg"); err != nil || got != "flag" {
		t.Fatalf("flag wins: %q %v", got, err)
	}
	if got, err := two.Resolve("", "cfg"); err != nil || got != "cfg" {
		t.Fatalf("config wins over cache: %q %v", got, err)
	}
	if got, err := one.Resolve("", ""); err != nil || got != "app" {
		t.Fatalf("single stream fallback: %q %v", got, err)
	}
	_, err := two.Resolve("", "")
	if err == nil || !strings.Contains(err.Error(), "app, k8s") || !strings.Contains(err.Error(), "stream:") {
		t.Fatalf("want listing error, got %v", err)
	}
	if _, err := (Streams{}).Resolve("", ""); err == nil {
		t.Fatal("want error for no streams")
	}
}

func TestHasField(t *testing.T) {
	s := fixture()
	if !s.HasField("k8s", "kubernetes_namespace_name") {
		t.Fatal("expected k8s to have kubernetes_namespace_name")
	}
	if s.HasField("k8s", "nope") {
		t.Fatal("did not expect field nope")
	}
}

func TestNames(t *testing.T) {
	got := fixture().Names()
	if len(got) != 2 || got[0] != "app" || got[1] != "k8s" {
		t.Fatalf("Names() = %v", got)
	}
}
