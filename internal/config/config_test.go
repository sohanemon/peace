package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func setup(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	for _, name := range []string{
		"PEACE_RECITER", "PEACE_TRANSLATION", "PEACE_WBW_LANGUAGE",
		"PEACE_TRANSLATION_ENABLED", "PEACE_WBW_ENABLED", "PEACE_WBW_TRANSLITERATION",
	} {
		t.Setenv(name, "")
	}
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDefaultsWhenFileMissing(t *testing.T) {
	root := setup(t)
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if want := filepath.Join(root, "config", "peace", "config.toml"); path != want {
		t.Fatalf("Path() = %q, want %q", path, want)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg != DefaultConfig() {
		t.Errorf("Load() = %+v, want defaults %+v", cfg, DefaultConfig())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Load created config file for missing file: %v", err)
	}
}

func TestRoundTripCommentsAndPermissions(t *testing.T) {
	setup(t)
	want := DefaultConfig()
	want.Reciter = "6"
	want.Translation = "131"
	want.TranslationEnabled = false
	want.WBWEnabled = true
	want.WBWLanguage = "ur"
	want.WBWTransliteration = true
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("config mode = %o, want 600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("config dir mode = %o, want 700", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	for _, wantText := range []string{"# Peace user settings.", "version = 1", "reciter = \"6\"", "translation_enabled = false", "wbw_language = \"ur\""} {
		if !strings.Contains(string(data), wantText) {
			t.Errorf("config missing %q:\n%s", wantText, data)
		}
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestMissingKeysUseDefaultsAndUnknownKeysAreIgnored(t *testing.T) {
	setup(t)
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	writeFile(t, path, "version = 1\ntranslation = \"131\"\nfuture_option = \"ignored\"\n")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := DefaultConfig()
	want.Translation = "131"
	if cfg != want {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestMalformedFileErrorsAndIsNotOverwritten(t *testing.T) {
	setup(t)
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	bad := "version = 1\nreciter = [\n"
	writeFile(t, path, bad)
	_, loadErr := Load()
	if loadErr == nil {
		t.Fatal("Load malformed file succeeded")
	}
	if !strings.Contains(loadErr.Error(), path) || !strings.Contains(loadErr.Error(), "line") {
		t.Errorf("malformed error missing path/line: %v", loadErr)
	}
	if err := Save(DefaultConfig()); err == nil {
		t.Fatal("Save overwrote malformed file")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read malformed file: %v", err)
	}
	if string(after) != bad {
		t.Errorf("malformed file changed:\n%s", after)
	}
}

func TestSaveOnlyWritesWhenValuesChange(t *testing.T) {
	setup(t)
	cfg := DefaultConfig()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := Save(cfg); err != nil {
		t.Fatalf("Save unchanged: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("unchanged Save rewrote file: before %v after %v", before.ModTime(), after.ModTime())
	}
}

func TestConcurrentSavesRemainAtomic(t *testing.T) {
	setup(t)
	if err := Save(DefaultConfig()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var group sync.WaitGroup
	for i := 1; i <= 20; i++ {
		group.Add(1)
		go func(id int) {
			defer group.Done()
			cfg := DefaultConfig()
			cfg.Translation = strings.Repeat("1", id%3+1)
			if err := Save(cfg); err != nil {
				t.Errorf("concurrent Save: %v", err)
			}
		}(i)
	}
	group.Wait()
	if _, err := Load(); err != nil {
		t.Fatalf("Load after concurrent saves: %v", err)
	}
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".config-*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(temps) != 0 {
		t.Errorf("leftover temp files: %v", temps)
	}
}

func TestEnvOverridesFileAndDefaults(t *testing.T) {
	setup(t)
	file := DefaultConfig()
	file.Reciter = "6"
	file.Translation = "131"
	file.TranslationEnabled = true
	file.WBWEnabled = false
	file.WBWLanguage = "ur"
	file.WBWTransliteration = false
	if err := Save(file); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg != file {
		t.Fatalf("Load() = %+v, want file %+v", cfg, file)
	}
	for name, value := range map[string]string{
		"PEACE_RECITER":             "7",
		"PEACE_TRANSLATION":         "20",
		"PEACE_TRANSLATION_ENABLED": "false",
		"PEACE_WBW_ENABLED":         "true",
		"PEACE_WBW_LANGUAGE":        "bn",
		"PEACE_WBW_TRANSLITERATION": "true",
	} {
		t.Setenv(name, value)
	}
	got, err := EnvOverrides(cfg)
	if err != nil {
		t.Fatalf("EnvOverrides: %v", err)
	}
	want := DefaultConfig()
	want.Reciter = "7"
	want.Translation = "20"
	want.TranslationEnabled = false
	want.WBWEnabled = true
	want.WBWLanguage = "bn"
	want.WBWTransliteration = true
	if got != want {
		t.Errorf("EnvOverrides() = %+v, want %+v", got, want)
	}
}

func TestInvalidConfigValues(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{name: "zero reciter", change: func(c *Config) { c.Reciter = "0" }, want: "reciter"},
		{name: "text translation", change: func(c *Config) { c.Translation = "en.sahih" }, want: "translation"},
		{name: "empty language", change: func(c *Config) { c.WBWLanguage = "" }, want: "wbw_language"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := DefaultConfig()
			test.change(&cfg)
			err := Validate(cfg)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("Validate() = %v, want error mentioning %q", err, test.want)
			}
		})
	}
}

func TestValueAndAssign(t *testing.T) {
	cfg := DefaultConfig()
	for _, test := range []struct {
		key, value string
		wantError  bool
	}{
		{key: "reciter", value: "6"},
		{key: "translation", value: "131"},
		{key: "translation_enabled", value: "false"},
		{key: "wbw_enabled", value: "true"},
		{key: "wbw_language", value: "ur"},
		{key: "wbw_transliteration", value: "true"},
		{key: "version", value: "1", wantError: true},
		{key: "unknown", value: "x", wantError: true},
		{key: "wbw_enabled", value: "not-a-bool", wantError: true},
	} {
		err := Assign(&cfg, test.key, test.value)
		if (err != nil) != test.wantError {
			t.Errorf("Assign(%q, %q) error = %v, wantError %t", test.key, test.value, err, test.wantError)
			continue
		}
		if !test.wantError {
			got, err := Value(cfg, test.key)
			if err != nil || got != test.value {
				t.Errorf("Value(%q) = %q, %v, want %q", test.key, got, err, test.value)
			}
		}
	}
}
