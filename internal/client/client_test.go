package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iamnikolie/oolog/internal/config"
)

func TestBuildSearchBody(t *testing.T) {
	b := buildSearchBody("SELECT * FROM \"k8s\"", 100, 200, 50)
	var got map[string]map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	q := got["query"]
	if q["sql"] != "SELECT * FROM \"k8s\"" || q["start_time"].(float64) != 100 ||
		q["end_time"].(float64) != 200 || q["size"].(float64) != 50 {
		t.Fatalf("body wrong: %s", b)
	}
}

func TestParseSearchResponse(t *testing.T) {
	raw := []byte(`{"took":12,"total":2,"hits":[{"message":"a"},{"message":"b"}]}`)
	r, err := parseSearchResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 2 || len(r.Hits) != 2 || r.Hits[0]["message"] != "a" {
		t.Fatalf("parsed wrong: %+v", r)
	}
}

func TestParseStreams(t *testing.T) {
	raw := []byte(`{"list":[{"name":"k8s","schema":[{"name":"message","type":"Utf8"}]}]}`)
	s, err := parseStreams(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.List) != 1 || s.List[0].Name != "k8s" || s.List[0].Fields[0].Name != "message" {
		t.Fatalf("parsed wrong: %+v", s)
	}
}

func TestSearchSendsAuthAndUnwraps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := strings.TrimPrefix(r.Header.Get("Authorization"), "Basic ")
		dec, _ := base64.StdEncoding.DecodeString(enc)
		if string(dec) != "e:p" {
			t.Errorf("auth credential = %q, want e:p", dec)
		}
		if r.URL.Path != "/api/default/_search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("type") != "logs" {
			t.Errorf("type query = %q, want logs", r.URL.Query().Get("type"))
		}
		w.Write([]byte(`{"took":1,"total":1,"hits":[{"message":"hi"}]}`))
	}))
	defer srv.Close()

	c := New(config.Config{URL: srv.URL, Org: "default", Email: "e", Password: "p"}, false)
	res, err := c.Search(context.Background(), "SELECT * FROM \"k8s\"", 1, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 1 || res.Hits[0]["message"] != "hi" {
		t.Fatalf("unexpected: %+v", res)
	}
}
