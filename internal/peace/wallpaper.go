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

	"io.github.sohanemon/peace/internal/quran"
)

type wallpaperOptions struct {
	Action      string
	Random      bool
	Translation string
	WBWLanguage string
	Render      RenderOptions
}

func (r Runtime) wallpaper(options wallpaperOptions) error {
	if r.pidAlive(r.followingFile()) {
		if options.Action == "current" {
			if data, err := os.ReadFile(filepath.Join(r.Config.StateDir, "displayed-ayah")); err == nil {
				parts := strings.Split(strings.TrimSpace(string(data)), "-")
				if len(parts) == 2 {
					surah, _ := strconv.Atoi(parts[0])
					ayah, _ := strconv.Atoi(parts[1])
					if surah > 0 && ayah > 0 {
						client := quran.New(r.Config.CacheDir)
						ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
						defer cancel()
						cache := filepath.Join(r.Config.CacheDir, "follow")
						baseImage := ""
						if src, err := os.ReadFile(filepath.Join(cache, "base-source")); err == nil {
							baseImage = strings.TrimSpace(string(src))
						}
						if baseImage == "" {
							baseImage, _ = r.snapshotBackground(false)
						}
						target := followArtifact(cache, surah, ayah, ".png")
						if entry, err := client.AyahWithWBW(ctx, surah, ayah, options.Translation, options.WBWLanguage); err == nil {
							entry.Arabic = displayAyahArabic(surah, ayah, entry.Arabic, false)
							_ = r.renderAyahText(entry, baseImage, target, options.Render)
							_ = os.Remove(filepath.Join(r.Config.StateDir, "displayed-ayah"))
							r.displayWallpaper(cache, surah, ayah)
						}
						if state, err := r.readState(); err == nil {
							state.Translation = options.Translation
							state.WBWLanguage = options.WBWLanguage
							state.Render = options.Render
							_ = r.writeState(state)
						}
						return nil
					}
				}
			}
		}
		fmt.Println("peace: follow is active; skipping wallpaper change")
		return nil
	}
	absoluteAyah, err := r.chooseWallpaperAyah(options.Action)
	if err != nil {
		return err
	}
	client := quran.New(r.Config.CacheDir)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	ayah, err := fetchAyahByAbsolute(ctx, client, absoluteAyah, options.Translation, options.WBWLanguage)
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
	if err := r.renderAyahText(ayah, base, output, options.Render); err != nil {
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
	case "current":
		data, err := os.ReadFile(r.wallpaperStateFile())
		if err != nil {
			return 1, nil
		}
		current, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || current < 1 || current > total {
			return 1, nil
		}
		return current, nil
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

func fetchAyahByAbsolute(ctx context.Context, client *quran.Client, number int, translation, wbwLanguage string) (quran.Ayah, error) {
	chapters, err := client.Chapters(ctx)
	if err != nil {
		return quran.Ayah{}, err
	}
	surah, ayah, err := chapterForAbsolute(chapters, number)
	if err != nil {
		return quran.Ayah{}, err
	}
	return client.AyahWithWBW(ctx, surah, ayah, translation, wbwLanguage)
}

func chapterForAbsolute(chapters []quran.Chapter, number int) (int, int, error) {
	remaining := number
	for _, chapter := range chapters {
		if remaining <= chapter.Verses {
			return chapter.Number, remaining, nil
		}
		remaining -= chapter.Verses
	}
	return 0, 0, fmt.Errorf("ayah number %d is out of range", number)
}

func (r Runtime) wallpaperStateFile() string {
	return filepath.Join(r.Config.Home, ".local", "state", "omapeace", "wallpaper-ayah")
}
