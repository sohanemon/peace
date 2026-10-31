package peace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (r Runtime) prepare(args []string) error {
	if len(args) != 3 {
		return errors.New("peace: invalid preparation request")
	}
	surah, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}
	ayah, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}
	ref := fmt.Sprintf("%d:%d", surah, ayah)
	cache := filepath.Join(r.Config.CacheDir, "follow")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return err
	}
	image := filepath.Join(cache, fmt.Sprintf("%d-%d.png", surah, ayah))
	audio := filepath.Join(cache, fmt.Sprintf("%d-%d.mp3", surah, ayah))
	var downloadErr, renderErr error
	done := make(chan struct{})
	go func() { downloadErr = downloadFile(args[2], audio+".tmp"); close(done) }()
	for attempt := 1; attempt <= 2; attempt++ {
		cmd := exec.Command(filepath.Join(r.Config.ThemeDir, "bin", "peace-wallpaper"), "--ref", ref, "--out", image+".tmp")
		if output, err := cmd.CombinedOutput(); err == nil {
			renderErr = os.Rename(image+".tmp", image)
			if renderErr == nil {
				break
			}
		} else {
			renderErr = fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
		}
		_ = os.Remove(image + ".tmp")
	}
	<-done
	if downloadErr == nil {
		if err := os.Rename(audio+".tmp", audio); err != nil {
			downloadErr = err
		}
	}
	if downloadErr != nil {
		_ = os.Remove(audio + ".tmp")
		_ = os.WriteFile(filepath.Join(cache, fmt.Sprintf("%d-%d.audio-fail", surah, ayah)), nil, 0o600)
	}
	if renderErr != nil {
		_ = os.WriteFile(filepath.Join(cache, fmt.Sprintf("%d-%d.skip", surah, ayah)), nil, 0o600)
	}
	if downloadErr != nil && renderErr != nil {
		return fmt.Errorf("peace: preparation failed for %s", ref)
	}
	return nil
}

func downloadFile(url, destination string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %s", response.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(destination)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(destination)
		return closeErr
	}
	return nil
}

