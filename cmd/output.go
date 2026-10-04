package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/iamnikolie/oolog/internal/render"
)

// effectiveFormat folds --json into --format.
func effectiveFormat() string {
	if outJSON {
		return "json"
	}
	return outFormat
}

// renderHits formats search hits per the active --format (default: LLM lines).
func renderHits(hits []map[string]any) (string, error) {
	switch effectiveFormat() {
	case "csv":
		return render.CSV(hits)
	case "tsv":
		return render.TSV(hits)
	default:
		return render.LogLines(hits, renderFields()), nil
	}
}

// outputJSON writes raw JSON when --json/--format json is set, else calls fallback.
func outputJSON(raw any, fallback func() (string, error)) error {
	if effectiveFormat() == "json" {
		b, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(b))
		return nil
	}
	s, err := fallback()
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, s)
	return nil
}

// renderFields converts the configured field mapping for the line renderer.
func renderFields() render.Fields {
	f := cfg.Fields.Resolved()
	return render.Fields{Namespace: f.Namespace, Pod: f.Pod, Container: f.Container, Level: f.Level, Message: f.Message}
}
