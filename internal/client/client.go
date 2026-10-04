// Package client is the single place that talks HTTP to OpenObserve.
package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/iamnikolie/oolog/internal/cache"
	"github.com/iamnikolie/oolog/internal/config"
	"github.com/iamnikolie/oolog/internal/tunnel"
)

// Client targets one OpenObserve org with basic-auth credentials.
type Client struct {
	cfg     config.Config
	http    *http.Client
	verbose bool
	initErr error // invalid ssh block; surfaced on the first request
}

// SearchResult is the unwrapped form of an OpenObserve search response.
type SearchResult struct {
	Hits  []map[string]any
	Total int
	Took  int
}

// New builds a client from config.
// With an ssh block, requests go through a lazily opened SSH tunnel; the
// connection lives until the process exits.
func New(c config.Config, verbose bool) *Client {
	cl := &Client{cfg: c, http: &http.Client{Timeout: 60 * time.Second}, verbose: verbose}
	if c.SSH != nil {
		d, err := tunnel.New(*c.SSH)
		if err != nil {
			cl.initErr = err
			return cl
		}
		cl.http.Transport = &http.Transport{DialContext: d.DialContext}
	}
	return cl
}

func (c *Client) authHeader() string {
	tok := base64.StdEncoding.EncodeToString([]byte(c.cfg.Email + ":" + c.cfg.Password))
	return "Basic " + tok
}

// do performs an HTTP request with basic auth and 429 retry (3 attempts).
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if c.initErr != nil {
		return nil, c.initErr
	}
	full := c.cfg.URL + path
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, full, rdr)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", c.authHeader())
		req.Header.Set("Content-Type", "application/json")
		if c.verbose {
			fmt.Fprintf(os.Stderr, "%s %s\n%s\n", method, full, body)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			last = err
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if c.verbose {
			fmt.Fprintf(os.Stderr, "%d %s\n", resp.StatusCode, b)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			select {
			case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			last = fmt.Errorf("429 rate limited")
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, b)
		}
		return b, nil
	}
	return nil, last
}

func buildSearchBody(sql string, start, end int64, size int) []byte {
	body := map[string]any{"query": map[string]any{
		"sql":        sql,
		"start_time": start,
		"end_time":   end,
		"from":       0,
		"size":       size,
	}}
	b, _ := json.Marshal(body)
	return b
}

func parseSearchResponse(b []byte) (SearchResult, error) {
	var r struct {
		Took  int              `json:"took"`
		Total int              `json:"total"`
		Hits  []map[string]any `json:"hits"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Hits: r.Hits, Total: r.Total, Took: r.Took}, nil
}

func parseStreams(b []byte) (cache.Streams, error) {
	var raw struct {
		List []struct {
			Name   string        `json:"name"`
			Schema []cache.Field `json:"schema"`
		} `json:"list"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return cache.Streams{}, err
	}
	out := cache.Streams{}
	for _, s := range raw.List {
		out.List = append(out.List, cache.Stream{Name: s.Name, Fields: s.Schema})
	}
	return out, nil
}

// Search runs an SQL query over the org within [start,end] microseconds.
func (c *Client) Search(ctx context.Context, sql string, start, end int64, size int) (SearchResult, error) {
	b, err := c.do(ctx, http.MethodPost,
		"/api/"+url.PathEscape(c.cfg.Org)+"/_search?type=logs",
		buildSearchBody(sql, start, end, size))
	if err != nil {
		return SearchResult{}, err
	}
	return parseSearchResponse(b)
}

// Streams fetches the stream list + field schema.
func (c *Client) Streams(ctx context.Context) (cache.Streams, error) {
	b, err := c.do(ctx, http.MethodGet,
		"/api/"+url.PathEscape(c.cfg.Org)+"/streams?type=logs&fetchSchema=true", nil)
	if err != nil {
		return cache.Streams{}, err
	}
	return parseStreams(b)
}

// Exec posts a raw search body and returns the raw response.
func (c *Client) Exec(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/api/"+url.PathEscape(c.cfg.Org)+"/_search?type=logs", body)
}
