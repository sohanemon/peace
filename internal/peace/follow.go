package peace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (r Runtime) snapshotBackground(random bool) (string, error) {
	current := filepath.Join(r.Config.Home, ".local", "state", "omarchy", "current", "background")
	source := ""
	if random {
		source = r.fallbackBackground(true)
	}
	if source == "" {
		var err error
		source, err = filepath.EvalSymlinks(current)
		if err != nil {
			source = r.fallbackBackground(false)
		}
	}
	outputDir := filepath.Join(r.Config.Home, ".local", "share", "omapeace", "backgrounds")
	if source != "" {
		if relative, relErr := filepath.Rel(outputDir, source); relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			basePath, readErr := os.ReadFile(source + ".base")
			if readErr != nil {
				source = r.fallbackBackground(false)
			} else if resolved, resolveErr := filepath.EvalSymlinks(strings.TrimSpace(string(basePath))); resolveErr != nil {
				source = r.fallbackBackground(false)
			} else {
				source = resolved
			}
		}
	}
	if isTemporaryBackground(source) {
		basePath, readErr := os.ReadFile(source + ".base")
		if readErr == nil {
			if resolved, resolveErr := filepath.EvalSymlinks(strings.TrimSpace(string(basePath))); resolveErr == nil {
				source = resolved
			}
		}
	}
	info, err := os.Stat(source)
	if err != nil || !info.Mode().IsRegular() || !supportedBackground(source) || isTemporaryBackground(source) {
		source = r.fallbackBackground(false)
		info, err = os.Stat(source)
		if err != nil || !info.Mode().IsRegular() || isTemporaryBackground(source) {
			return "", nil
		}
	}
	if err := os.MkdirAll(filepath.Join(r.Config.CacheDir, "follow"), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(r.Config.CacheDir, "follow", "base-source"), []byte(source+"\n"), 0o600); err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(source))
	if len(ext) > 8 || ext == "" {
		ext = ".img"
	}
	destination := filepath.Join(r.Config.CacheDir, "follow", "base"+ext)
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = input.Close()
	}()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(destination)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(destination)
		return "", closeErr
	}
	return destination, nil
}

func (r Runtime) fallbackBackground(random bool) string {
	candidates := r.backgroundCandidates()
	if len(candidates) == 0 {
		return ""
	}
	if random {
		return candidates[rand.Intn(len(candidates))]
	}
	return candidates[0]
}

func (r Runtime) backgroundCandidates() []string {
	var candidates []string
	seen := make(map[string]bool)
	for _, directory := range r.backgroundDirectories() {
		paths, _ := filepath.Glob(filepath.Join(directory, "*"))
		for _, path := range paths {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				continue
			}
			if seen[resolved] || isTemporaryBackground(path) || !supportedBackground(path) {
				continue
			}
			if info, err := os.Stat(resolved); err == nil && info.Mode().IsRegular() && supportedBackground(resolved) {
				candidates = append(candidates, resolved)
				seen[resolved] = true
			}
		}
	}
	return candidates
}

func backgroundForAyah(sources []string, start, ayah int) string {
	if len(sources) == 0 {
		return ""
	}
	index := (ayah - start) % len(sources)
	return sources[index]
}

func (r Runtime) backgroundDirectories() []string {
	return []string{
		filepath.Join(r.Config.Home, ".local", "state", "omarchy", "current", "theme", "backgrounds"),
		filepath.Join(r.Config.Home, "Code", "peace", "backgrounds"),
		filepath.Join(r.Config.PluginDir, "assets", "backgrounds"),
	}
}

func isTemporaryBackground(path string) bool {
	isTemporaryName := func(candidate string) bool {
		name := strings.ToLower(filepath.Base(candidate))
		return strings.HasPrefix(name, "temp_") || strings.HasPrefix(name, "tmp_")
	}
	if isTemporaryName(path) {
		return true
	}
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && isTemporaryName(resolved)
}

func followArtifact(cache string, surah, ayah int, extension string) string {
	return filepath.Join(cache, fmt.Sprintf("%d-%d%s", surah, ayah, extension))
}

