package quran

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	resourceCacheTTL = 30 * 24 * time.Hour
	surahCacheTTL    = 30 * 24 * time.Hour
	recitersCacheKey = "reciters-v2"
)

func cachePath(dir, key string) string {
	return filepath.Join(dir, "quran", safeName(key)+".json")
}

func loadCache(dir, key string, ttl time.Duration, value any) bool {
	if dir == "" {
		return false
	}
	path := cachePath(dir, key)
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > ttl {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, value) == nil
}

func saveCache(dir, key string, value any) error {
	if dir == "" {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeAtomic(cachePath(dir, key), data, 0o600)
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
