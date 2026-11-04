// Package config owns peace's persistent user settings. The TOML schema,
// defaults, loading, validation, and atomic saving all live here so no other
// package reads or writes the configuration file directly.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"

	"io.github.sohanemon/peace/internal/quran"
)

// CurrentVersion is the supported configuration schema version.
const CurrentVersion = 1

// Config is the complete persistent peace user configuration.
type Config struct {
	Version            int    `toml:"version"`
	Reciter            string `toml:"reciter"`
	Translation        string `toml:"translation"`
	TranslationEnabled bool   `toml:"translation_enabled"`
	WBWEnabled         bool   `toml:"wbw_enabled"`
	WBWLanguage        string `toml:"wbw_language"`
	WBWTransliteration bool   `toml:"wbw_transliteration"`
}

// DefaultConfig returns the built-in defaults. A missing file or missing keys
// fall back to these values.
func DefaultConfig() Config {
	return Config{
		Version:            CurrentVersion,
		Reciter:            quran.DefaultReciterID,
		Translation:        quran.DefaultTranslationID,
		TranslationEnabled: true,
		WBWEnabled:         false,
		WBWLanguage:        quran.DefaultWBWLanguage,
		WBWTransliteration: false,
	}
}

// Validate checks that a config is safe to use and persist.
func Validate(cfg Config) error {
	if cfg.Version != CurrentVersion {
		return fmt.Errorf("unsupported config version %d (want %d)", cfg.Version, CurrentVersion)
	}
	if err := validateID("reciter", cfg.Reciter); err != nil {
		return err
	}
	if err := validateID("translation", cfg.Translation); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.WBWLanguage) == "" {
		return errors.New("wbw_language must not be empty")
	}
	return nil
}

func validateID(name, value string) error {
	id, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || id <= 0 {
		return fmt.Errorf("%s must be a positive numeric Quran.com ID, got %q", name, value)
	}
	return nil
}

// Load reads the configuration file. A missing file is not an error; the
// built-in defaults are returned.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	return decode(path, data)
}

