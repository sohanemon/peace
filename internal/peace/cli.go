// Package peace implements the command-line interface for the peace application.
// NOTE: Keep command routing here and place playback, rendering, and API work in focused modules.
package peace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Run parses argv and dispatches to the appropriate command handler.
func Run(argv0 string, args []string) error {
	cfg, err := loadConfig(argv0)
	if err != nil {
		return err
	}
	runtime := Runtime{Config: cfg}
	cmd := newRootCommand(runtime, filepath.Base(argv0))
	cmd.SetArgs(args)
	return executeRoot(cmd)
}

type cliUsageError struct{ err error }

func (e *cliUsageError) Error() string { return e.err.Error() }
func (e *cliUsageError) Unwrap() error { return e.err }

func executeRoot(root *cobra.Command) error {
	command, err := root.ExecuteC()
	if err == nil {
		return nil
	}
	var usageErr *cliUsageError
	if errors.As(err, &usageErr) || strings.Contains(err.Error(), "unknown flag") || strings.Contains(err.Error(), "unknown shorthand flag") {
		if command != nil {
			return fmt.Errorf("%w\n%s", err, command.UsageString())
		}
		return fmt.Errorf("%w\n%s", err, root.UsageString())
	}
	return err
}

func newRootCommand(runtime Runtime, executable string) *cobra.Command {
	root := &cobra.Command{
		Use:                        executable,
		Short:                      "Listen to Quran recitation and control Peace wallpapers",
		Long:                       "Play Quran recitation, control playback, and set ayah wallpapers.",
		Example:                    "  peace listen 2:255\n  peace listen 110 --follow --random\n  peace wallpaper next",
		SilenceUsage:               true,
		SilenceErrors:              true,
		SuggestionsMinimumDistance: 2,
	}
	root.SetFlagErrorFunc(suggestFlag)
	root.CompletionOptions.DisableDefaultCmd = true
	if executable != "peace" {
		configureAliasRoot(root, runtime, executable)
		return root
	}

	listen := newListenCommand(runtime)
	root.AddCommand(listen)
	root.AddCommand(&cobra.Command{Use: "pause", Short: "Pause or resume audio playback", Long: "Toggle pause on the current recitation.", Example: "  peace pause", Args: noArgs, RunE: func(*cobra.Command, []string) error { return runtime.pause() }})
	root.AddCommand(&cobra.Command{Use: "stop", Short: "Stop audio playback", Long: "Stop recitation and any pending wallpaper preparation.", Example: "  peace stop", Args: noArgs, RunE: func(*cobra.Command, []string) error { return runtime.stop() }})
	root.AddCommand(&cobra.Command{Use: "status", Short: "Show playback status", Long: "Show the current ayah and playback state.", Example: "  peace status", Args: noArgs, RunE: func(*cobra.Command, []string) error { return runtime.status() }})
	root.AddCommand(newWallpaperCommand(runtime, "wallpaper", "Set an ayah wallpaper", "  peace wallpaper\n  peace wallpaper next\n  peace wallpaper --random"))
	for _, action := range []string{"random", "next", "prev"} {
		action := action
		short := map[string]string{"random": "Set a random ayah wallpaper", "next": "Show the next ayah wallpaper", "prev": "Show the previous ayah wallpaper"}[action]
		cmd := &cobra.Command{Use: action, Short: short, Long: short + ".", Example: "  peace " + action, Args: noArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			randomBackground, _ := cmd.Flags().GetBool("random")
			return runtime.wallpaper(action, randomBackground)
		}}
		addWallpaperFlag(cmd)
		root.AddCommand(cmd)
	}
	root.AddCommand(&cobra.Command{Use: "__follow", Hidden: true, Args: noArgs, RunE: func(*cobra.Command, []string) error { return runtime.follow() }})
	root.AddCommand(&cobra.Command{Use: "__prepare SURAH AYAH", Hidden: true, Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error { return runtime.prepare(args) }})
	for _, action := range []string{"init", "uninstall", "enable", "disable", "logs"} {
		action := action
		root.AddCommand(&cobra.Command{Use: action, Hidden: true, Args: cobra.ArbitraryArgs, RunE: func(_ *cobra.Command, args []string) error {
			return runCommand(filepath.Join(runtime.Config.Home, ".local", "bin", "peace-"+action), args...)
		}})
	}
	return root
}

func suggestFlag(cmd *cobra.Command, err error) error {
	fields := strings.Fields(err.Error())
	if len(fields) < 2 || !strings.Contains(err.Error(), "unknown flag") {
		return &cliUsageError{err: err}
	}
	unknown := strings.TrimLeft(fields[len(fields)-1], "-")
	best, bestDistance := "", 3
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		distance := editDistance(unknown, flag.Name)
		if distance < bestDistance {
			best, bestDistance = flag.Name, distance
		}
	})
	if best != "" {
		return &cliUsageError{err: fmt.Errorf("%w (did you mean --%s?)", err, best)}
	}
	return &cliUsageError{err: err}
}

func noArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.NoArgs(cmd, args); err != nil {
		return &cliUsageError{err: err}
	}
	return nil
}

