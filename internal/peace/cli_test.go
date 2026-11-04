package peace

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"io.github.sohanemon/peace/internal/config"
)

func TestRootHelpListsCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}} {
		var stdout, stderr bytes.Buffer
		cmd := newRootCommand(Runtime{})
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)

		if err := executeRoot(cmd); err != nil {
			t.Fatalf("execute root help with args %v: %v", args, err)
		}
		for _, command := range []string{"listen", "words", "reciters", "translations", "languages", "settings", "config", "pause", "stop", "status", "wallpaper", "random", "next", "prev", "current"} {
			if !strings.Contains(stdout.String(), command) {
				t.Errorf("root help missing %q:\n%s", command, stdout.String())
			}
		}
		if strings.Contains(stderr.String(), "Usage:") {
			t.Errorf("help wrote usage to stderr: %s", stderr.String())
		}
	}
}

func TestListenHelpShowsListenUsageAndExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(Runtime{})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"listen", "--help"})

	if err := executeRoot(cmd); err != nil {
		t.Fatalf("execute listen help: %v", err)
	}
	for _, want := range []string{"peace listen", "SURAH", "--follow", "--basmala", "Examples:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("listen help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestHelpSubcommandShowsListenHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(Runtime{})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"help", "listen"})

	if err := executeRoot(cmd); err != nil {
		t.Fatalf("execute help listen: %v", err)
	}
	if !strings.Contains(stdout.String(), "peace listen") || !strings.Contains(stdout.String(), "--follow") {
		t.Errorf("help listen did not show command help:\n%s", stdout.String())
	}
}

func TestListenMissingReferenceShowsContextAndUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(Runtime{})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"listen"})

	err := executeRoot(cmd)
	if err == nil || !strings.Contains(err.Error(), "reference") {
		t.Fatalf("expected contextual missing-reference error, got %v", err)
	}
	if !strings.Contains(err.Error(), "Usage:") || !strings.Contains(err.Error(), "Examples:") {
		t.Errorf("expected listen usage and examples in error:\n%s", err)
	}
}

func TestListenInvalidReferenceExplainsExpectedFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(Runtime{})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"listen", "abc"})

	err := executeRoot(cmd)
	if err == nil || !strings.Contains(err.Error(), "SURAH[:AYAH") {
		t.Fatalf("expected reference format error, got %v", err)
	}
	if !strings.Contains(err.Error(), "Usage:") || !strings.Contains(err.Error(), "peace listen") {
		t.Errorf("expected contextual usage in error:\n%s", err)
	}
}

func TestUnknownFlagShowsCommandUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(Runtime{})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"listen", "110", "--follw"})

	err := executeRoot(cmd)
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
	if !strings.Contains(err.Error(), "Usage:") || !strings.Contains(err.Error(), "follow") {
		t.Errorf("expected flag suggestion and command usage, err=%v", err)
	}
}

func TestListenAcceptsExplicitBasmalaValue(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		cmd := newRootCommand(Runtime{})
		cmd.SetArgs([]string{"listen", "invalid", "--basmala=" + value})
		err := executeRoot(cmd)
		if err == nil || strings.Contains(err.Error(), "unknown flag") {
			t.Errorf("explicit --basmala=%s was not accepted: %v", value, err)
		}
	}
}

func TestUnknownCommandSuggestsCloseMatch(t *testing.T) {
	cmd := newRootCommand(Runtime{})
	cmd.SetArgs([]string{"liset"})

	err := executeRoot(cmd)
	if err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("expected listen suggestion, got %v", err)
	}
}

func TestRandomListenFlagRequiresFollow(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(Runtime{})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"listen", "110", "--random"})

	err := executeRoot(cmd)
	if err == nil || !strings.Contains(err.Error(), "--random requires --follow") || !strings.Contains(err.Error(), "Usage:") {
		t.Fatalf("expected contextual random/follow validation, got %v", err)
	}
}

func TestRuntimeErrorDoesNotPrintUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(Runtime{})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"pause"})

	err := executeRoot(cmd)
	if err == nil {
		t.Fatal("expected pause runtime error")
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("runtime error printed command usage: %s", stderr.String())
	}
}