type followWindow struct {
	appendLimit int
	audioStart  int
	audioEnd    int
	imageStart  int
	imageEnd    int
}

func planFollowWindow(state PlayState, current, appended int, paused bool) followWindow {
	total := state.End - state.Start + 1
	appendLimit := max(0, current-state.Start+1) + state.Buffer
	appendLimit = min(total, appendLimit)
	window := max(state.Buffer, state.Lead)
	if paused && current == state.Start {
		window = max(0, window-appended)
	}
	audioStart := max(state.Start, current+1)
	imageStart := max(state.Start, current)
	return followWindow{
		appendLimit: appendLimit,
		audioStart:  audioStart,
		audioEnd:    min(state.End, audioStart+window-1),
		imageStart:  imageStart,
		imageEnd:    min(state.End, imageStart+window-1),
	}
}

func supportedBackground(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".bmp", ".gif", ".jpeg", ".jpg", ".png", ".tif", ".tiff", ".webp":
		return true
	default:
		return false
	}
}

func (r Runtime) prepare(args []string) error {
	if len(args) != 2 {
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
	state, err := r.readState()
	if err != nil {
		return err
	}
	cache := filepath.Join(r.Config.CacheDir, "follow")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return err
	}
	image := followArtifact(cache, surah, ayah, ".png")
	var renderErr error
	if fileExists(image) || fileExists(followArtifact(cache, surah, ayah, ".skip")) {
		renderErr = nil
	} else {
		for attempt := 1; attempt <= 2; attempt++ {
			renderBase := state.BaseImage
			if state.Random {
				if source := backgroundForAyah(state.RandomSources, state.Start, ayah); source != "" {
					renderBase = source
				}
			}
			renderErr = r.renderFollowWallpaper(surah, ayah, renderBase, state.Basmala, state.UseTheme)
			if renderErr == nil {
				break
			}
		}
	}
	if renderErr != nil {
		_ = os.WriteFile(followArtifact(cache, surah, ayah, ".skip"), nil, 0o600)
		r.logEvent(fmt.Sprintf("wallpaper render skipped %s: %v", ref, renderErr))
	} else {
		r.logEvent(fmt.Sprintf("wallpaper cached %s", ref))
	}
	return nil
}

func (r Runtime) prepareInitialWallpaper(surah, ayah int, baseImage string, basmala, useTheme bool) error {
	cache := filepath.Join(r.Config.CacheDir, "follow")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return err
	}
	return r.renderFollowWallpaper(surah, ayah, baseImage, basmala, useTheme)
}

func (r Runtime) renderFollowWallpaper(surah, ayah int, baseImage string, basmala, useTheme bool) error {
	cache := filepath.Join(r.Config.CacheDir, "follow")
	image := followArtifact(cache, surah, ayah, ".png")
	temp := image + ".tmp.png"
	if err := r.renderAyah(surah, ayah, baseImage, temp, basmala, useTheme); err != nil {
		_ = os.Remove(temp)
		return err
	}
	if err := os.Rename(temp, image); err != nil {
		_ = os.Remove(temp)
		return err
	}
	if err := writeAtomic(followArtifact(cache, surah, ayah, ".base"), []byte(baseImage+"\n"), 0o600); err != nil {
		_ = os.Remove(image)
		return err
	}
	return nil
}

