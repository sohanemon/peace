package peace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var referencePattern = regexp.MustCompile(`^([0-9]+)(?::([0-9]+)(?:-([0-9]*))?)?$`)

func (r Runtime) listenOptions(o Options) error {
	surah, start, end, err := parseReference(o.Reference)
	if err != nil {
		return err
	}
	if err := r.stop(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	counts, err := r.surahCounts(ctx)
	if err != nil {
		return fmt.Errorf("peace listen: unable to fetch surah list: %w", err)
	}
	count := counts[surah]
	if count == 0 || start < 1 {
		return errors.New("peace listen: invalid ayah number")
	}
	if end == 0 {
		end = count
	}
	if start > count || end > count {
		return fmt.Errorf("peace listen: surah %d has %d ayahs", surah, count)
	}
	if start > end {
		return errors.New("peace listen: ayah range start must be less than or equal to end")
	}
	urls, err := r.audioURLs(ctx, surah, start, end, o.Reciter)
	if err != nil {
		return fmt.Errorf("peace listen: unable to fetch audio for surah %d (%s): %w", surah, o.Reciter, err)
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		return errors.New("peace listen: mpv is required")
	}
	baseImage := ""
	if o.Follow {
		baseImage, err = r.snapshotBackground(o.Random)
		if err != nil {
			return fmt.Errorf("peace listen: snapshot current wallpaper: %w", err)
		}
	}
	state := PlayState{Surah: surah, Start: start, End: end, Reciter: o.Reciter, Follow: o.Follow, BaseImage: baseImage, Preparing: o.Follow}
	if o.Follow {
		state.AudioURLs, state.Buffer, state.Lead, state.Jobs = urls, o.Buffer, o.Lead, o.Jobs
		if err := os.MkdirAll(r.Config.StateDir, 0o700); err != nil {
			return err
		}
		fmt.Println("Preparing first wallpapers…")
		leadEnd := min(end, start+o.Lead-1)
		for ayah := start; ayah <= leadEnd; ayah++ {
			if err := r.prepareInitialWallpaper(surah, ayah, baseImage); err != nil {
				if ayah == start {
					return fmt.Errorf("peace listen: prepare first ayah wallpaper: %w", err)
				}
				cache := filepath.Join(r.Config.CacheDir, "follow")
				_ = os.WriteFile(filepath.Join(cache, fmt.Sprintf("%d-%d.skip", surah, ayah)), nil, 0o600)
				r.logEvent(fmt.Sprintf("startup wallpaper skipped %d:%d: %v", surah, ayah, err))
			}
		}
		if !fileExists(filepath.Join(r.Config.CacheDir, "follow", fmt.Sprintf("%d-%d.png", surah, start))) {
			return errors.New("peace listen: first ayah wallpaper could not be prepared")
		}
		r.displayWallpaper(filepath.Join(r.Config.CacheDir, "follow"), surah, start)
	}
	if err := r.writeState(state); err != nil {
		return err
	}
	if o.Follow {
		if err := r.startMPV([]string{"--idle=yes", "--no-video", "--no-terminal", "--gapless-audio=yes", "--prefetch-playlist=yes", "--input-ipc-server=" + r.Config.Socket}); err != nil {
			_ = r.stop()
			return err
		}
		if err := r.appendAudio(urls[0], fmt.Sprintf("%d:%d", surah, start)); err != nil {
			_ = r.stop()
			return fmt.Errorf("peace listen: start first ayah: %w", err)
		}
		state.Appended, state.Preparing = 1, false
		if err := r.writeState(state); err != nil {
			_ = r.stop()
			return err
		}
	} else {
		args := []string{"--no-video", "--no-terminal", "--gapless-audio=yes", "--prefetch-playlist=yes", "--input-ipc-server=" + r.Config.Socket}
		args = append(args, urls...)
		if err := r.startMPV(args); err != nil {
			_ = r.stop()
			return err
		}
	}
	if o.Follow {
		if err := r.startWorker("__follow"); err != nil {
			_ = r.stop()
			return err
		}
		fmt.Println("Playback started; buffering upcoming ayahs and wallpapers")
	}
	return nil
}

func parseReference(ref string) (int, int, int, error) {
	m := referencePattern.FindStringSubmatch(ref)
	if m == nil {
		return 0, 0, 0, fmt.Errorf("peace listen: invalid reference %q (expected SURAH[:AYAH|:START-END|:START-]; example: peace listen 2:255)", ref)
	}
	surah, _ := strconv.Atoi(m[1])
	if surah < 1 || surah > 114 {
		return 0, 0, 0, errors.New("peace listen: surah must be between 1 and 114 (example: peace listen 2:255)")
	}
	start, end := 1, 0
	if m[2] != "" {
		start, _ = strconv.Atoi(m[2])
		end = start
	}
	if m[3] != "" {
		end, _ = strconv.Atoi(m[3])
	}
	if len(m) > 3 && m[2] != "" && ref[len(ref)-1] == '-' && m[3] == "" {
		end = 0
	}
	return surah, start, end, nil
}

func (r Runtime) surahCounts(ctx context.Context) (map[int]int, error) {
	path := filepath.Join(r.Config.CacheDir, "surahs.json")
	var cached []Surah
	if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) < 30*24*time.Hour {
		if data, readErr := os.ReadFile(path); readErr == nil && json.Unmarshal(data, &cached) == nil {
			return countMap(cached), nil
		}
	}
	var response APIResponse
	if err := getJSON(ctx, "https://api.alquran.cloud/v1/surah", &response); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(response.Data, &cached); err != nil {
		return nil, err
	}
	if len(cached) != 114 {
		return nil, errors.New("incomplete surah list received")
	}
	data, _ := json.Marshal(cached)
	if err := writeAtomic(path, data, 0o600); err != nil {
		return nil, err
	}
	return countMap(cached), nil
}

func countMap(surahs []Surah) map[int]int {
	out := make(map[int]int, len(surahs))
	for _, surah := range surahs {
		out[surah.Number] = surah.NumberOfAyahs
	}
	return out
}

func (r Runtime) audioURLs(ctx context.Context, surah, start, end int, reciter string) ([]string, error) {
	var response APIResponse
	url := fmt.Sprintf("https://api.alquran.cloud/v1/surah/%d/%s", surah, reciter)
	if err := getJSON(ctx, url, &response); err != nil {
		return nil, err
	}
	var data SurahAudio
	if err := json.Unmarshal(response.Data, &data); err != nil {
		return nil, err
	}
	urls := make([]string, 0, end-start+1)
	for _, ayah := range data.Ayahs {
		if ayah.NumberInSurah >= start && ayah.NumberInSurah <= end {
			if ayah.Audio == "" {
				return nil, errors.New("audio URL missing")
			}
			urls = append(urls, ayah.Audio)
		}
	}
	if len(urls) != end-start+1 {
		return nil, errors.New("audio URLs are incomplete")
	}
	return urls, nil
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// openURL performs a GET with context cancellation and a 2xx status check.
func openURL(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_ = response.Body.Close()
		return nil, fmt.Errorf("HTTP %s", response.Status)
	}
	return response, nil
}

func getJSON(ctx context.Context, url string, target any) error {
	response, err := openURL(ctx, url)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	return json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(target)
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
