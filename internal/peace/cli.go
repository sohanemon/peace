// NOTE: Keep command routing here and place playback, rendering, and API work in focused modules.
package peace

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func Run(argv0 string, args []string) error {
	cfg, err := loadConfig(argv0)
	if err != nil {
		return err
	}
	runtime := Runtime{Config: cfg}
	name := filepath.Base(argv0)
	if name != "peace" {
		return runtime.runAlias(name, args)
	}
	if len(args) == 0 {
		printUsage()
		return nil
	}

	action, rest := args[0], args[1:]
	switch action {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "listen":
		if len(rest) == 1 && (rest[0] == "--help" || rest[0] == "-h") {
			printUsage()
			return nil
		}
		return runtime.listen(rest)
	case "pause":
		return runtime.pause()
	case "stop":
		return runtime.stop()
	case "status":
		return runtime.status()
	case "install":
		return runtime.installCommands()
	case "__follow":
		return runtime.follow()
	case "__prepare":
		return runtime.prepare(rest)
	case "wallpaper", "random", "next", "prev":
		wallpaperAction := action
		if action == "wallpaper" && len(rest) > 0 {
			wallpaperAction, rest = rest[0], rest[1:]
		}
		if len(rest) != 0 {
			return fmt.Errorf("peace wallpaper: unexpected argument %q", rest[0])
		}
		return runtime.wallpaper(wallpaperAction)
	case "init", "uninstall", "enable", "disable", "logs":
		return runCommand(filepath.Join(runtime.Config.Home, ".local", "bin", "peace-"+action), rest...)
	default:
		return fmt.Errorf("peace: unknown action %q", action)
	}
}

func (r Runtime) runAlias(name string, args []string) error {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		switch name {
		case "peace-pause":
			fmt.Println("Usage: peace pause\nToggle audio pause or resume.")
		case "peace-stop":
			fmt.Println("Usage: peace stop\nStop audio and follow preparation.")
		case "peace-status":
			fmt.Println("Usage: peace status\nShow the current ayah and playback state.")
		default:
			printUsage()
		}
		return nil
	}
	switch name {
	case "peace-listen":
		return r.listen(args)
	case "peace-pause":
		if len(args) != 0 {
			return errors.New("peace-pause: unexpected arguments")
		}
		return r.pause()
	case "peace-stop":
		if len(args) != 0 {
			return errors.New("peace-stop: unexpected arguments")
		}
		return r.stop()
	case "peace-status":
		if len(args) != 0 {
			return errors.New("peace-status: unexpected arguments")
		}
		return r.status()
	default:
		return fmt.Errorf("peace: unknown command name %q", name)
	}
}

func (r Runtime) installCommands() error {
	userBin, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	userBin = filepath.Join(userBin, ".local", "bin")
	if err := os.MkdirAll(userBin, 0o755); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	for _, name := range []string{"peace", "peace-listen", "peace-pause", "peace-stop", "peace-status"} {
		link := filepath.Join(userBin, name)
		if existing, readErr := os.Readlink(link); readErr == nil && existing == executable {
			continue
		}
		if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Symlink(executable, link); err != nil {
			return err
		}
	}
	fontSource := filepath.Join(r.Config.PluginDir, "assets", "fonts", "Amiri-Regular.ttf")
	if input, openErr := os.Open(fontSource); openErr == nil {
		fontDir := filepath.Join(userBin, "..", "share", "fonts", "omapeace")
		if err := os.MkdirAll(fontDir, 0o755); err != nil {
			_ = input.Close()
			return err
		}
		fontTarget := filepath.Join(fontDir, "Amiri-Regular.ttf")
		output, createErr := os.OpenFile(fontTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if createErr != nil {
			_ = input.Close()
			return createErr
		}
		_, copyErr := io.Copy(output, input)
		closeOutputErr, closeInputErr := output.Close(), input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
		if closeInputErr != nil {
			return closeInputErr
		}
		if fcCache, lookErr := exec.LookPath("fc-cache"); lookErr == nil {
			_ = exec.Command(fcCache, "-f", fontDir).Run()
		}
	}
	fmt.Printf("Installed peace commands in %s\n", userBin)
	return nil
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

func parseOptions(args []string) (Options, error) {
	if len(args) == 0 {
		return Options{}, errors.New("peace listen: ayah reference is required")
	}
	o := Options{Reference: args[0], Reciter: os.Getenv("PEACE_RECITER"), Buffer: 10, Lead: 2, Jobs: 2}
	if o.Reciter == "" {
		o.Reciter = "ar.alafasy"
	}
	for i := 1; i < len(args); {
		if args[i] == "--follow" {
			o.Follow = true
			i++
			continue
		}
		if i+1 >= len(args) {
			return Options{}, fmt.Errorf("peace listen: %s requires a value", args[i])
		}
		switch args[i] {
		case "--reciter":
			o.Reciter = args[i+1]
		case "--buffer":
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return Options{}, errors.New("peace listen: invalid --buffer")
			}
			o.Buffer = n
		case "--lead":
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return Options{}, errors.New("peace listen: invalid --lead")
			}
			o.Lead = n
		case "--jobs":
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return Options{}, errors.New("peace listen: invalid --jobs")
			}
			o.Jobs = n
		default:
			return Options{}, fmt.Errorf("peace listen: unknown option %q", args[i])
		}
		i += 2
	}
	for _, pair := range []struct {
		name        string
		n, min, max int
	}{{"buffer", o.Buffer, 1, 20}, {"lead", o.Lead, 1, 5}, {"jobs", o.Jobs, 1, 4}} {
		if pair.n < pair.min || pair.n > pair.max {
			return Options{}, fmt.Errorf("peace listen: --%s must be between %d and %d", pair.name, pair.min, pair.max)
		}
	}
	if !safeID(o.Reciter) {
		return Options{}, errors.New("peace listen: invalid reciter ID")
	}
	return o, nil
}

func safeID(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._-", c)) {
			return false
		}
	}
	return true
}

func printUsage() {
	fmt.Println("Usage: peace <action> [options]")
	fmt.Println("  Actions: listen, pause, stop, status, wallpaper, random, next, prev, install, help")
}
