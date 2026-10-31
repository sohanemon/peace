package peace

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ayahEdition struct {
	Text  string `json:"text"`
	Surah struct {
		EnglishName string `json:"englishName"`
		Number      int    `json:"number"`
	} `json:"surah"`
	NumberInSurah int `json:"numberInSurah"`
}

type ayahText struct {
	Arabic    string
	English   string
	Reference string
}

type themeColors struct {
	Accent     string
	Foreground string
	Muted      string
	Background string
}

var colorPattern = regexp.MustCompile(`(?m)^([a-z_]+)\s*=\s*"(#[0-9A-Fa-f]{6})"`)

func (r Runtime) renderAyah(surah, ayah int, baseImage, output string) error {
	text, err := fetchAyahText(surah, ayah)
	if err != nil {
		return err
	}
	return r.renderAyahText(surah, ayah, text, baseImage, output)
}

func (r Runtime) renderAyahText(surah, ayah int, text ayahText, baseImage, output string) error {
	magick, err := exec.LookPath("magick")
	if err != nil {
		return fmt.Errorf("wallpaper rendering requires ImageMagick: %w", err)
	}
	colors := r.currentColors()
	width, height := r.screenSize()
	textWidth := width * 78 / 100
	maxHeight := height * 76 / 100
	fontScale := min(width, height*16/9) * 100 / 2560
	fontScale = min(150, max(60, fontScale))
	arabicSize := 76 * fontScale / 100
	englishSize := 38 * fontScale / 100
	referenceSize := 18 * fontScale / 100
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(output), ".omapeace-render-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	texts := []string{text.Arabic, trimEdgeQuote(text.English), text.Reference}
	fonts := []*int{&arabicSize, &englishSize, &referenceSize}
	minimums := []int{24, 18, 12}
	files := []string{filepath.Join(tempDir, "arabic.png"), filepath.Join(tempDir, "english.png"), filepath.Join(tempDir, "reference.png")}
	colorsByText := []string{colors.Accent, colors.Foreground, colors.Muted}
	families := []string{"Amiri", "DejaVu Serif", "DejaVu Sans"}
	widths, heights := make([]int, 3), make([]int, 3)
	gap := max(18, height*5/100)
	for attempt := 0; attempt < 18; attempt++ {
		for i := range texts {
			markup := fmt.Sprintf("<span font_desc='%s %d' foreground='%s'>%s</span>", families[i], *fonts[i], colorsByText[i], html.EscapeString(texts[i]))
			if err := runMagick(magick, "-size", strconv.Itoa(textWidth)+"x", "-define", "pango:align=center", "-background", "none", "pango:"+markup, files[i]); err != nil {
				return err
			}
			widths[i], heights[i], err = imageSize(magick, files[i])
			if err != nil {
				return err
			}
		}
		stackHeight := heights[0] + heights[1] + heights[2] + gap*2
		if stackHeight <= maxHeight && widths[0] <= textWidth && widths[1] <= textWidth && widths[2] <= textWidth {
			break
		}
		for i := range fonts {
			*fonts[i] = max(minimums[i], *fonts[i]*9/10)
		}
		if attempt == 17 {
			return fmt.Errorf("ayah %d:%d does not fit the display", surah, ayah)
		}
	}
	stackHeight := heights[0] + heights[1] + heights[2] + gap*2
	stack := filepath.Join(tempDir, "stack.png")
	stackArgs := []string{"-size", fmt.Sprintf("%dx%d", textWidth, stackHeight), "xc:none"}
	y := 0
	for i, path := range files {
		stackArgs = append(stackArgs, path, "-gravity", "north", "-geometry", fmt.Sprintf("+0+%d", y), "-compose", "over", "-composite")
		y += heights[i] + gap
	}
	stackArgs = append(stackArgs, stack)
	if err := runMagick(magick, stackArgs...); err != nil {
		return err
	}
	backdrop := filepath.Join(tempDir, "backdrop.png")
	if baseImage != "" {
		if err := runMagick(magick, baseImage, "-auto-orient", "-resize", fmt.Sprintf("%dx%d^", width, height), "-gravity", "center", "-extent", fmt.Sprintf("%dx%d", width, height), backdrop); err != nil {
			return err
		}
	} else if err := runMagick(magick, "-size", fmt.Sprintf("%dx%d", width, height), "xc:"+colors.Background, backdrop); err != nil {
		return err
	}
	shadow := filepath.Join(tempDir, "shadow.png")
	if err := runMagick(magick, stack, "-background", "none", "-shadow", "80x10+0+0", shadow); err != nil {
		return err
	}
	composed := filepath.Join(tempDir, "composed.png")
	if err := runMagick(magick, backdrop, shadow, "-gravity", "center", "-compose", "over", "-composite", composed); err != nil {
		return err
	}
	if err := runMagick(magick, composed, stack, "-gravity", "center", "-compose", "over", "-composite", output); err != nil {
		return err
	}
	return nil
}