func (r Runtime) follow() error {
	state, err := r.readState()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(r.Config.CacheDir, "follow"), 0o700); err != nil {
		return err
	}
	pidText := strconv.Itoa(os.Getpid()) + "\n"
	_ = os.WriteFile(r.followingFile(), []byte(pidText), 0o600)
	_ = os.WriteFile(r.supervisorFile(), []byte(pidText), 0o600)
	defer r.finishFollow()
	cache := filepath.Join(r.Config.CacheDir, "follow")
	total := state.End - state.Start + 1
	prepCount := min(state.Lead, total, state.Buffer+2)
	prepEnd := state.Start + prepCount - 1
	prep := map[int]int{}
	appendIndex, current := 0, 0
	warmup, started := true, r.Now()
	lastTitle, waitRef := "", ""
	waitStarted := time.Time{}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for r.mpvAlive() {
		<-ticker.C
		now := r.Now()
		idle, _ := r.property("idle-active")
		position, _ := r.property("playlist-pos")
		count, _ := r.property("playlist-count")
		title, _ := r.property("media-title")
		paused, _ := r.property("pause")
		idleActive, _ := idle.(bool)
		positionValue, hasPosition := position.(float64)
		playlistCount, hasCount := count.(float64)
		mediaTitle, _ := title.(string)
		if !idleActive {
			ayah, ok := titleAyah(mediaTitle, state.Surah)
			if !ok && hasPosition {
				ayah = state.Start + int(positionValue)
				ok = true
				mediaTitle = fmt.Sprintf("%d:%d", state.Surah, ayah)
			}
			if ok {
				current = ayah
				if mediaTitle != lastTitle {
					r.logEvent("audio started " + mediaTitle)
					lastTitle = mediaTitle
				}
				r.displayWallpaper(cache, state.Surah, current)
			}
		}
		if warmup {
			wallpapers, audioReady := 0, false
			for ayah := state.Start; ayah <= prepEnd; ayah++ {
				if fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.png", state.Surah, ayah))) || fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.skip", state.Surah, ayah))) {
					wallpapers++
				}
				if ayah == state.Start && (fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.mp3", state.Surah, ayah))) || fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.audio-fail", state.Surah, ayah)))) {
					audioReady = true
				}
			}
			state.Ready, state.Preparing = wallpapers, true
			_ = r.writeState(state)
			if wallpapers >= prepCount && audioReady {
				warmup = false
				state.Preparing = false
				_ = r.writeState(state)
				fmt.Println()
			}
			if now.Sub(started) > 45*time.Second {
				for ayah := state.Start; ayah <= prepEnd; ayah++ {
					if !fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.png", state.Surah, ayah))) {
						_ = os.WriteFile(filepath.Join(cache, fmt.Sprintf("%d-%d.skip", state.Surah, ayah)), nil, 0o600)
					}
					if !fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.mp3", state.Surah, ayah))) {
						_ = os.WriteFile(filepath.Join(cache, fmt.Sprintf("%d-%d.audio-fail", state.Surah, ayah)), nil, 0o600)
					}
					if pid := prep[ayah]; pid > 0 {
						_ = syscall.Kill(-pid, syscall.SIGTERM)
						delete(prep, ayah)
					}
				}
				warmup = false
				state.Preparing = false
				_ = r.writeState(state)
			}
		}
		appendLimit := max(0, current-state.Start+1) + state.Buffer
		if appendLimit > total {
			appendLimit = total
		}
		if !warmup && appendIndex < appendLimit {
			ayah := state.Start + appendIndex
			png := filepath.Join(cache, fmt.Sprintf("%d-%d.png", state.Surah, ayah))
			mp3 := filepath.Join(cache, fmt.Sprintf("%d-%d.mp3", state.Surah, ayah))
			imageResolved := fileExists(png) || fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.skip", state.Surah, ayah)))
			audioResolved := fileExists(mp3) || fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.audio-fail", state.Surah, ayah)))
			if imageResolved && audioResolved {
				audioSource := mp3
				if !fileExists(mp3) && appendIndex < len(state.AudioURLs) {
					audioSource = state.AudioURLs[appendIndex]
				}
				response, sendErr := r.sendIPC(map[string]any{"command": []any{"loadfile", audioSource, "append-play", map[string]string{"force-media-title": fmt.Sprintf("%d:%d", state.Surah, ayah)}}, "request_id": 1})
				if sendErr == nil && response["error"] == "success" {
					appendIndex++
					state.Appended = appendIndex
					_ = r.writeState(state)
					waitRef = ""
				}
			} else {
				key := state.Surah*1000 + ayah
				if waitRef != fmt.Sprint(key) {
					waitRef = fmt.Sprint(key)
					waitStarted = now
				}
				if now.Sub(waitStarted) >= 45*time.Second {
					if !imageResolved {
						_ = os.WriteFile(filepath.Join(cache, fmt.Sprintf("%d-%d.skip", state.Surah, ayah)), nil, 0o600)
					}
					if !audioResolved {
						_ = os.WriteFile(filepath.Join(cache, fmt.Sprintf("%d-%d.audio-fail", state.Surah, ayah)), nil, 0o600)
					}
					if pid := prep[ayah]; pid > 0 {
						_ = syscall.Kill(-pid, syscall.SIGTERM)
						delete(prep, ayah)
					}
				}
			}
		}
		base, window := state.Start+appendIndex, state.Buffer
		if warmup {
			base, window = state.Start, prepCount
		} else if current > 0 {
			base = current + 1
		}
		if pausedValue, ok := paused.(bool); ok && pausedValue && current == 0 && !warmup {
			window = max(0, state.Buffer-appendIndex)
		}
		end := min(state.End, base+window-1)
		active := 0
		for ayah, pid := range prep {
			if syscall.Kill(pid, 0) == nil {
				active++
			} else {
				delete(prep, ayah)
				_ = os.Remove(filepath.Join(r.Config.StateDir, fmt.Sprintf("prep-%d.pid", ayah)))
			}
		}
		limit := 1
		if warmup {
			limit = state.Jobs
		} else {
			ready := 0
			for ayah := base; ayah <= end; ayah++ {
				if fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.png", state.Surah, ayah))) && fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.mp3", state.Surah, ayah))) {
					ready++
				}
			}
			if ready < 3 {
				limit = state.Jobs
			}
		}
		for ayah := base; ayah <= end && active < limit; ayah++ {
			if _, exists := prep[ayah]; exists {
				continue
			}
			if fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.png", state.Surah, ayah))) && fileExists(filepath.Join(cache, fmt.Sprintf("%d-%d.mp3", state.Surah, ayah))) {
				continue
			}
			if ayah-state.Start >= len(state.AudioURLs) {
				continue
			}
			pid, startErr := r.startPrepare(state.Surah, ayah, state.AudioURLs[ayah-state.Start])
			if startErr == nil {
				prep[ayah] = pid
				active++
			}
		}
		if appendIndex >= total && idleActive && hasCount && int(playlistCount) >= total {
			_, _ = r.sendIPC(map[string]any{"command": []any{"quit"}, "request_id": 2})
			return nil
		}
	}
	return nil
}

