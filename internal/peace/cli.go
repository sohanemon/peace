// Package peace implements the command-line interface for the peace application.
// NOTE: Keep command routing here and place playback, rendering, and API work in focused modules.
package peace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"io.github.sohanemon/peace/internal/config"
	"io.github.sohanemon/peace/internal/quran"
)

// Run parses argv and dispatches to the appropriate command handler.
func Run(args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	settings := config.DefaultConfig()
	loaded, loadErr := config.Load()
	if loadErr == nil {
		loaded, loadErr = config.EnvOverrides(loaded)
	}
	if loadErr != nil {
		if !settingsOptional(args) {
			return fmt.Errorf("peace: %w", loadErr)
		}
	} else {
		settings = loaded
	}
	runtime := Runtime{Config: cfg, Settings: settings}
	cmd := newRootCommand(runtime)
	cmd.SetArgs(args)
	return executeRoot(cmd)
}

func settingsOptional(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	switch args[0] {
	case "help":
		return true
	case "config":
		return len(args) > 1 && (args[1] == "path" || args[1] == "edit")
	default:
		return false
	}
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

func newRootCommand(runtime Runtime) *cobra.Command {
	root := &cobra.Command{
		Use:                        "peace",
		Short:                      "Listen to Quran recitation and control Peace wallpapers",
		Long:                       "Play Quran recitation, control playback, and set ayah wallpapers.",
		Example:                    "  peace listen 2:255\n  peace listen 110 --follow --random\n  peace wallpaper next",
		SilenceUsage:               true,
		SilenceErrors:              true,
		SuggestionsMinimumDistance: 2,
	}
	root.SetFlagErrorFunc(suggestFlag)
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().String("translation", "", "Translation ID (numeric; legacy IDs such as en.sahih migrate automatically)")
	root.PersistentFlags().String("wbw-language", "", "Word-by-word meaning language (for example en, ur, bn)")

	listen := newListenCommand(runtime)
	root.AddCommand(listen)
	root.AddCommand(newWordsCommand(runtime))
	root.AddCommand(newListCommand(runtime, "reciters", "List available reciters"))
	root.AddCommand(newListCommand(runtime, "translations", "List available translations"))
	root.AddCommand(newListCommand(runtime, "languages", "List available word-by-word languages"))
	root.AddCommand(newSettingsCommand(runtime))
	root.AddCommand(newConfigCommand(runtime))
	root.AddCommand(&cobra.Command{Use: "pause", Short: "Pause or resume audio playback", Long: "Toggle pause on the current recitation.", Example: "  peace pause", Args: noArgs, RunE: func(*cobra.Command, []string) error { return runtime.pause() }})
	root.AddCommand(&cobra.Command{Use: "stop", Short: "Stop audio playback", Long: "Stop recitation and any pending wallpaper preparation.", Example: "  peace stop", Args: noArgs, RunE: func(*cobra.Command, []string) error { return runtime.stop() }})
	root.AddCommand(&cobra.Command{Use: "status", Short: "Show playback status", Long: "Show the current ayah and playback state.", Example: "  peace status", Args: noArgs, RunE: func(*cobra.Command, []string) error { return runtime.status() }})
	root.AddCommand(newWallpaperCommand(runtime, "wallpaper", "Set an ayah wallpaper", "  peace wallpaper\n  peace wallpaper next\n  peace wallpaper --random"))
	for _, action := range []string{"random", "next", "prev", "current"} {
		action := action
		short := map[string]string{"random": "Set a random ayah wallpaper", "next": "Show the next ayah wallpaper", "prev": "Show the previous ayah wallpaper", "current": "Re-render the current ayah wallpaper"}[action]
		cmd := &cobra.Command{Use: action, Short: short, Long: short + ".", Example: "  peace " + action, Args: noArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			return runtime.wallpaper(wallpaperOptions{
				Action: action, Random: flagBool(cmd, "random"),
				Translation: resolveTranslation(cmd, runtime),
				WBWLanguage: resolveWBWLanguage(cmd, runtime),
				Render:      wallpaperRenderOptions(cmd),
			})
		}}
		addWallpaperFlags(cmd, runtime)
		root.AddCommand(cmd)
	}
	root.AddCommand(&cobra.Command{Use: "__follow", Hidden: true, Args: noArgs, RunE: func(*cobra.Command, []string) error { return runtime.follow() }})
	root.AddCommand(&cobra.Command{Use: "__prepare SURAH AYAH", Hidden: true, Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error { return runtime.prepare(args) }})
	root.AddCommand(&cobra.Command{Use: "__track DIRECTION", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error { return runtime.track(args[0]) }})
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

func newListenCommand(runtime Runtime) *cobra.Command {
	settings := runtime.effectiveSettings()
	options := Options{
		Reciter: settings.Reciter, Buffer: 10, Lead: 2, Jobs: 2,
		Render: RenderOptions{
			ShowTranslation:     settings.TranslationEnabled,
			ShowWordTranslation: settings.WBWEnabled,
			ShowReference:       true,
		},
	}
	cmd := &cobra.Command{
		Use:     "listen SURAH[:AYAH|:START-END|:START-]",
		Short:   "Play Quran recitation",
		Long:    "Play one ayah, an ayah range, or the rest of a surah. Use --follow to update the wallpaper as playback advances. Wallpaper translation and reference are shown by default and can be hidden. Use --use-theme to keep the active Omarchy text colors instead of adapting colors to each wallpaper.",
		Example: "  peace listen 2:255\n  peace listen 18:1-10 --reciter 7\n  peace listen 110 --follow --random\n  peace listen 110 --follow --use-theme",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return &cliUsageError{err: errors.New("peace listen: ayah reference is required")}
			}
			if _, _, _, err := parseReference(args[0]); err != nil {
				return &cliUsageError{err: err}
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			for _, pair := range []struct {
				name            string
				value, min, max int
			}{{"buffer", options.Buffer, 1, 20}, {"lead", options.Lead, 1, 5}, {"jobs", options.Jobs, 1, 4}} {
				if pair.value < pair.min || pair.value > pair.max {
					return &cliUsageError{err: fmt.Errorf("peace listen: --%s must be between %d and %d (example: --%s %d)", pair.name, pair.min, pair.max, pair.name, pair.min)}
				}
			}
			if !safeID(options.Reciter) {
				return &cliUsageError{err: errors.New("peace listen: invalid reciter ID (example: --reciter 7; run `peace reciters`)")}
			}
			options.Translation = resolveTranslation(cmd, runtime)
			options.WBWLanguage = resolveWBWLanguage(cmd, runtime)
			if !safeID(options.Translation) {
				return &cliUsageError{err: errors.New("peace listen: invalid translation ID (example: --translation 20; run `peace translations`)")}
			}
			if !safeID(options.WBWLanguage) {
				return &cliUsageError{err: errors.New("peace listen: invalid word-by-word language (example: --wbw-language ur; run `peace languages`)")}
			}
			if options.Random && !options.Follow {
				return &cliUsageError{err: errors.New("peace listen: --random requires --follow (example: peace listen 110 --follow --random)")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Reference = args[0]
			options.Translation = resolveTranslation(cmd, runtime)
			options.WBWLanguage = resolveWBWLanguage(cmd, runtime)
			return runtime.listenOptions(options)
		},
	}
	cmd.Flags().StringVar(&options.WBWLanguage, "wbw-language", "", "Word-by-word meaning language")
	cmd.Flags().BoolVar(&options.Follow, "follow", false, "Update wallpaper as each ayah plays")
	cmd.Flags().BoolVar(&options.Random, "random", false, "Shuffle backgrounds between ayahs (requires --follow)")
	cmd.Flags().BoolVar(&options.Render.UseTheme, "use-theme", false, "Use active Omarchy colors instead of wallpaper-adapted colors")
	cmd.Flags().BoolVar(&options.Render.ShowTranslation, "show-translation", options.Render.ShowTranslation, "Show the ayah translation on the wallpaper")
	cmd.Flags().BoolVar(&options.Render.ShowWordTranslation, "show-word-translation", options.Render.ShowWordTranslation, "Show word-by-word meanings on the wallpaper")
	cmd.Flags().BoolVar(&options.Render.ShowReference, "show-reference", true, "Show the surah and ayah reference on the wallpaper")
	cmd.Flags().BoolVar(&options.Basmala, "basmala", false, "Show and play the introductory Basmala")
	cmd.Flags().StringVar(&options.Reciter, "reciter", options.Reciter, "Reciter ID")
	cmd.Flags().IntVar(&options.Buffer, "buffer", 10, "Upcoming ayahs to buffer (1-20)")
	cmd.Flags().IntVar(&options.Lead, "lead", 2, "Wallpapers to prepare ahead (1-5)")
	cmd.Flags().IntVar(&options.Jobs, "jobs", 2, "Concurrent wallpaper preparations (1-4)")
	return cmd
}

func newWallpaperCommand(runtime Runtime, use, short, example string) *cobra.Command {
	cmd := &cobra.Command{Use: use + " [random|next|prev|current]", Short: short, Long: short + ".", Example: example,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &cliUsageError{err: fmt.Errorf("peace wallpaper: expected at most one action, got %d", len(args))}
			}
			if len(args) == 1 && args[0] != "random" && args[0] != "next" && args[0] != "prev" && args[0] != "current" {
				return &cliUsageError{err: fmt.Errorf("peace wallpaper: invalid action %q (expected random, next, prev, or current; example: peace wallpaper next)", args[0])}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			action := "wallpaper"
			if len(args) == 1 {
				action = args[0]
			}
			return runtime.wallpaper(wallpaperOptions{
				Action: action, Random: flagBool(cmd, "random"),
				Translation: resolveTranslation(cmd, runtime),
				WBWLanguage: resolveWBWLanguage(cmd, runtime),
				Render:      wallpaperRenderOptions(cmd),
			})
		}}
	addWallpaperFlags(cmd, runtime)
	return cmd
}

func addWallpaperFlags(cmd *cobra.Command, runtime Runtime) {
	settings := runtime.effectiveSettings()
	cmd.Flags().Bool("random", false, "Choose a random current background")
	cmd.Flags().Bool("use-theme", false, "Use active Omarchy colors instead of wallpaper-adapted colors")
	cmd.Flags().Bool("show-translation", settings.TranslationEnabled, "Show the ayah translation")
	cmd.Flags().Bool("show-word-translation", settings.WBWEnabled, "Show word-by-word meanings")
	cmd.Flags().Bool("show-reference", true, "Show the surah and ayah reference")
	cmd.Flags().String("wbw-language", "", "Word-by-word meaning language")
}

func wallpaperRenderOptions(cmd *cobra.Command) RenderOptions {
	return RenderOptions{
		UseTheme:            flagBool(cmd, "use-theme"),
		ShowTranslation:     flagBool(cmd, "show-translation"),
		ShowWordTranslation: flagBool(cmd, "show-word-translation"),
		ShowReference:       flagBool(cmd, "show-reference"),
	}
}

func flagString(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}

func flagBool(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func resolveTranslation(cmd *cobra.Command, runtime Runtime) string {
	if cmd != nil {
		if val := flagString(cmd, "translation"); val != "" {
			return val
		}
	}
	return runtime.effectiveSettings().Translation
}

func resolveWBWLanguage(cmd *cobra.Command, runtime Runtime) string {
	if cmd != nil {
		if val := flagString(cmd, "wbw-language"); val != "" {
			return val
		}
	}
	return runtime.effectiveSettings().WBWLanguage
}

func newWordsCommand(runtime Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "words SURAH:AYAH",
		Short:   "Show word-by-word Arabic, transliteration, and meaning",
		Long:    "Print each word of one ayah with its Arabic text, transliteration, and English meaning.",
		Example: "  peace words 2:255\n  peace words 2:255 --translation 20\n  peace words 2:255 --wbw-language ur",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &cliUsageError{err: errors.New("peace words: ayah reference is required (example: peace words 2:255)")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			surah, ayah, err := parseSingleAyah(args[0])
			if err != nil {
				return &cliUsageError{err: err}
			}
			client := quran.New(runtime.Config.CacheDir)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			words, err := client.WordsWithWBW(ctx, surah, ayah, resolveTranslation(cmd, runtime), resolveWBWLanguage(cmd, runtime))
			if err != nil {
				return fmt.Errorf("peace words: %w", err)
			}
			output := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(output, "#\tARABIC\tTRANSLITERATION\tENGLISH")
			for index, word := range words {
				_, _ = fmt.Fprintf(output, "%d\t%s\t%s\t%s\n", index+1, word.Arabic, word.Transliteration, word.Translation)
			}
			return output.Flush()
		},
	}
	cmd.Flags().String("wbw-language", "", "Word-by-word meaning language (for example en, ur, bn)")
	return cmd
}

func parseSingleAyah(reference string) (int, int, error) {
	surah, start, end, err := parseReference(reference)
	if err != nil || end != start || start < 1 {
		return 0, 0, fmt.Errorf("peace words: invalid reference %q (expected SURAH:AYAH; example: peace words 2:255)", reference)
	}
	return surah, start, nil
}

func newListCommand(runtime Runtime, name, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     name,
		Short:   short,
		Long:    short + " from Quran.com. Results are cached in the peace cache directory.",
		Example: "  peace " + name + "\n  peace " + name + " --lang en",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client := quran.New(runtime.Config.CacheDir)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			language := flagString(cmd, "lang")
			switch name {
			case "reciters":
				reciters, listErr := client.Reciters(ctx)
				if listErr != nil {
					return fmt.Errorf("peace reciters: %w", listErr)
				}
				if flagBool(cmd, "json") {
					data, _ := json.Marshal(reciters)
					_, _ = cmd.OutOrStdout().Write(data)
					_, _ = cmd.OutOrStdout().Write([]byte("\n"))
					return nil
				}
				output := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(output, "ID\tNAME\tLANGUAGE")
				for _, reciter := range reciters {
					if languageMatches(reciter.Language, language) {
						_, _ = fmt.Fprintf(output, "%s\t%s\t%s\n", reciter.ID, reciter.Name, reciter.Language)
					}
				}
				return output.Flush()
			case "translations":
				translations, listErr := client.Translations(ctx)
				if listErr != nil {
					return fmt.Errorf("peace translations: %w", listErr)
				}
				if flagBool(cmd, "json") {
					data, _ := json.Marshal(translations)
					_, _ = cmd.OutOrStdout().Write(data)
					_, _ = cmd.OutOrStdout().Write([]byte("\n"))
					return nil
				}
				output := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(output, "ID\tNAME\tLANGUAGE")
				for _, translation := range translations {
					if languageMatches(translation.Language, language) {
						_, _ = fmt.Fprintf(output, "%s\t%s\t%s\n", translation.ID, translation.Name, translation.Language)
					}
				}
				return output.Flush()
			case "languages":
				languages, listErr := client.WBWLanguages(ctx)
				if listErr != nil {
					return fmt.Errorf("peace languages: %w", listErr)
				}
				if flagBool(cmd, "json") {
					data, _ := json.Marshal(languages)
					_, _ = cmd.OutOrStdout().Write(data)
					_, _ = cmd.OutOrStdout().Write([]byte("\n"))
					return nil
				}
				output := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(output, "CODE\tNAME\tNATIVE")
				for _, lang := range languages {
					if languageMatches(lang.Name, language) || languageMatches(lang.ISOCode, language) {
						_, _ = fmt.Fprintf(output, "%s\t%s\t%s\n", lang.ISOCode, lang.Name, lang.NativeName)
					}
				}
				return output.Flush()
			default:
				return fmt.Errorf("peace: unknown list %q", name)
			}
		},
	}
	cmd.Flags().String("lang", "", "Filter by language (for example en, arabic, or urdu)")
	cmd.Flags().Bool("json", false, "Output results as JSON")
	return cmd
}

