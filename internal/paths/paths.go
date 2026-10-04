// Package paths computes per-account oolog directories under ~/.oolog
// (or $OOLOG_HOME), creating them on demand.
package paths

import (
	"os"
	"path/filepath"
)

// Home returns the directory for the given account ("" = default), creating it.
func Home(account string) (string, error) {
	base := os.Getenv("OOLOG_HOME")
	if base == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(h, ".oolog")
	}
	dir := base
	if account != "" {
		dir = filepath.Join(base, account)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