func (r Runtime) startPrepare(surah, ayah int, url string) (int, error) {
	executable, err := os.Executable()
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(executable, "__prepare", strconv.Itoa(surah), strconv.Itoa(ayah), url)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	logFile, err := os.OpenFile(filepath.Join(r.Config.StateDir, "follow.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, logFile, logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return 0, err
	}
	_ = logFile.Close()
	_ = os.WriteFile(filepath.Join(r.Config.StateDir, fmt.Sprintf("prep-%d.pid", ayah)), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o600)
	pid := cmd.Process.Pid
	return pid, cmd.Process.Release()
}

func (r Runtime) displayWallpaper(cache string, surah, ayah int) {
	ref := fmt.Sprintf("%d:%d", surah, ayah)
	shown, _ := os.ReadFile(filepath.Join(r.Config.StateDir, "displayed-ayah"))
	if strings.TrimSpace(string(shown)) == fmt.Sprintf("%d-%d", surah, ayah) {
		return
	}
	image := filepath.Join(cache, fmt.Sprintf("%d-%d.png", surah, ayah))
	if !fileExists(image) {
		return
	}
	backgroundDir := filepath.Join(r.Config.Home, ".config", "omarchy", "backgrounds", "peace")
	if os.MkdirAll(backgroundDir, 0o700) != nil {
		return
	}
	background := filepath.Join(backgroundDir, fmt.Sprintf("follow-%d.png", r.Now().UnixMilli()))
	data, err := os.ReadFile(image)
	if err != nil {
		return
	}
	if os.WriteFile(background, data, 0o600) != nil {
		return
	}
	cmd := exec.Command("omarchy", "theme", "bg", "set", background)
	if cmd.Run() != nil {
		_ = os.Remove(background)
		r.logEvent("wallpaper switch failed " + ref)
		return
	}
	_ = os.WriteFile(filepath.Join(r.Config.StateDir, "displayed-ayah"), []byte(fmt.Sprintf("%d-%d", surah, ayah)), 0o600)
	r.logEvent("wallpaper switched " + ref)
	entries, _ := filepath.Glob(filepath.Join(backgroundDir, "follow-*.png"))
	if len(entries) > 2 {
		for _, path := range entries[:len(entries)-2] {
			_ = os.Remove(path)
		}
	}
}

func (r Runtime) finishFollow() {
	entries, _ := os.ReadDir(r.Config.StateDir)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "prep-") || !strings.HasSuffix(entry.Name(), ".pid") {
			continue
		}
		path := filepath.Join(r.Config.StateDir, entry.Name())
		if pid, err := readPID(path); err == nil {
			_ = syscall.Kill(-pid, syscall.SIGTERM)
		}
		_ = os.Remove(path)
	}
	_ = os.Remove(r.followingFile())
	_ = os.Remove(r.supervisorFile())
	_ = os.Remove(r.pidFile())
	_ = os.Remove(r.playFile())
	_ = os.RemoveAll(filepath.Join(r.Config.CacheDir, "follow"))
}

func (r Runtime) logEvent(message string) {
	file, err := os.OpenFile(filepath.Join(r.Config.StateDir, "follow.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "[%s] %s\n", r.Now().Format(time.RFC3339), message)
}

func titleAyah(title string, surah int) (int, bool) {
	var parsedSurah, ayah int
	if _, err := fmt.Sscanf(title, "%d:%d", &parsedSurah, &ayah); err != nil || parsedSurah != surah || ayah < 1 {
		return 0, false
	}
	return ayah, true
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