func newSettingsCommand(_ Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "settings",
		Short:   "View or update persistent peace settings",
		Long:    "Show current settings or update translation and word-by-word language. Settings are stored in config.toml; use `peace config` for the full schema.",
		Example: "  peace settings\n  peace settings --translation 131\n  peace settings --wbw-language ur",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("peace settings: %w", err)
			}
			translation := flagString(cmd, "translation")
			wbwLanguage := flagString(cmd, "wbw-language")
			changed := false
			if translation != "" {
				if err := config.Assign(&cfg, "translation", translation); err != nil {
					return fmt.Errorf("peace settings: %w", err)
				}
				changed = true
			}
			if wbwLanguage != "" {
				if err := config.Assign(&cfg, "wbw_language", wbwLanguage); err != nil {
					return fmt.Errorf("peace settings: %w", err)
				}
				changed = true
			}
			if changed {
				if err := config.Validate(cfg); err != nil {
					return fmt.Errorf("peace settings: %w", err)
				}
				if err := config.Save(cfg); err != nil {
					return fmt.Errorf("peace settings: %w", err)
				}
			}
			output := settingsOutput{Translation: cfg.Translation, WBWLanguage: cfg.WBWLanguage}
			if flagBool(cmd, "json") {
				data, _ := json.Marshal(output)
				_, _ = cmd.OutOrStdout().Write(data)
				_, _ = cmd.OutOrStdout().Write([]byte("\n"))
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "translation: %s\nwbw_language: %s\n", output.Translation, output.WBWLanguage)
			return nil
		},
	}
	cmd.Flags().String("translation", "", "Set translation ID")
	cmd.Flags().String("wbw-language", "", "Set word-by-word meaning language")
	cmd.Flags().Bool("json", false, "Output settings as JSON")
	return cmd
}

