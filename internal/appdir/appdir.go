// Package appdir resolves on-disk locations for hashdupes (database, etc.) in a
// single place so the GUI app and CLI tooling agree on paths.
package appdir

import (
	"os"
	"path/filepath"
)

// dirName is the per-user application subdirectory.
const dirName = "hashdupes"

// ConfigDir returns the hashdupes configuration directory, creating it if
// necessary (e.g. ~/.config/hashdupes on Linux).
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// DefaultDBPath returns the default path to the index database.
func DefaultDBPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hashdupes.db"), nil
}
