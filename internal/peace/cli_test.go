package peace

import (
	"bytes"
	"strings"
	"testing"
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
		for _, command := range []string{"listen", "pause", "stop", "status", "wallpaper", "random", "next", "prev"} {
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
