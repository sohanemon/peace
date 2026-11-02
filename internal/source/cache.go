package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const cacheMaxAge = 30 * 24 * time.Hour

func readCache(path string, value any) bool {
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > cacheMaxAge {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, value) == nil
}

func writeCache(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeAtomic(path, data, 0o600)
}

func cacheFile(dir, name string) string {
	return filepath.Join(dir, "sources", safeName(name)+".json")
}

func getenv(key string) string { return os.Getenv(key) }

func userCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "peace"), nil
}

func safeName(value string) string {
	var builder strings.Builder
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '.', character == '-', character == '_':
			builder.WriteRune(character)
		default:
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func translateError(provider string, err error) error {
	if err == nil {
		return nil
	}
	var requestErr *providerError
	if errors.As(err, &requestErr) {
		return err
	}
	return fmt.Errorf("%s: %w", provider, err)
}

type providerError struct {
	provider string
	err      error
}

func (e *providerError) Error() string { return e.provider + ": " + e.err.Error() }
func (e *providerError) Unwrap() error { return e.err }
