package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Dir returns the peace configuration directory. os.UserConfigDir respects
// XDG_CONFIG_HOME on Linux.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate config directory: %w", err)
	}
	return filepath.Join(base, "peace"), nil
}

// Path returns the absolute path of the peace TOML configuration file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}
