package peace

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"io.github.sohanemon/peace/internal/source"
)

type wallpaperOptions struct {
	Action      string
	Random      bool
	Source      string
	Fallback    string
	Translation string
	Render      RenderOptions
}

func (r Runtime) wallpaper(options wallpaperOptions) error {
	if r.pidAlive(r.followingFile()) {
		fmt.Println("peace: follow is active; skipping wallpaper change")
		return nil
	}
	absoluteAyah, err := r.chooseWallpaperAyah(options.Action)
	if err != nil {
		return err
	}
	dataSource, err := r.library(options.Source, options.Fallback)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	surah, ayah, text, err := fetchAyahByAbsolute(ctx, dataSource, absoluteAyah, options.Translation)
	if err != nil {
		return err
	}
	base, err := r.snapshotBackground(options.Random)
	if err != nil {
		return err
	}
	outputDir := filepath.Join(r.Config.Home, ".local", "share", "omapeace", "backgrounds")
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return err
	}
	output := filepath.Join(outputDir, fmt.Sprintf("ayah-%d-%d.png", time.Now().UnixNano(), absoluteAyah))
	if err := r.renderAyahText(surah, ayah, text, base, output, options.Render); err != nil {
		return err
	}
	cmd := exec.Command("omarchy", "theme", "bg", "set", output)
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("peace wallpaper: set background: %s: %w", strings.TrimSpace(string(combined)), err)
	}
	if err := writeAtomic(r.wallpaperStateFile(), []byte(strconv.Itoa(absoluteAyah)+"\n"), 0o600); err != nil {
		return err
	}
	if source, readErr := os.ReadFile(filepath.Join(r.Config.CacheDir, "follow", "base-source")); readErr == nil {
		_ = os.WriteFile(output+".base", source, 0o600)
	}
	return nil
}

func (r Runtime) chooseWallpaperAyah(action string) (int, error) {
	const total = 6236
	switch action {
	case "", "wallpaper", "random":
		return rand.Intn(total) + 1, nil
	case "next", "prev":
		data, err := os.ReadFile(r.wallpaperStateFile())
		if err != nil {
			return 0, errors.New("peace wallpaper: no current ayah; use 'peace random' first")
		}
		current, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || current < 1 || current > total {
			return 0, errors.New("peace wallpaper: invalid current ayah state")
		}
		if action == "next" {
			return current%total + 1, nil
		}
		return (current+total-2)%total + 1, nil
	default:
		return 0, fmt.Errorf("peace wallpaper: unknown action %q", action)
	}
}

func fetchAyahByAbsolute(ctx context.Context, dataSource source.Library, number int, translation string) (int, int, ayahText, error) {
	chapters, err := dataSource.Chapters(ctx)
	if err != nil {
		return 0, 0, ayahText{}, err
	}
	remaining := number
	for _, chapter := range chapters {
		if remaining <= chapter.Verses {
			text, textErr := fetchAyahText(ctx, dataSource, chapter.Number, remaining, translation)
			if textErr != nil {
				return 0, 0, ayahText{}, textErr
			}
			return chapter.Number, remaining, text, nil
		}
		remaining -= chapter.Verses
	}
	return 0, 0, ayahText{}, fmt.Errorf("ayah number %d is out of range", number)
}

func (r Runtime) wallpaperStateFile() string {
	return filepath.Join(r.Config.Home, ".local", "state", "omapeace", "wallpaper-ayah")
}