func fetchAyahText(surah, ayah int) (ayahText, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://api.alquran.cloud/v1/ayah/%d:%d/editions/quran-uthmani,en.sahih", surah, ayah)
	data, err := fetchAyahEditions(ctx, url)
	if err != nil {
		return ayahText{}, err
	}
	return ayahText{
		Arabic:    data[0].Text,
		English:   data[1].Text,
		Reference: fmt.Sprintf("%s %d:%d", data[0].Surah.EnglishName, surah, data[0].NumberInSurah),
	}, nil
}

// fetchAyahEditions fetches and validates the two ayah editions (Arabic + English).
func fetchAyahEditions(ctx context.Context, url string) ([]ayahEdition, error) {
	var response struct {
		Data []ayahEdition `json:"data"`
	}
	if err := getJSON(ctx, url, &response); err != nil {
		return nil, err
	}
	if len(response.Data) != 2 {
		return nil, fmt.Errorf("ayah text is incomplete")
	}
	return response.Data, nil
}

func (r Runtime) currentColors() themeColors {
	colors := themeColors{Accent: "#d6b858", Foreground: "#f2eee0", Muted: "#c3bfae", Background: "#101b18"}
	paths := []string{filepath.Join(r.Config.Home, ".local", "state", "omarchy", "current", "theme", "colors.toml")}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, match := range colorPattern.FindAllStringSubmatch(string(data), -1) {
			switch match[1] {
			case "accent":
				colors.Accent = match[2]
			case "foreground":
				colors.Foreground = match[2]
			case "muted":
				colors.Muted = match[2]
			case "background":
				colors.Background = match[2]
			}
		}
		return colors
	}
	return colors
}

func (r Runtime) screenSize() (int, int) {
	cmd := exec.Command("hyprctl", "monitors", "-j")
	data, err := cmd.Output()
	if err == nil {
		var monitors []struct {
			Focused bool `json:"focused"`
			Width   int  `json:"width"`
			Height  int  `json:"height"`
		}
		if json.Unmarshal(data, &monitors) == nil {
			for _, monitor := range monitors {
				if monitor.Focused && monitor.Width > 0 && monitor.Height > 0 {
					return monitor.Width, monitor.Height
				}
			}
		}
	}
	return 1920, 1080
}

func imageSize(magick, path string) (int, int, error) {
	output, err := exec.Command(magick, "identify", "-format", "%w %h", path).Output()
	if err != nil {
		return 0, 0, err
	}
	var width, height int
	if _, err := fmt.Sscanf(string(output), "%d %d", &width, &height); err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

func runMagick(magick string, args ...string) error {
	cmd := exec.Command(magick, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("magick: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func trimEdgeQuote(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	quotes := "\"“”"
	count := 0
	for _, value := range runes {
		if strings.ContainsRune(quotes, value) {
			count++
		}
	}
	if count%2 == 1 {
		if len(runes) > 0 && strings.ContainsRune(quotes, runes[0]) {
			runes = runes[1:]
		} else if len(runes) > 0 && strings.ContainsRune(quotes, runes[len(runes)-1]) {
			runes = runes[:len(runes)-1]
		}
	}
	return strings.TrimSpace(string(runes))
}
