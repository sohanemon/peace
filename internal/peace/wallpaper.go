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
)

func (r Runtime) wallpaper(action string, randomBackground, useTheme bool) error {
	if r.pidAlive(r.followingFile()) {
		fmt.Println("peace: follow is active; skipping wallpaper change")
		return nil
	}
	absoluteAyah, err := r.chooseWallpaperAyah(action)
	if err != nil {
		return err
	}
	surah, ayah, text, err := fetchAyahByAbsolute(absoluteAyah)
	if err != nil {
		return err
	}
	base, err := r.snapshotBackground(randomBackground)
	if err != nil {
		return err
	}
	outputDir := filepath.Join(r.Config.Home, ".local", "share", "omapeace", "backgrounds")
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return err
	}
	output := filepath.Join(outputDir, fmt.Sprintf("ayah-%d-%d.png", time.Now().UnixNano(), absoluteAyah))
	if err := r.renderAyahText(surah, ayah, text, base, output, useTheme); err != nil {
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

func fetchAyahByAbsolute(number int) (int, int, ayahText, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://api.alquran.cloud/v1/ayah/%d/editions/quran-uthmani,en.sahih", number)
	data, err := fetchAyahEditions(ctx, url)
	if err != nil {
		return 0, 0, ayahText{}, err
	}
	surah := data[0].Surah.Number
	ayah := data[0].NumberInSurah
	text := ayahText{
		Arabic:    data[0].Text,
		English:   data[1].Text,
		Reference: fmt.Sprintf("%s %d:%d", data[0].Surah.EnglishName, surah, ayah),
	}
	return surah, ayah, text, nil
}

func (r Runtime) wallpaperStateFile() string {
	return filepath.Join(r.Config.Home, ".local", "state", "omapeace", "wallpaper-ayah")
}