func TestWordsRejectsRangeAndWholeSurah(t *testing.T) {
	for _, reference := range []string{"2", "2:255-260", "2:255-"} {
		cmd := newRootCommand(Runtime{})
		cmd.SetArgs([]string{"words", reference})
		err := executeRoot(cmd)
		if err == nil || !strings.Contains(err.Error(), "SURAH:AYAH") {
			t.Errorf("words %q error = %v, want single-ayah format error", reference, err)
		}
	}
}

func TestWordsMissingReferenceShowsUsage(t *testing.T) {
	cmd := newRootCommand(Runtime{})
	cmd.SetArgs([]string{"words"})
	err := executeRoot(cmd)
	if err == nil || !strings.Contains(err.Error(), "reference") || !strings.Contains(err.Error(), "Usage:") {
		t.Fatalf("words missing reference error = %v", err)
	}
}

func TestLanguageMatches(t *testing.T) {
	for _, test := range []struct {
		value, filter string
		want          bool
	}{
		{value: "english", filter: "", want: true},
		{value: "english", filter: "en", want: true},
		{value: "english", filter: "english", want: true},
		{value: "arabic", filter: "ar", want: true},
		{value: "urdu", filter: "ur", want: true},
		{value: "english", filter: "ur", want: false},
		{value: "urdu", filter: "english", want: false},
	} {
		if got := languageMatches(test.value, test.filter); got != test.want {
			t.Errorf("languageMatches(%q, %q) = %t, want %t", test.value, test.filter, got, test.want)
		}
	}
}

func TestUnexpectedCommandArgumentShowsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(Runtime{})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"pause", "extra"})

	err := executeRoot(cmd)
	if err == nil || !strings.Contains(err.Error(), "Usage:") || !strings.Contains(err.Error(), "peace pause") {
		t.Fatalf("expected pause usage for unexpected argument, got %v", err)
	}
}

func TestSettingsCommandAndPersistence(t *testing.T) {
	setConfigHome(t)
	runtime := Runtime{Config: Config{StateDir: t.TempDir()}, Settings: config.DefaultConfig()}
	run := func(args ...string) settingsOutput {
		t.Helper()
		var stdout bytes.Buffer
		cmd := newRootCommand(runtime)
		cmd.SetOut(&stdout)
		cmd.SetArgs(append([]string{"settings", "--json"}, args...))
		if err := executeRoot(cmd); err != nil {
			t.Fatalf("settings %v: %v", args, err)
		}
		var output settingsOutput
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			t.Fatalf("unmarshal settings json: %v", err)
		}
		return output
	}

	if initial := run(); initial.Translation != "20" || initial.WBWLanguage != "en" {
		t.Errorf("initial settings = %+v, want translation: 20, wbw_language: en", initial)
	}
	if updated := run("--translation", "131"); updated.Translation != "131" || updated.WBWLanguage != "en" {
		t.Errorf("after translation update = %+v, want translation: 131, wbw_language: en", updated)
	}
	if updated := run("--wbw-language", "ur"); updated.Translation != "131" || updated.WBWLanguage != "ur" {
		t.Errorf("after wbw update = %+v, want translation: 131, wbw_language: ur", updated)
	}
	saved, err := config.Load()
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if saved.Translation != "131" || saved.WBWLanguage != "ur" {
		t.Errorf("saved config = %+v, want translation: 131, wbw_language: ur", saved)
	}
}

func setConfigHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", root+"/config")
	t.Setenv("XDG_STATE_HOME", root+"/state")
	t.Setenv("XDG_CACHE_HOME", root+"/cache")
	return root
}

func TestWallpaperCurrentValidAction(t *testing.T) {
	runtime := Runtime{Config: Config{StateDir: t.TempDir()}}
	cmd := newRootCommand(runtime)
	cmd.SetArgs([]string{"wallpaper", "current", "--help"})
	if err := executeRoot(cmd); err != nil {
		t.Fatalf("wallpaper current --help failed: %v", err)
	}
}