func decode(path string, data []byte) (Config, error) {
	cfg := DefaultConfig()
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, parseError(path, err)
	}
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes cfg atomically. It refuses to overwrite malformed files and
// skips the write when the effective values are already present.
func Save(cfg Config) error {
	if err := Validate(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	path, err := Path()
	if err != nil {
		return err
	}

	writeMu.Lock()
	defer writeMu.Unlock()

	data := render(cfg)
	if existing, readErr := os.ReadFile(path); readErr == nil {
		current := DefaultConfig()
		if unmarshalErr := toml.Unmarshal(existing, &current); unmarshalErr != nil {
			return parseError(path, unmarshalErr)
		}
		if current == cfg {
			return nil
		}
	} else if !errors.Is(readErr, fs.ErrNotExist) {
		return fmt.Errorf("read config %s: %w", path, readErr)
	}
	if err := writeAtomic(path, data); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

var writeMu sync.Mutex

func parseError(path string, err error) error {
	var decodeErr *toml.DecodeError
	if errors.As(err, &decodeErr) {
		line, column := decodeErr.Position()
		return fmt.Errorf("parse config %s: line %d, column %d: %w", path, line, column, err)
	}
	return fmt.Errorf("parse config %s: %w", path, err)
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	temp := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(temp)
	}
	if err := file.Chmod(0o600); err != nil {
		cleanup()
		return err
	}
	if _, err := file.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temp)
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return err
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func render(cfg Config) []byte {
	var builder strings.Builder
	builder.WriteString("# Peace user settings.\n")
	builder.WriteString("# Precedence: CLI flags > PEACE_* environment variables > this file > built-in defaults.\n")
	builder.WriteString("# Edit with `peace config edit` or `peace config set <key> <value>`.\n\n")

	builder.WriteString("# Configuration schema version. Do not edit.\n")
	fmt.Fprintf(&builder, "version = %d\n\n", cfg.Version)

	builder.WriteString("# Quran.com numeric reciter ID. List them with `peace reciters`.\n")
	fmt.Fprintf(&builder, "reciter = %s\n\n", strconv.Quote(cfg.Reciter))

	builder.WriteString("# Quran.com numeric translation ID. List them with `peace translations`.\n")
	fmt.Fprintf(&builder, "translation = %s\n\n", strconv.Quote(cfg.Translation))

	builder.WriteString("# Show the ayah translation on wallpapers by default.\n")
	fmt.Fprintf(&builder, "translation_enabled = %t\n\n", cfg.TranslationEnabled)

	builder.WriteString("# Show word-by-word meanings on wallpapers by default.\n")
	fmt.Fprintf(&builder, "wbw_enabled = %t\n\n", cfg.WBWEnabled)

	builder.WriteString("# Word-by-word meaning language (for example en, ur, bn). List them with `peace languages`.\n")
	fmt.Fprintf(&builder, "wbw_language = %s\n\n", strconv.Quote(cfg.WBWLanguage))

	builder.WriteString("# Show word-by-word transliteration on wallpapers by default.\n")
	fmt.Fprintf(&builder, "wbw_transliteration = %t\n", cfg.WBWTransliteration)
	return []byte(builder.String())
}

// KeyNames lists the keys accepted by Value and Assign.
func KeyNames() []string {
	return []string{"version", "reciter", "translation", "translation_enabled", "wbw_enabled", "wbw_language", "wbw_transliteration"}
}

// Value returns the printable value of one configuration key.
func Value(cfg Config, key string) (string, error) {
	switch key {
	case "version":
		return strconv.Itoa(cfg.Version), nil
	case "reciter":
		return cfg.Reciter, nil
	case "translation":
		return cfg.Translation, nil
	case "translation_enabled":
		return strconv.FormatBool(cfg.TranslationEnabled), nil
	case "wbw_enabled":
		return strconv.FormatBool(cfg.WBWEnabled), nil
	case "wbw_language":
		return cfg.WBWLanguage, nil
	case "wbw_transliteration":
		return strconv.FormatBool(cfg.WBWTransliteration), nil
	default:
		return "", fmt.Errorf("unknown config key %q (valid keys: %s)", key, strings.Join(KeyNames(), ", "))
	}
}

// Assign sets one configuration key from its string representation.
func Assign(cfg *Config, key, value string) error {
	value = strings.TrimSpace(value)
	switch key {
	case "version":
		return errors.New("version is managed automatically and cannot be set")
	case "reciter":
		cfg.Reciter = value
	case "translation":
		cfg.Translation = value
	case "translation_enabled":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("translation_enabled must be true or false, got %q", value)
		}
		cfg.TranslationEnabled = parsed
	case "wbw_enabled":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("wbw_enabled must be true or false, got %q", value)
		}
		cfg.WBWEnabled = parsed
	case "wbw_language":
		cfg.WBWLanguage = value
	case "wbw_transliteration":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("wbw_transliteration must be true or false, got %q", value)
		}
		cfg.WBWTransliteration = parsed
	default:
		return fmt.Errorf("unknown config key %q (valid keys: %s)", key, strings.Join(KeyNames(), ", "))
	}
	return nil
}

// EnvOverrides layers PEACE_* environment variables over cfg. CLI flags are
// applied by the command layer and take precedence over the result.
func EnvOverrides(cfg Config) (Config, error) {
	if value := strings.TrimSpace(os.Getenv("PEACE_RECITER")); value != "" {
		cfg.Reciter = value
	}
	if value := strings.TrimSpace(os.Getenv("PEACE_TRANSLATION")); value != "" {
		cfg.Translation = value
	}
	if value := strings.TrimSpace(os.Getenv("PEACE_WBW_LANGUAGE")); value != "" {
		cfg.WBWLanguage = value
	}
	for _, item := range []struct {
		name  string
		value *bool
	}{
		{"PEACE_TRANSLATION_ENABLED", &cfg.TranslationEnabled},
		{"PEACE_WBW_ENABLED", &cfg.WBWEnabled},
		{"PEACE_WBW_TRANSLITERATION", &cfg.WBWTransliteration},
	} {
		value := strings.TrimSpace(os.Getenv(item.name))
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, fmt.Errorf("%s must be true or false, got %q", item.name, value)
		}
		*item.value = parsed
	}
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("environment overrides: %w", err)
	}
	return cfg, nil
}
