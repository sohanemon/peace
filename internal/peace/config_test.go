package peace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"io.github.sohanemon/peace/internal/config"
)

func runCLI(t *testing.T, runtime Runtime, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(runtime)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := executeRoot(cmd)
	return stdout.String(), err
}

func TestConfigPathAndGetSetCommands(t *testing.T) {
	root := setConfigHome(t)
	runtime := Runtime{Config: Config{StateDir: t.TempDir()}, Settings: config.DefaultConfig()}

	wantPath := filepath.Join(root, "config", "peace", "config.toml")
	output, err := runCLI(t, runtime, "config", "path")
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if strings.TrimSpace(output) != wantPath {
		t.Errorf("config path = %q, want %q", strings.TrimSpace(output), wantPath)
	}

	for _, args := range [][]string{
		{"config", "set", "translation", "131"},
		{"config", "set", "wbw_enabled", "true"},
		{"config", "set", "reciter", "6"},
	} {
		if _, err := runCLI(t, runtime, args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Translation != "131" || !cfg.WBWEnabled || cfg.Reciter != "6" {
		t.Errorf("config after set = %+v", cfg)
	}
	output, err = runCLI(t, runtime, "config", "get", "translation")
	if err != nil {
		t.Fatalf("config get: %v", err)
	}
	if strings.TrimSpace(output) != "131" {
		t.Errorf("config get translation = %q, want 131", strings.TrimSpace(output))
	}
}

func TestConfigRejectsInvalidValues(t *testing.T) {
	setConfigHome(t)
	runtime := Runtime{Config: Config{StateDir: t.TempDir()}, Settings: config.DefaultConfig()}
	for _, args := range [][]string{
		{"config", "set", "translation", "en.sahih"},
		{"config", "set", "reciter", "0"},
		{"config", "set", "wbw_enabled", "maybe"},
		{"config", "set", "version", "2"},
	} {
		if _, err := runCLI(t, runtime, args...); err == nil {
			t.Errorf("%v succeeded, want validation error", args)
		}
	}
	if _, err := runCLI(t, runtime, "config", "get", "unknown"); err == nil {
		t.Error("config get unknown succeeded")
	}
}

func TestTranslationPrecedenceFlagsEnvFileDefaults(t *testing.T) {
	for _, test := range []struct {
		name string
		file string
		env  string
		flag string
		want string
	}{
		{name: "default", want: "20"},
		{name: "file", file: "131", want: "131"},
		{name: "env over file", file: "131", env: "19", want: "19"},
		{name: "flag over env and file", file: "131", env: "19", flag: "6", want: "6"},
		{name: "env only", env: "19", want: "19"},
		{name: "flag only", flag: "6", want: "6"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setConfigHome(t)
			if test.file != "" {
				cfg := config.DefaultConfig()
				cfg.Translation = test.file
				if err := config.Save(cfg); err != nil {
					t.Fatalf("Save: %v", err)
				}
			}
			if test.env != "" {
				t.Setenv("PEACE_TRANSLATION", test.env)
			}
			settings, err := config.Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			settings, err = config.EnvOverrides(settings)
			if err != nil {
				t.Fatalf("EnvOverrides: %v", err)
			}
			runtime := Runtime{Settings: settings}
			cmd := &cobra.Command{}
			cmd.Flags().String("translation", "", "")
			if test.flag != "" {
				if err := cmd.Flags().Set("translation", test.flag); err != nil {
					t.Fatalf("set flag: %v", err)
				}
			}
			if got := resolveTranslation(cmd, runtime); got != test.want {
				t.Errorf("resolveTranslation() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRenderDefaultsComeFromConfig(t *testing.T) {
	setConfigHome(t)
	cfg := config.DefaultConfig()
	cfg.TranslationEnabled = false
	cfg.WBWEnabled = true
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	runtime := Runtime{Settings: cfg}
	cmd := &cobra.Command{}
	addWallpaperFlags(cmd, runtime)
	render := wallpaperRenderOptions(cmd)
	if render.ShowTranslation || !render.ShowWordTranslation {
		t.Errorf("wallpaper defaults = %+v, want translation false and words true", render)
	}
	if err := cmd.Flags().Set("show-translation", "true"); err != nil {
		t.Fatalf("set show-translation: %v", err)
	}
	if err := cmd.Flags().Set("show-word-translation", "false"); err != nil {
		t.Fatalf("set show-word-translation: %v", err)
	}
	render = wallpaperRenderOptions(cmd)
	if !render.ShowTranslation || render.ShowWordTranslation {
		t.Errorf("flag overrides = %+v, want translation true and words false", render)
	}
}

func TestConfigPathWorksWithMalformedConfig(t *testing.T) {
	root := setConfigHome(t)
	path := filepath.Join(root, "config", "peace", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("version = 1\nreciter = [\n"), 0o600); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}
	if err := Run([]string{"config", "path"}); err != nil {
		t.Errorf("config path with malformed file: %v", err)
	}
	if err := Run([]string{"settings"}); err == nil {
		t.Error("settings with malformed file succeeded")
	}
}

func TestListenUsesConfiguredReciterUnlessFlagOverrides(t *testing.T) {
	setConfigHome(t)
	cfg := config.DefaultConfig()
	cfg.Reciter = "6"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	runtime := Runtime{Settings: cfg}
	cmd := newListenCommand(runtime)
	value, err := cmd.Flags().GetString("reciter")
	if err != nil {
		t.Fatalf("get reciter flag: %v", err)
	}
	if value != "6" {
		t.Errorf("reciter flag default = %q, want 6", value)
	}
	if err := cmd.Flags().Set("reciter", "7"); err != nil {
		t.Fatalf("set reciter: %v", err)
	}
	value, _ = cmd.Flags().GetString("reciter")
	if value != "7" {
		t.Errorf("reciter flag override = %q, want 7", value)
	}
}
