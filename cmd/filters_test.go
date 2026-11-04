package cmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/iamnikolie/oolog/internal/config"
	"github.com/iamnikolie/oolog/internal/sqlbuild"
)

func withCfg(t *testing.T, c config.Config) {
	t.Helper()
	prev := cfg
	cfg = c
	t.Cleanup(func() { cfg = prev })
}

func TestCondsUnscopedUnchanged(t *testing.T) {
	withCfg(t, config.Config{})
	f := &filterFlags{ns: "any", where: "x = 1"}
	got, err := f.conds("k8s")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`"kubernetes_namespace_name" = 'any'`, "x = 1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestCondsScopedPrependsAllowlist(t *testing.T) {
	withCfg(t, config.Config{Namespaces: []string{"shop", "blog"}})
	f := &filterFlags{where: "a = 1 OR b = 2"}
	got, err := f.conds("k8s")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		sqlbuild.In("kubernetes_namespace_name", []string{"shop", "blog"}),
		"(a = 1 OR b = 2)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	if sql := sqlbuild.Search("k8s", nil, got); !strings.Contains(sql, `WHERE "kubernetes_namespace_name" IN ('shop', 'blog') AND (a = 1 OR b = 2)`) {
		t.Fatalf("sql = %s", sql)
	}
}

func TestCondsScopedRejects(t *testing.T) {
	withCfg(t, config.Config{Namespaces: []string{"shop"}})
	for _, f := range []*filterFlags{
		{ns: "other"},
		{where: "1=1) OR (1=1"},
		{where: "1=1 -- x"},
	} {
		if _, err := f.conds("k8s"); err == nil {
			t.Errorf("conds(%+v) = nil error, want refusal", *f)
		}
	}
	if _, err := (&filterFlags{ns: "shop"}).conds("k8s"); err != nil {
		t.Fatalf("allowed ns refused: %v", err)
	}
}

func TestRawCommandsRefusedWhenScoped(t *testing.T) {
	withCfg(t, config.Config{Namespaces: []string{"shop"}})
	if err := refuseScoped("query"); err == nil || !strings.Contains(err.Error(), "shop") {
		t.Fatalf("refuseScoped = %v", err)
	}
	withCfg(t, config.Config{})
	if err := refuseScoped("query"); err != nil {
		t.Fatalf("unscoped refused: %v", err)
	}
}