type settingsOutput struct {
	Translation string `json:"translation"`
	WBWLanguage string `json:"wbw_language"`
}

var languageNames = map[string]string{
	"en": "english", "ar": "arabic", "ur": "urdu", "id": "indonesian", "fr": "french",
	"tr": "turkish", "bn": "bengali", "ru": "russian", "es": "spanish", "de": "german",
	"nl": "dutch", "pt": "portuguese", "hi": "hindi", "zh": "chinese", "ja": "japanese",
	"ko": "korean", "th": "thai", "ms": "malay", "sw": "swahili", "ha": "hausa", "fa": "persian",
}

func languageMatches(value, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if value == filter {
		return true
	}
	if full, ok := languageNames[filter]; ok {
		return strings.EqualFold(value, full)
	}
	return strings.HasPrefix(value, filter)
}

func loadConfig() (Config, error) {
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
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		state = filepath.Join(stateHome, "peace")
	}
	cache := filepath.Join(home, ".cache", "peace")
	if cacheHome, cacheErr := os.UserCacheDir(); cacheErr == nil {
		cache = filepath.Join(cacheHome, "peace")
	}
	socket := filepath.Join(os.TempDir(), "peace-mpv-"+strconv.Itoa(os.Getuid())+".sock")
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		socket = filepath.Join(runtimeDir, "peace-mpv.sock")
	}
	return Config{Home: home, PluginDir: pluginDir, StateDir: state, CacheDir: cache, Socket: socket}, nil
}

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
