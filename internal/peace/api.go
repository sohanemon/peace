package peace

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"io.github.sohanemon/peace/internal/quran"
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
	client := quran.New(r.Config.CacheDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	counts, err := r.surahCounts(ctx, client)
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
	urls, err := r.audioURLs(ctx, client, surah, start, end, o.Reciter)
	if err != nil {
		return fmt.Errorf("peace listen: unable to fetch audio for surah %d (%s): %w", surah, o.Reciter, err)
	}
	introURL := ""
	if shouldUseBasmala(o.Basmala, surah, start) {
		introURL, err = r.basmalaAudioURL(ctx, client, o.Reciter)
		if err != nil {
			return fmt.Errorf("peace listen: unable to fetch introductory Basmala for %s: %w", o.Reciter, err)
		}
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		return errors.New("peace listen: mpv is required")
	}
	baseImage := ""
	if o.Follow {
		baseImage, err = r.snapshotBackground(false)
		if err != nil {
			return fmt.Errorf("peace listen: snapshot current wallpaper: %w", err)
		}
	}
	state := PlayState{Surah: surah, Start: start, End: end, Reciter: o.Reciter, Translation: o.Translation, Follow: o.Follow, Random: o.Random, Render: o.Render, Basmala: o.Basmala, BaseImage: baseImage, Preparing: o.Follow}
	if o.Follow && o.Random {
		state.RandomSources = r.backgroundCandidates()
		rand.Shuffle(len(state.RandomSources), func(i, j int) {
			state.RandomSources[i], state.RandomSources[j] = state.RandomSources[j], state.RandomSources[i]
		})
	}
	if o.Follow {
		state.AudioURLs, state.IntroAudioURL, state.Buffer, state.Lead, state.Jobs = urls, introURL, o.Buffer, o.Lead, o.Jobs
		if err := os.MkdirAll(r.Config.StateDir, 0o700); err != nil {
			return err
		}
		fmt.Println("Preparing first wallpapers…")
		leadEnd := min(end, start+o.Lead-1)
		for ayah := start; ayah <= leadEnd; ayah++ {
			wallpaperBase := backgroundForAyah(state.RandomSources, start, ayah)
			if wallpaperBase == "" {
				wallpaperBase = baseImage
			}
			request := wallpaperRequest{Client: client, Translation: o.Translation, Surah: surah, Ayah: ayah, BaseImage: wallpaperBase, Basmala: o.Basmala, Render: o.Render}
			if err := r.prepareInitialWallpaper(ctx, request); err != nil {
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
		if introURL != "" {
			if err := r.appendAudio(introURL, "Introductory Basmala"); err != nil {
				_ = r.stop()
				return fmt.Errorf("peace listen: start introductory Basmala: %w", err)
			}
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
		args = append(args, playbackAudioURLs(urls, introURL)...)
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

func needsIntroductoryBasmala(surah, start int) bool {
	return start == 1 && hasIntroductoryBasmala(surah)
}

func hasIntroductoryBasmala(surah int) bool {
	return surah != 1 && surah != 9
}

func shouldUseBasmala(enabled bool, surah, start int) bool {
	return enabled && needsIntroductoryBasmala(surah, start)
}

func playbackAudioURLs(urls []string, introURL string) []string {
	ordered := make([]string, 0, len(urls)+1)
	if introURL != "" {
		ordered = append(ordered, introURL)
	}
	return append(ordered, urls...)
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

func (r Runtime) surahCounts(ctx context.Context, client *quran.Client) (map[int]int, error) {
	chapters, err := client.Chapters(ctx)
	if err != nil {
		return nil, err
	}
	if len(chapters) != 114 {
		return nil, errors.New("incomplete surah list received")
	}
	out := make(map[int]int, len(chapters))
	for _, chapter := range chapters {
		out[chapter.Number] = chapter.Verses
	}
	return out, nil
}

func (r Runtime) audioURLs(ctx context.Context, client *quran.Client, surah, start, end int, reciter string) ([]string, error) {
	urls := make([]string, 0, end-start+1)
	for ayah := start; ayah <= end; ayah++ {
		url, err := client.AudioURL(ctx, reciter, surah, ayah)
		if err != nil {
			return nil, err
		}
		if url == "" {
			return nil, errors.New("audio URL missing")
		}
		urls = append(urls, url)
	}
	return urls, nil
}

func (r Runtime) basmalaAudioURL(ctx context.Context, client *quran.Client, reciter string) (string, error) {
	url, err := client.AudioURL(ctx, reciter, 1, 1)
	if err != nil {
		return "", err
	}
	if url == "" {
		return "", errors.New("Basmala audio URL missing")
	}
	return url, nil
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
