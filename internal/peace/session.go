package peace

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var ipcSequence atomic.Uint64

func (r Runtime) startMPV(args []string) error {
	if err := os.MkdirAll(r.Config.StateDir, 0o700); err != nil {
		return err
	}
	_ = os.Remove(r.Config.Socket)
	logPath := filepath.Join(r.Config.StateDir, "mpv.log")
	cmdArgs := append(append([]string(nil), args...), "--log-file="+logPath, "--msg-level=all=warn")
	cmd := exec.Command("mpv", cmdArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	devnull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer devnull.Close()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, logFile, logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("peace listen: start mpv: %w", err)
	}
	if err := os.WriteFile(r.pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o600); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !r.pidAlive(r.pidFile()) {
			data, _ := os.ReadFile(filepath.Join(r.Config.StateDir, "mpv.log"))
			detail := strings.TrimSpace(string(data))
			if detail == "" {
				detail = "mpv stopped before becoming ready; inspect ~/.local/state/peace/mpv.log"
			}
			return fmt.Errorf("peace listen: mpv exited during startup: %s", detail)
		}
		if !containsArgument(args, "--idle=yes") && r.mpvAlive() {
			return nil
		}
		if r.mpvAlive() {
			if _, err := r.property("idle-active"); err == nil {
				return nil
			}
		}
		if _, err := os.Stat(r.Config.Socket); err == nil {
			if !r.mpvAlive() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	data, _ := os.ReadFile(filepath.Join(r.Config.StateDir, "mpv.log"))
	detail := strings.TrimSpace(string(data))
	if detail == "" {
		detail = "IPC socket did not become ready; inspect ~/.local/state/peace/mpv.log"
	}
	return fmt.Errorf("peace listen: mpv did not become ready: %s", detail)
}

func containsArgument(args []string, expected string) bool {
	for _, arg := range args {
		if arg == expected {
			return true
		}
	}
	return false
}

func (r Runtime) appendAudio(source, title string) error {
	response, err := r.sendIPC(map[string]any{"command": []any{"loadfile", source, "append-play", -1, map[string]string{"force-media-title": title}}})
	if err != nil {
		return err
	}
	if response["error"] != "success" {
		return fmt.Errorf("mpv rejected audio: %v", response["error"])
	}
	return nil
}

func (r Runtime) startWorker(action string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(r.Config.StateDir, "follow.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd := exec.Command(executable, action)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devnull.Close()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, logFile, logFile
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(r.supervisorFile(), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o600); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		return err
	}
	return cmd.Process.Release()
}

func (r Runtime) pause() error {
	if !r.mpvAlive() {
		return errors.New("not playing")
	}
	_, err := r.sendIPC(map[string]any{"command": []any{"cycle", "pause"}})
	if err != nil {
		return errors.New("not playing")
	}
	return nil
}

func (r Runtime) status() error {
	state, stateErr := r.readState()
	if r.mpvAlive() && stateErr == nil && state.Preparing {
		fmt.Printf("preparing (%d/%d ready)\n", state.Ready, state.Lead)
		return nil
	}
	if !r.mpvAlive() {
		fmt.Println("not playing")
		return nil
	}
	if stateErr != nil {
		fmt.Println("playing")
		return nil
	}
	position, err := r.property("playlist-pos")
	if err != nil {
		fmt.Println("playing")
		return nil
	}
	pos, ok := position.(float64)
	if !ok {
		fmt.Println("playing")
		return nil
	}
	ayah := state.Start + int(pos)
	if ayah > state.End {
		ayah = state.End
	}
	line := fmt.Sprintf("playing %d:%d (reciter %s", state.Surah, ayah, state.Reciter)
	if state.Follow && r.pidAlive(r.supervisorFile()) {
		line += ", following"
	}
	fmt.Println(line + ")")
	return nil
}

func (r Runtime) stop() error {
	for _, file := range []string{r.pidFile(), r.supervisorFile()} {
		pid, err := readPID(file)
		if err == nil {
			_ = syscall.Kill(-pid, syscall.SIGTERM)
		}
	}
	for _, path := range []string{r.pidFile(), r.supervisorFile(), r.playFile(), r.followingFile(), filepath.Join(r.Config.StateDir, "displayed-ayah")} {
		_ = os.Remove(path)
	}
	_ = os.Remove(r.Config.Socket)
	_ = filepath.WalkDir(r.Config.StateDir, func(path string, entry os.DirEntry, err error) error {
		if err == nil && strings.HasPrefix(entry.Name(), "prep-") && strings.HasSuffix(entry.Name(), ".pid") {
			if pid, readErr := readPID(path); readErr == nil {
				_ = syscall.Kill(-pid, syscall.SIGTERM)
			}
			_ = os.Remove(path)
		}
		return nil
	})
	return os.RemoveAll(filepath.Join(r.Config.CacheDir, "follow"))
}

func (r Runtime) mpvAlive() bool {
	pid, err := readPID(r.pidFile())
	if err != nil || syscall.Kill(pid, 0) != nil {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	return err == nil && strings.Contains(strings.ReplaceAll(string(data), "\x00", " "), "--input-ipc-server="+r.Config.Socket)
}

func (r Runtime) pidAlive(path string) bool {
	pid, err := readPID(path)
	return err == nil && processAlive(pid)
}

func processAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true
	}
	fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:]))
	return len(fields) == 0 || fields[0] != "Z" && fields[0] != "X"
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return 0, errors.New("invalid pid")
	}
	return pid, nil
}

func (r Runtime) sendIPC(command any) (map[string]any, error) {
	connection, err := net.DialTimeout("unix", r.Config.Socket, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	request, ok := command.(map[string]any)
	if !ok {
		return nil, errors.New("invalid mpv IPC command")
	}
	requestID := ipcSequence.Add(1)
	request["request_id"] = requestID
	if err := json.NewEncoder(connection).Encode(request); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(connection)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var response map[string]any
		if json.Unmarshal(line, &response) != nil {
			continue
		}
		if id, ok := response["request_id"].(float64); ok && uint64(id) == requestID {
			return response, nil
		}
	}
}

func (r Runtime) property(name string) (any, error) {
	response, err := r.sendIPC(map[string]any{"command": []any{"get_property", name}})
	if err != nil {
		return nil, err
	}
	if response["error"] != "success" {
		return nil, errors.New("mpv property unavailable")
	}
	return response["data"], nil
}

func (r Runtime) readState() (PlayState, error) {
	var state PlayState
	data, err := os.ReadFile(r.playFile())
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(data, &state)
	return state, err
}

func (r Runtime) writeState(state PlayState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return writeAtomic(r.playFile(), data, 0o600)
}

func (r Runtime) pidFile() string        { return filepath.Join(r.Config.StateDir, "mpv.pid") }
func (r Runtime) playFile() string       { return filepath.Join(r.Config.StateDir, "play.json") }
func (r Runtime) supervisorFile() string { return filepath.Join(r.Config.StateDir, "follow.pid") }
func (r Runtime) followingFile() string  { return filepath.Join(r.Config.StateDir, "following") }
