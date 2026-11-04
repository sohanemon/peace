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

// Configuration keys accepted by Value and Assign.
const (
	KeyVersion            = "version"
	KeyReciter            = "reciter"
	KeyTranslation        = "translation"
	KeyTranslationEnabled = "translation_enabled"
	KeyWBWEnabled         = "wbw_enabled"
	KeyWBWLanguage        = "wbw_language"
	KeyWBWTransliteration = "wbw_transliteration"
)

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
		current, decodeErr := decode(path, existing)
		if decodeErr != nil {
			return decodeErr
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

type keySpec struct {
	name    string
	comment string
	value   func(Config) string
	literal func(Config) string
	assign  func(*Config, string) error
}

func intKey(name, comment string, get func(Config) int) keySpec {
	format := func(cfg Config) string { return strconv.Itoa(get(cfg)) }
	return keySpec{
		name:    name,
		comment: comment,
		value:   format,
		literal: format,
	}
}

func stringKey(name, comment string, get func(Config) string, set func(*Config, string)) keySpec {
	return keySpec{
		name:    name,
		comment: comment,
		value:   get,
		literal: func(cfg Config) string { return strconv.Quote(get(cfg)) },
		assign: func(cfg *Config, value string) error {
			set(cfg, value)
			return nil
		},
	}
}

func boolKey(name, comment string, get func(Config) bool, set func(*Config, bool)) keySpec {
	format := func(cfg Config) string { return strconv.FormatBool(get(cfg)) }
	return keySpec{
		name:    name,
		comment: comment,
		value:   format,
		literal: format,
		assign: func(cfg *Config, value string) error {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("%s must be true or false, got %q", name, value)
			}
			set(cfg, parsed)
			return nil
		},
	}
}

var keySpecs = []keySpec{
	intKey(
		KeyVersion,
		"Configuration schema version. Do not edit.",
		func(cfg Config) int { return cfg.Version },
	),
	stringKey(
		KeyReciter,
		"Quran.com numeric reciter ID. List them with `peace reciters`.",
		func(cfg Config) string { return cfg.Reciter },
		func(cfg *Config, value string) { cfg.Reciter = value },
	),
	stringKey(
		KeyTranslation,
		"Quran.com numeric translation ID. List them with `peace translations`.",
		func(cfg Config) string { return cfg.Translation },
		func(cfg *Config, value string) { cfg.Translation = value },
	),
	boolKey(
		KeyTranslationEnabled,
		"Show the ayah translation on wallpapers by default.",
		func(cfg Config) bool { return cfg.TranslationEnabled },
		func(cfg *Config, value bool) { cfg.TranslationEnabled = value },
	),
	boolKey(
		KeyWBWEnabled,
		"Show word-by-word meanings on wallpapers by default.",
		func(cfg Config) bool { return cfg.WBWEnabled },
		func(cfg *Config, value bool) { cfg.WBWEnabled = value },
	),
	stringKey(
		KeyWBWLanguage,
		"Word-by-word meaning language (for example en, ur, bn). List them with `peace languages`.",
		func(cfg Config) string { return cfg.WBWLanguage },
		func(cfg *Config, value string) { cfg.WBWLanguage = value },
	),
	boolKey(
		KeyWBWTransliteration,
		"Show word-by-word transliteration on wallpapers by default.",
		func(cfg Config) bool { return cfg.WBWTransliteration },
		func(cfg *Config, value bool) { cfg.WBWTransliteration = value },
	),
}

func keyFor(name string) (keySpec, bool) {
	for _, spec := range keySpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return keySpec{}, false
}

func unknownKey(name string) error {
	return fmt.Errorf("unknown config key %q (valid keys: %s)", name, strings.Join(KeyNames(), ", "))
}

func render(cfg Config) []byte {
	var builder strings.Builder
	builder.WriteString("# Peace user settings.\n")
	builder.WriteString("# Precedence: CLI flags > PEACE_* environment variables > this file > built-in defaults.\n")
	builder.WriteString("# Edit with `peace config edit` or `peace config set <key> <value>`.\n\n")
	for index, spec := range keySpecs {
		if index > 0 {
			builder.WriteByte('\n')
		}
		fmt.Fprintf(&builder, "# %s\n%s = %s\n", spec.comment, spec.name, spec.literal(cfg))
	}
	return []byte(builder.String())
}

// KeyNames lists the keys accepted by Value and Assign.
func KeyNames() []string {
	names := make([]string, len(keySpecs))
	for index, spec := range keySpecs {
		names[index] = spec.name
	}
	return names
}

// Value returns the printable value of one configuration key.
func Value(cfg Config, key string) (string, error) {
	spec, ok := keyFor(key)
	if !ok {
		return "", unknownKey(key)
	}
	return spec.value(cfg), nil
}

// Assign sets one configuration key from its string representation.
func Assign(cfg *Config, key, value string) error {
	spec, ok := keyFor(key)
	if !ok {
		return unknownKey(key)
	}
	if spec.assign == nil {
		return fmt.Errorf("%s is managed automatically and cannot be set", key)
	}
	return spec.assign(cfg, strings.TrimSpace(value))
}

// EnvOverrides layers PEACE_* environment variables over cfg. CLI flags are
// applied by the command layer and take precedence over the result.
func EnvOverrides(cfg Config) (Config, error) {
	for _, spec := range keySpecs {
		if spec.assign == nil {
			continue
		}
		env := "PEACE_" + strings.ToUpper(spec.name)
		value := strings.TrimSpace(os.Getenv(env))
		if value == "" {
			continue
		}
		if err := Assign(&cfg, spec.name, value); err != nil {
			return Config{}, fmt.Errorf("%s: %w", env, err)
		}
	}
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("environment overrides: %w", err)
	}
	return cfg, nil
}
