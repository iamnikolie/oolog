// Package cache stores the OpenObserve streams + field schema on disk so
// commands can resolve stream/field names without hitting the API.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iamnikolie/oolog/internal/paths"
)

// Field is one column of a stream's schema.
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Stream is one log stream and its fields.
type Stream struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
}

// Streams is the cached set of streams for an account.
type Streams struct {
	List []Stream `json:"list"`
}

func file(account string) (string, error) {
	dir, err := paths.Home(account)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "streams.json"), nil
}

// Load reads streams.json for the account.
func Load(account string) (Streams, error) {
	p, err := file(account)
	if err != nil {
		return Streams{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Streams{}, fmt.Errorf("no streams cache (%s); run 'oolog streams sync': %w", p, err)
	}
	var s Streams
	if err := json.Unmarshal(b, &s); err != nil {
		return Streams{}, err
	}
	return s, nil
}

// Save writes streams.json (pretty-printed).
func Save(s Streams, account string) error {
	p, err := file(account)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// Names lists stream names in cache order.
func (s Streams) Names() []string {
	out := make([]string, len(s.List))
	for i, st := range s.List {
		out[i] = st.Name
	}
	return out
}

// Get returns the named stream.
func (s Streams) Get(name string) (Stream, bool) {
	for _, st := range s.List {
		if st.Name == name {
			return st, true
		}
	}
	return Stream{}, false
}

// Resolve picks the stream to query: the explicit flag, else the configured
// default, else the only stream when there is exactly one, else an error that
// lists the available streams.
func (s Streams) Resolve(flag, configured string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if configured != "" {
		return configured, nil
	}
	switch len(s.List) {
	case 0:
		return "", fmt.Errorf("no streams found; ingest some logs or run 'oolog streams sync'")
	case 1:
		return s.List[0].Name, nil
	}
	return "", fmt.Errorf("multiple streams (%s); pick one with --stream or set `stream:` in config (run 'oolog streams sync' if the list is stale)",
		strings.Join(s.Names(), ", "))
}

// HasField reports whether stream has the named field.
func (s Streams) HasField(stream, field string) bool {
	st, ok := s.Get(stream)
	if !ok {
		return false
	}
	for _, f := range st.Fields {
		if f.Name == field {
			return true
		}
	}
	return false
}