func downloadFile(url, destination string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	response, err := openURL(ctx, url)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
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
	prep := map[int]int{}
	audioJobs := map[int]bool{}
	audioDone := make(chan int, 32)
	appendIndex, current := state.Appended, state.Start
	lastTitle := ""
	waitAyah := 0
	waitStarted := time.Time{}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for r.mpvAlive() {
		<-ticker.C
		for {
			select {
			case ayah := <-audioDone:
				delete(audioJobs, ayah)
			default:
				goto audioPrefetchComplete
			}
		}
	audioPrefetchComplete:
		now := time.Now()
		properties, _ := r.properties("idle-active", "playlist-pos", "playlist-count", "media-title", "pause")
		idle := properties["idle-active"]
		position := properties["playlist-pos"]
		count := properties["playlist-count"]
		title := properties["media-title"]
		paused := properties["pause"]
		idleActive, _ := idle.(bool)
		positionValue, hasPosition := position.(float64)
		playlistCount, hasCount := count.(float64)
		mediaTitle, _ := title.(string)
		if !idleActive {
			ayah, ok := followTrackAyah(mediaTitle, int(positionValue), hasPosition, state)
			if ok && mediaTitle == "" {
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
		pausedValue, _ := paused.(bool)
		window := planFollowWindow(state, current, appendIndex, pausedValue)
		if appendIndex < window.appendLimit {
			ayah := state.Start + appendIndex
			mp3 := followArtifact(cache, state.Surah, ayah, ".mp3")
			imageResolved := fileExists(followArtifact(cache, state.Surah, ayah, ".png")) || fileExists(followArtifact(cache, state.Surah, ayah, ".skip"))
			audioResolved := fileExists(mp3) || fileExists(followArtifact(cache, state.Surah, ayah, ".audio-fail"))
			if waitAyah != ayah {
				waitAyah = ayah
				waitStarted = now
			}
			if imageResolved && (audioResolved || now.Sub(waitStarted) >= 2*time.Second) {
				audioSource := mp3
				if !fileExists(mp3) && appendIndex < len(state.AudioURLs) {
					audioSource = state.AudioURLs[appendIndex]
				}
				if err := r.appendAudio(audioSource, fmt.Sprintf("%d:%d", state.Surah, ayah)); err == nil {
					appendIndex++
					state.Appended = appendIndex
					if err := r.writeState(state); err != nil {
						r.logEvent(fmt.Sprintf("state update failed after append: %v", err))
					}
					waitAyah = 0
					r.logEvent(fmt.Sprintf("audio appended %d:%d (%s)", state.Surah, ayah, audioSource))
				} else {
					r.logEvent(fmt.Sprintf("audio append failed %d:%d: %v", state.Surah, ayah, err))
				}
			}
		}
		active := 0
		for ayah, pid := range prep {
			if processAlive(pid) {
				active++
			} else {
				delete(prep, ayah)
				_ = os.Remove(filepath.Join(r.Config.StateDir, fmt.Sprintf("prep-%d.pid", ayah)))
			}
		}
		limit := state.Jobs
		activeAudio := len(audioJobs)
		for ayah := window.audioStart; ayah <= window.audioEnd && activeAudio < state.Jobs; ayah++ {
			if fileExists(followArtifact(cache, state.Surah, ayah, ".mp3")) || fileExists(followArtifact(cache, state.Surah, ayah, ".audio-fail")) || audioJobs[ayah] {
				continue
			}
			if ayah-state.Start >= len(state.AudioURLs) {
				continue
			}
			audioJobs[ayah] = true
			activeAudio++
			go r.prefetchAudio(state.Surah, ayah, state.AudioURLs[ayah-state.Start], audioDone)
		}
		for ayah := window.imageStart; ayah <= window.imageEnd && active < limit; ayah++ {
			if _, exists := prep[ayah]; exists {
				continue
			}
			imageDone := fileExists(followArtifact(cache, state.Surah, ayah, ".png")) || fileExists(followArtifact(cache, state.Surah, ayah, ".skip"))
			if imageDone {
				continue
			}
			pid, startErr := r.startPrepare(state.Surah, ayah)
			if startErr == nil {
				prep[ayah] = pid
				active++
			}
		}
		playlistExtra := 0
		if state.IntroAudioURL != "" {
			playlistExtra = 1
		}
		if appendIndex >= total && idleActive && hasCount && int(playlistCount) >= total+playlistExtra {
			_, _ = r.sendIPC(map[string]any{"command": []any{"quit"}, "request_id": 2})
			return nil
		}
	}
	return nil
}

func (r Runtime) prefetchAudio(surah, ayah int, url string, done chan<- int) {
	defer func() { done <- ayah }()
	cache := filepath.Join(r.Config.CacheDir, "follow")
	audio := followArtifact(cache, surah, ayah, ".mp3")
	marker := followArtifact(cache, surah, ayah, ".audio-fail")
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		err = downloadFile(url, audio+".tmp")
		if err == nil {
			err = os.Rename(audio+".tmp", audio)
		}
		if err == nil {
			r.logEvent(fmt.Sprintf("audio cached %d:%d", surah, ayah))
			return
		}
		_ = os.Remove(audio + ".tmp")
	}
	_ = os.WriteFile(marker, nil, 0o600)
	r.logEvent(fmt.Sprintf("audio cache unavailable %d:%d: %v; playback will stream", surah, ayah, err))
}

func (r Runtime) startPrepare(surah, ayah int) (int, error) {
	executable, err := os.Executable()
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(executable, "__prepare", strconv.Itoa(surah), strconv.Itoa(ayah))
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

func (r Runtime) displayWallpaper(cache string, surah, ayah int) bool {
	ref := fmt.Sprintf("%d:%d", surah, ayah)
	shown, _ := os.ReadFile(filepath.Join(r.Config.StateDir, "displayed-ayah"))
	if strings.TrimSpace(string(shown)) == fmt.Sprintf("%d-%d", surah, ayah) {
		return true
	}
	image := followArtifact(cache, surah, ayah, ".png")
	if !fileExists(image) {
		return false
	}
	backgroundDir := filepath.Join(r.Config.Home, ".local", "share", "omapeace", "backgrounds")
	if os.MkdirAll(backgroundDir, 0o700) != nil {
		return false
	}
	background := filepath.Join(backgroundDir, fmt.Sprintf("follow-%d.png", time.Now().UnixNano()))
	data, err := os.ReadFile(image)
	if err != nil {
		return false
	}
	if os.WriteFile(background, data, 0o600) != nil {
		return false
	}
	baseFile := followArtifact(cache, surah, ayah, ".base")
	if baseSource, readErr := os.ReadFile(baseFile); readErr == nil {
		_ = os.WriteFile(background+".base", baseSource, 0o600)
	} else if baseSource, readErr := os.ReadFile(filepath.Join(cache, "base-source")); readErr == nil {
		_ = os.WriteFile(background+".base", baseSource, 0o600)
	}
	cmd := exec.Command("omarchy", "theme", "bg", "set", background)
	if cmd.Run() != nil {
		_ = os.Remove(background)
		r.logEvent("wallpaper switch failed " + ref)
		return false
	}
	_ = os.WriteFile(filepath.Join(r.Config.StateDir, "displayed-ayah"), []byte(fmt.Sprintf("%d-%d", surah, ayah)), 0o600)
	r.logEvent("wallpaper switched " + ref)
	entries, _ := filepath.Glob(filepath.Join(backgroundDir, "follow-*.png"))
	if len(entries) > 2 {
		for _, path := range entries[:len(entries)-2] {
			_ = os.Remove(path)
			_ = os.Remove(path + ".base")
		}
	}
	return true
}

func (r Runtime) finishFollow() {
	stopPrep(r.Config.StateDir)
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
	defer func() {
		_ = file.Close()
	}()
	_, _ = fmt.Fprintf(file, "[%s] %s\n", time.Now().Format(time.RFC3339), message)
}

func titleAyah(title string, surah int) (int, bool) {
	var parsedSurah, ayah int
	if _, err := fmt.Sscanf(title, "%d:%d", &parsedSurah, &ayah); err != nil || parsedSurah != surah || ayah < 1 {
		return 0, false
	}
	return ayah, true
}

func followTrackAyah(title string, position int, hasPosition bool, state PlayState) (int, bool) {
	if state.IntroAudioURL != "" && title == "Introductory Basmala" {
		return 0, false
	}
	if ayah, ok := titleAyah(title, state.Surah); ok {
		return ayah, true
	}
	if !hasPosition {
		return 0, false
	}
	if state.IntroAudioURL != "" {
		position--
	}
	if position < 0 {
		return 0, false
	}
	return state.Start + position, true
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