func editDistance(a, b string) int {
	previous := make([]int, len(b)+1)
	for i := range previous {
		previous[i] = i
	}
	for i, left := range a {
		current := make([]int, len(b)+1)
		current[0] = i + 1
		for j, right := range b {
			cost := 0
			if left != right {
				cost = 1
			}
			current[j+1] = min(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = current
	}
	return previous[len(b)]
}

func configureAliasRoot(root *cobra.Command, runtime Runtime, name string) {
	root.Short = "Peace command alias"
	switch name {
	case "peace-listen":
		cmd := newListenCommand(runtime)
		cmd.Use = "peace-listen SURAH[:AYAH]"
		root.Short = cmd.Short
		root.Long, root.Example, root.Args, root.RunE = cmd.Long, cmd.Example, cmd.Args, cmd.RunE
		root.Flags().AddFlagSet(cmd.Flags())
	case "peace-pause":
		root.Short, root.Long, root.Example = "Pause or resume audio playback", "Toggle pause on the current recitation.", "  peace-pause"
		root.Args, root.RunE = noArgs, func(*cobra.Command, []string) error { return runtime.pause() }
	case "peace-stop":
		root.Short, root.Long, root.Example = "Stop audio playback", "Stop recitation and pending wallpaper preparation.", "  peace-stop"
		root.Args, root.RunE = noArgs, func(*cobra.Command, []string) error { return runtime.stop() }
	case "peace-status":
		root.Short, root.Long, root.Example = "Show playback status", "Show the current ayah and playback state.", "  peace-status"
		root.Args, root.RunE = noArgs, func(*cobra.Command, []string) error { return runtime.status() }
	default:
		root.RunE = func(*cobra.Command, []string) error { return fmt.Errorf("peace: unknown command name %q", name) }
	}
}

func newListenCommand(runtime Runtime) *cobra.Command {
	options := Options{Reciter: os.Getenv("PEACE_RECITER"), Buffer: 10, Lead: 2, Jobs: 2}
	if options.Reciter == "" {
		options.Reciter = "ar.alafasy"
	}
	cmd := &cobra.Command{
		Use:     "listen SURAH[:AYAH|:START-END|:START-]",
		Short:   "Play Quran recitation",
		Long:    "Play one ayah, an ayah range, or the rest of a surah. Use --follow to update the wallpaper as playback advances.",
		Example: "  peace listen 2:255\n  peace listen 18:1-10 --reciter ar.alafasy\n  peace listen 110 --follow --random",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return &cliUsageError{err: errors.New("peace listen: ayah reference is required")}
			}
			if _, _, _, err := parseReference(args[0]); err != nil {
				return &cliUsageError{err: err}
			}
			return nil
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			for _, pair := range []struct {
				name            string
				value, min, max int
			}{{"buffer", options.Buffer, 1, 20}, {"lead", options.Lead, 1, 5}, {"jobs", options.Jobs, 1, 4}} {
				if pair.value < pair.min || pair.value > pair.max {
					return &cliUsageError{err: fmt.Errorf("peace listen: --%s must be between %d and %d (example: --%s %d)", pair.name, pair.min, pair.max, pair.name, pair.min)}
				}
			}
			if !safeID(options.Reciter) {
				return &cliUsageError{err: errors.New("peace listen: invalid reciter ID (example: --reciter ar.alafasy)")}
			}
			if options.Random && !options.Follow {
				return &cliUsageError{err: errors.New("peace listen: --random requires --follow (example: peace listen 110 --follow --random)")}
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			options.Reference = args[0]
			return runtime.listenOptions(options)
		},
	}
	cmd.Flags().BoolVar(&options.Follow, "follow", false, "Update wallpaper as each ayah plays")
	cmd.Flags().BoolVar(&options.Random, "random", false, "Choose a random current background (requires --follow)")
	cmd.Flags().StringVar(&options.Reciter, "reciter", options.Reciter, "Reciter ID")
	cmd.Flags().IntVar(&options.Buffer, "buffer", 10, "Upcoming ayahs to buffer (1-20)")
	cmd.Flags().IntVar(&options.Lead, "lead", 2, "Wallpapers to prepare ahead (1-5)")
	cmd.Flags().IntVar(&options.Jobs, "jobs", 2, "Concurrent wallpaper preparations (1-4)")
	return cmd
}

func newWallpaperCommand(runtime Runtime, use, short, example string) *cobra.Command {
	cmd := &cobra.Command{Use: use + " [random|next|prev]", Short: short, Long: short + ".", Example: example,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &cliUsageError{err: fmt.Errorf("peace wallpaper: expected at most one action, got %d", len(args))}
			}
			if len(args) == 1 && args[0] != "random" && args[0] != "next" && args[0] != "prev" {
				return &cliUsageError{err: fmt.Errorf("peace wallpaper: invalid action %q (expected random, next, or prev; example: peace wallpaper next)", args[0])}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			action := "wallpaper"
			if len(args) == 1 {
				action = args[0]
			}
			randomBackground, _ := cmd.Flags().GetBool("random")
			return runtime.wallpaper(action, randomBackground)
		}}
	addWallpaperFlag(cmd)
	return cmd
}

func addWallpaperFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("random", false, "Choose a random current background")
}

func loadConfig(argv0 string) (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("peace: locate home directory: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return Config{}, fmt.Errorf("peace: locate executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	pluginDir := filepath.Dir(filepath.Dir(executable))
	if filepath.Base(filepath.Dir(executable)) != "bin" {
		pluginDir = filepath.Dir(executable)
	}
	state := filepath.Join(home, ".local", "state", "peace")
	cache := filepath.Join(home, ".cache", "peace")
	socket := filepath.Join(os.TempDir(), "peace-mpv-"+strconv.Itoa(os.Getuid())+".sock")
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		socket = filepath.Join(runtimeDir, "peace-mpv.sock")
	}
	return Config{Home: home, PluginDir: pluginDir, StateDir: state, CacheDir: cache, Socket: socket}, nil
}

func runCommand(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &commandExit{code: exitErr.ExitCode()}
		}
		return err
	}
	return nil
}

type commandExit struct{ code int }

func (e *commandExit) Error() string { return fmt.Sprintf("command exited with status %d", e.code) }
func (e *commandExit) ExitCode() int { return e.code }

func safeID(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && !strings.ContainsRune("._-", c) {
			return false
		}
	}
	return true
}
