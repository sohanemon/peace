package peace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	// image/gif registers the GIF image format with image.Decode.
	_ "image/gif"
	// image/jpeg registers the JPEG image format with image.Decode.
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	// golang.org/x/image/bmp registers the BMP image format with image.Decode.
	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"
	// golang.org/x/image/tiff registers the TIFF image format with image.Decode.
	_ "golang.org/x/image/tiff"
	"golang.org/x/image/vector"
	"golang.org/x/image/webp"

	"io.github.sohanemon/peace/assets"
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

type textStyle struct {
	Text           string
	Size           int
	Minimum        int
	Face           *font.Face
	Color          color.RGBA
	TargetContrast float64
	Outline        bool
	Direction      di.Direction
	Script         language.Script
}

type renderedText struct {
	Image  *image.RGBA
	Height int
	Width  int
}

var colorPattern = regexp.MustCompile(`(?m)^([a-z_]+)\s*=\s*"(#[0-9A-Fa-f]{6})"`)
var linearLuminance = func() [256]float64 {
	var values [256]float64
	for channel := range values {
		value := float64(channel) / 255
		if value <= 0.04045 {
			values[channel] = value / 12.92
		} else {
			values[channel] = math.Pow((value+0.055)/1.055, 2.4)
		}
	}
	return values
}()
var fontFaces struct {
	sync.Once
	Arabic, Serif, Sans *font.Face
	err                 error
}

func (r Runtime) renderAyah(surah, ayah int, baseImage, output string, showBasmala bool, renderOptions RenderOptions) error {
	text, err := fetchAyahText(surah, ayah)
	if err != nil {
		return err
	}
	text.Arabic = displayAyahArabic(surah, ayah, text.Arabic, showBasmala)
	return r.renderAyahText(surah, ayah, text, baseImage, output, renderOptions)
}

func displayAyahArabic(surah, ayah int, text string, showBasmala bool) string {
	if !showBasmala && ayah == 1 && hasIntroductoryBasmala(surah) {
		return removeIntroductoryBasmala(text)
	}
	return text
}

func removeIntroductoryBasmala(text string) string {
	phrase := []rune("بسماللهالرحمنالرحيم")
	runes := []rune(strings.TrimSpace(text))
	letterPositions := make([]int, 0, len(phrase))
	letters := make([]rune, 0, len(phrase))
	for position, current := range runes {
		if unicode.IsLetter(current) {
			letters = append(letters, normalizeArabicLetter(current))
			letterPositions = append(letterPositions, position)
		}
	}
	if len(letters) < len(phrase) || string(letters[:len(phrase)]) != string(phrase) {
		return text
	}
	position := letterPositions[len(phrase)-1] + 1
	for position < len(runes) && (unicode.Is(unicode.Mn, runes[position]) || unicode.IsSpace(runes[position])) {
		position++
	}
	if position == len(runes) {
		return ""
	}
	return string(runes[position:])
}

func normalizeArabicLetter(letter rune) rune {
	switch letter {
	case 'أ', 'إ', 'آ', 'ٱ':
		return 'ا'
	case 'ى':
		return 'ي'
	default:
		return letter
	}
}

func (r Runtime) renderAyahText(surah, ayah int, text ayahText, baseImage, output string, renderOptions RenderOptions) error {
	if err := loadRenderFonts(); err != nil {
		return err
	}
	colors := r.currentColors()
	width, height := r.screenSize()
	backdrop, err := loadBackdrop(baseImage, width, height, colors.Background)
	if err != nil {
		return fmt.Errorf("load wallpaper background: %w", err)
	}
	palette := themeTextPalette(colors)
	if !renderOptions.UseTheme {
		palette = wallpaperTextPalette(backdrop, parseColor(colors.Accent))
	}
	textWidth := width * 78 / 100
	maxHeight := height * 76 / 100
	fontScale := min(width, height*16/9) * 100 / 2560
	fontScale = min(150, max(60, fontScale))
	styles := []textStyle{{
		Text: text.Arabic, Size: 76 * fontScale / 100, Minimum: 24, Face: fontFaces.Arabic,
		Color: palette.Arabic, TargetContrast: 3, Outline: true, Direction: di.DirectionRTL, Script: language.Arabic,
	}}
	if renderOptions.ShowTranslation {
		styles = append(styles, textStyle{
			Text: trimEdgeQuote(text.English), Size: 32 * fontScale / 100, Minimum: 16, Face: fontFaces.Serif,
			Color: palette.Translation, TargetContrast: 4.5, Outline: true, Direction: di.DirectionLTR, Script: language.Latin,
		})
	}
	if renderOptions.ShowReference {
		styles = append(styles, textStyle{
			Text: text.Reference, Size: 18 * fontScale / 100, Minimum: 12, Face: fontFaces.Sans,
			Color: palette.Reference, TargetContrast: 3, Direction: di.DirectionLTR, Script: language.Latin,
		})
	}
	gap := max(18, height*5/100)
	rendered := make([]renderedText, len(styles))
	stackHeight := 0
	for attempt := 0; attempt < 18; attempt++ {
		stackHeight = gap * 2
		fits := true
		for i := range styles {
			var err error
			rendered[i], err = renderTextBlock(styles[i], textWidth)
			if err != nil {
				return fmt.Errorf("render ayah %d:%d text: %w", surah, ayah, err)
			}
			stackHeight += rendered[i].Height
			fits = fits && rendered[i].Width <= textWidth
		}
		if fits && stackHeight <= maxHeight {
			break
		}
		for i := range styles {
			styles[i].Size = max(styles[i].Minimum, styles[i].Size*9/10)
		}
		if attempt == 17 {
			return fmt.Errorf("ayah %d:%d does not fit the display", surah, ayah)
		}
	}
	stackHeight = gap * 2
	for _, block := range rendered {
		stackHeight += block.Height
	}
	if !renderOptions.UseTheme {
		textY := 0
		for i := range rendered {
			left := (width - rendered[i].Width) / 2
			top := (height-stackHeight)/2 + textY
			styles[i].Color = adjustTextColor(backdrop, rendered[i].Image, left, top, styles[i].Color, styles[i].TargetContrast)
			recolorText(rendered[i].Image, styles[i].Color)
			textY += rendered[i].Height + gap
		}
	}
	stack := image.NewRGBA(image.Rect(0, 0, textWidth, stackHeight))
	y := 0
	for i := range rendered {
		draw.Draw(stack, image.Rect(0, y, textWidth, y+rendered[i].Height), rendered[i].Image, image.Point{}, draw.Over)
		y += rendered[i].Height + gap
	}
	outlineColors := make([]color.NRGBA, len(rendered))
	outlineNeeded := make([]bool, len(rendered))
	textY := 0
	outlineRadius := max(1, min(2, min(width, height)/1200))
	for i := range rendered {
		if !styles[i].Outline {
			textY += rendered[i].Height + gap
			continue
		}
		left := (width - rendered[i].Width) / 2
		top := (height-stackHeight)/2 + textY
		outlineColors[i], outlineNeeded[i] = adaptiveOutlineColor(backdrop, rendered[i].Image, left, top)
		textY += rendered[i].Height + gap
	}
	shadowText(backdrop, stack, width/2, height/2)
	textY = 0
	for i := range rendered {
		if outlineNeeded[i] {
			left := (width - rendered[i].Width) / 2
			top := (height-stackHeight)/2 + textY
			outline := outlineText(rendered[i].Image, outlineColors[i], outlineRadius)
			outlineBounds := image.Rect(left, top, left+outline.Bounds().Dx(), top+outline.Bounds().Dy())
			draw.Draw(backdrop, outlineBounds, outline, image.Point{}, draw.Over)
		}
		textY += rendered[i].Height + gap
	}
	compositeCentered(backdrop, stack, width/2, height/2)
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	if err := png.Encode(file, backdrop); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func loadRenderFonts() error {
	fontFaces.Do(func() {
		fontFaces.Arabic, fontFaces.err = parseEmbeddedFont("fonts/Amiri-Regular.ttf")
		if fontFaces.err == nil {
			fontFaces.Serif, fontFaces.err = parseEmbeddedFont("fonts/DejaVuSerif.ttf")
		}
		if fontFaces.err == nil {
			fontFaces.Sans, fontFaces.err = parseEmbeddedFont("fonts/DejaVuSans.ttf")
		}
	})
	return fontFaces.err
}

func parseEmbeddedFont(path string) (*font.Face, error) {
	data, err := assets.FS.ReadFile(path)
	if err != nil {
		return nil, err
	}
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse bundled font %s: %w", path, err)
	}
	return face, nil
}

func renderTextBlock(style textStyle, maxWidth int) (renderedText, error) {
	text := strings.TrimSpace(style.Text)
	if text == "" {
		return renderedText{}, fmt.Errorf("text is empty")
	}
	pixelSize := style.Size * 4 / 3
	input := shaping.Input{Direction: style.Direction, Face: style.Face, Size: fixed.I(pixelSize), Script: style.Script}
	shaper := &shaping.HarfbuzzShaper{}
	shape := func(line string) shaping.Output {
		input.Text = []rune(line)
		input.RunEnd = len(input.Text)
		return shaper.Shape(input)
	}
	lines := wrapShapedText(text, maxWidth, shape)
	type textLine struct {
		Output  shaping.Output
		Ascent  int
		Descent int
		Height  int
	}
	lineLayouts := make([]textLine, len(lines))
	imageHeight := 0
	for row, line := range lines {
		out := shape(line)
		ascent, descent := out.LineBounds.Ascent.Round(), out.LineBounds.Descent.Round()
		for _, glyph := range out.Glyphs {
			glyphAscent := (glyph.YBearing + glyph.YOffset).Round()
			glyphDescent := (glyph.YBearing + glyph.Height + glyph.YOffset).Round()
			ascent = max(ascent, glyphAscent)
			descent = min(descent, glyphDescent)
		}
		lineHeight := max(pixelSize*6/5, ascent-descent) + 8
		lineLayouts[row] = textLine{Output: out, Ascent: ascent, Descent: descent, Height: lineHeight}
		imageHeight += lineHeight
	}
	imageHeight = max(1, imageHeight)
	mask := image.NewAlpha(image.Rect(0, 0, maxWidth, imageHeight))
	rowY := 0
	for _, line := range lineLayouts {
		out := line.Output
		lineWidth := abs(out.Advance.Round())
		if lineWidth > maxWidth {
			return renderedText{}, fmt.Errorf("line width %d exceeds available width %d", lineWidth, maxWidth)
		}
		penX := (maxWidth - lineWidth) / 2
		if out.Direction == di.DirectionRTL && out.Advance < 0 {
			penX += lineWidth
		}
		baseline := rowY + (line.Height-(line.Ascent-line.Descent))/2 + line.Ascent
		rasterizer := vector.NewRasterizer(maxWidth, imageHeight)
		for _, glyph := range out.Glyphs {
			outline, ok := out.Face.GlyphDataOutline(glyph.GlyphID)
			if !ok {
				penX += glyph.Advance.Round()
				continue
			}
			dotX := float32(penX) + float32(glyph.XOffset)/64
			dotY := float32(baseline) - float32(glyph.YOffset)/64
			scale := float32(out.Size) / 64 / float32(out.Face.Upem())
			appendGlyphOutline(rasterizer, outline, scale, dotX, dotY)
			penX += glyph.Advance.Round()
		}
		rasterizer.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: 0xff}), image.Point{})
		rowY += line.Height
	}
	canvas := image.NewRGBA(image.Rect(0, 0, maxWidth, imageHeight))
	draw.DrawMask(canvas, canvas.Bounds(), image.NewUniform(style.Color), image.Point{}, mask, image.Point{}, draw.Over)
	return renderedText{Image: canvas, Width: maxWidth, Height: imageHeight}, nil
}

func appendGlyphOutline(rasterizer *vector.Rasterizer, outline font.GlyphOutline, scale, x, baseline float32) {
	point := func(p opentype.SegmentPoint) (float32, float32) {
		return x + p.X*scale, baseline - p.Y*scale
	}
	for _, segment := range outline.Segments {
		args := segment.ArgsSlice()
		switch segment.Op {
		case opentype.SegmentOpMoveTo:
			rasterizer.ClosePath()
			px, py := point(args[0])
			rasterizer.MoveTo(px, py)
		case opentype.SegmentOpLineTo:
			px, py := point(args[0])
			rasterizer.LineTo(px, py)
		case opentype.SegmentOpQuadTo:
			cx, cy := point(args[0])
			px, py := point(args[1])
			rasterizer.QuadTo(cx, cy, px, py)
		case opentype.SegmentOpCubeTo:
			c1x, c1y := point(args[0])
			c2x, c2y := point(args[1])
			px, py := point(args[2])
			rasterizer.CubeTo(c1x, c1y, c2x, c2y, px, py)
		}
	}
	rasterizer.ClosePath()
}

func wrapShapedText(text string, maxWidth int, shape func(string) shaping.Output) []string {
	words := strings.Fields(text)
	lines := make([]string, 0, 4)
	line := ""
	for _, word := range words {
		candidate := strings.TrimSpace(line + " " + word)
		if line != "" && abs(shape(candidate).Advance.Round()) > maxWidth {
			lines = append(lines, line)
			line = word
		} else {
			line = candidate
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func loadBackdrop(path string, width, height int, fallback string) (*image.RGBA, error) {
	if path == "" {
		canvas := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(parseColor(fallback)), image.Point{}, draw.Src)
		return canvas, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()
	source, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	bounds := source.Bounds()
	scale := math.Max(float64(width)/float64(bounds.Dx()), float64(height)/float64(bounds.Dy()))
	resizedWidth := int(math.Ceil(float64(bounds.Dx()) * scale))
	resizedHeight := int(math.Ceil(float64(bounds.Dy()) * scale))
	resized := image.NewRGBA(image.Rect(0, 0, resizedWidth, resizedHeight))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), source, bounds, xdraw.Over, nil)
	left, top := (resizedWidth-width)/2, (resizedHeight-height)/2
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), resized, image.Pt(left, top), draw.Src)
	return canvas, nil
}

func compositeCentered(backdrop, overlay *image.RGBA, centerX, centerY int) {
	left, top := centerX-overlay.Bounds().Dx()/2, centerY-overlay.Bounds().Dy()/2
	draw.Draw(backdrop, image.Rect(left, top, left+overlay.Bounds().Dx(), top+overlay.Bounds().Dy()), overlay, image.Point{}, draw.Over)
}

func adaptiveOutlineColor(backdrop, text *image.RGBA, left, top int) (color.NRGBA, bool) {
	var backgroundLightness float64
	var samples, lowContrast int
	height, width := text.Bounds().Dy(), text.Bounds().Dx()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			ink := text.RGBAAt(x, y)
			if ink.A < 192 {
				continue
			}
			backgroundX, backgroundY := left+x, top+y
			if !image.Pt(backgroundX, backgroundY).In(backdrop.Bounds()) {
				continue
			}
			background := backdrop.RGBAAt(backgroundX, backgroundY)
			alpha := float64(ink.A) / 255
			red := min(255, int(float64(ink.R)/alpha))
			green := min(255, int(float64(ink.G)/alpha))
			blue := min(255, int(float64(ink.B)/alpha))
			backgroundLuminosity := relativeLuminance(background.R, background.G, background.B)
			fillLuminosity := relativeLuminance(uint8(red), uint8(green), uint8(blue))
			samples++
			if contrastRatio(fillLuminosity, backgroundLuminosity) < 3 {
				lowContrast++
				backgroundLightness += backgroundLuminosity
			}
		}
	}
	if samples == 0 || lowContrast*10 < samples {
		return color.NRGBA{}, false
	}
	if backgroundLightness/float64(lowContrast) > 0.179 {
		return color.NRGBA{A: 200}, true
	}
	return color.NRGBA{R: 255, G: 255, B: 255, A: 200}, true
}

func outlineText(text *image.RGBA, outlineColor color.NRGBA, radius int) *image.RGBA {
	width, height := text.Bounds().Dx(), text.Bounds().Dy()
	original := image.NewAlpha(text.Bounds())
	dilated := image.NewAlpha(text.Bounds())
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			alpha := text.RGBAAt(x, y).A
			original.SetAlpha(x, y, color.Alpha{A: alpha})
			if alpha == 0 {
				continue
			}
			for offsetY := max(0, y-radius); offsetY <= min(height-1, y+radius); offsetY++ {
				for offsetX := max(0, x-radius); offsetX <= min(width-1, x+radius); offsetX++ {
					if alpha > dilated.AlphaAt(offsetX, offsetY).A {
						dilated.SetAlpha(offsetX, offsetY, color.Alpha{A: alpha})
					}
				}
			}
		}
	}
	outlineMask := image.NewAlpha(text.Bounds())
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			outer := int(dilated.AlphaAt(x, y).A) - int(original.AlphaAt(x, y).A)
			if outer > 0 {
				outlineMask.SetAlpha(x, y, color.Alpha{A: uint8(outer)})
			}
		}
	}
	outline := image.NewRGBA(text.Bounds())
	draw.DrawMask(outline, outline.Bounds(), image.NewUniform(outlineColor), image.Point{}, outlineMask, image.Point{}, draw.Over)
	return outline
}

func contrastRatio(first, second float64) float64 {
	if first < second {
		first, second = second, first
	}
	return (first + 0.05) / (second + 0.05)
}

func relativeLuminance(red, green, blue uint8) float64 {
	return 0.2126*linearLuminance[red] + 0.7152*linearLuminance[green] + 0.0722*linearLuminance[blue]
}

type textPalette struct {
	Arabic      color.RGBA
	Translation color.RGBA
	Reference   color.RGBA
}

func themeTextPalette(colors themeColors) textPalette {
	return textPalette{
		Arabic:      parseColor(colors.Accent),
		Translation: parseColor(colors.Foreground),
		Reference:   parseColor(colors.Muted),
	}
}

func wallpaperTextPalette(backdrop *image.RGBA, themeAccent color.RGBA) textPalette {
	const hueBuckets = 24
	var hueWeights [hueBuckets]float64
	var hueSaturation [hueBuckets]float64
	var totalLightness float64
	var sampleCount int
	width, height := backdrop.Bounds().Dx(), backdrop.Bounds().Dy()
	for row := 0; row < 64; row++ {
		y := row * height / 64
		for column := 0; column < 64; column++ {
			x := column * width / 64
			pixel := backdrop.RGBAAt(x, y)
			hue, saturation, lightness := rgbToHSL(pixel.R, pixel.G, pixel.B)
			totalLightness += relativeLuminance(pixel.R, pixel.G, pixel.B)
			sampleCount++
			if saturation < 0.18 || lightness < 0.08 || lightness > 0.94 {
				continue
			}
			bucket := int(hue / 15)
			weight := saturation * (1 - math.Abs(lightness-0.55)*0.7)
			hueWeights[bucket] += weight
			hueSaturation[bucket] += saturation * weight
		}
	}
	backgroundLightness := 0.5
	if sampleCount > 0 {
		backgroundLightness = totalLightness / float64(sampleCount)
	}
	bestBucket := -1
	for bucket, weight := range hueWeights {
		if bestBucket < 0 || weight > hueWeights[bestBucket] {
			bestBucket = bucket
		}
	}
	hue, saturation, _ := rgbToHSL(themeAccent.R, themeAccent.G, themeAccent.B)
	if bestBucket >= 0 && hueWeights[bestBucket] > 0 {
		hue = (float64(bestBucket) + 0.5) * 15
		saturation = hueSaturation[bestBucket] / hueWeights[bestBucket]
	}
	saturation = min(0.9, max(0.65, saturation+0.18))
	darkBackdrop := backgroundLightness < 0.45
	accentLightness, foregroundLightness, referenceLightness := 0.34, 0.10, 0.29
	if darkBackdrop {
		accentLightness, foregroundLightness, referenceLightness = 0.74, 0.94, 0.63
	}
	return textPalette{
		Arabic:      readablePaletteColor(hue, saturation, backgroundLightness, 3, accentLightness),
		Translation: readablePaletteColor(hue, 0.08, backgroundLightness, 4.5, foregroundLightness),
		Reference:   readablePaletteColor(hue, 0.2, backgroundLightness, 3, referenceLightness),
	}
}

func readablePaletteColor(hue, saturation, backgroundLightness, targetContrast, preferredLightness float64) color.RGBA {
	bestDistance := math.MaxFloat64
	var best color.RGBA
	for step := 1; step < 100; step++ {
		lightness := float64(step) / 100
		candidate := hslColor(hue, saturation, lightness)
		if contrastRatio(relativeLuminance(candidate.R, candidate.G, candidate.B), backgroundLightness) < targetContrast {
			continue
		}
		distance := math.Abs(lightness - preferredLightness)
		if distance < bestDistance {
			bestDistance, best = distance, candidate
		}
	}
	if bestDistance < math.MaxFloat64 {
		return best
	}
	black := hslColor(hue, saturation, 0.01)
	white := hslColor(hue, saturation, 0.99)
	if contrastRatio(relativeLuminance(black.R, black.G, black.B), backgroundLightness) > contrastRatio(relativeLuminance(white.R, white.G, white.B), backgroundLightness) {
		return black
	}
	return white
}

func adjustTextColor(backdrop, text *image.RGBA, left, top int, current color.RGBA, targetContrast float64) color.RGBA {
	var backgroundLightness float64
	var samples int
	for y := 0; y < text.Bounds().Dy(); y++ {
		for x := 0; x < text.Bounds().Dx(); x++ {
			if text.RGBAAt(x, y).A < 192 {
				continue
			}
			point := image.Pt(left+x, top+y)
			if point.In(backdrop.Bounds()) {
				pixel := backdrop.RGBAAt(point.X, point.Y)
				backgroundLightness += relativeLuminance(pixel.R, pixel.G, pixel.B)
				samples++
			}
		}
	}
	if samples == 0 {
		return current
	}
	backgroundLightness /= float64(samples)
	if contrastRatio(relativeLuminance(current.R, current.G, current.B), backgroundLightness) >= targetContrast {
		return current
	}
	hue, saturation, lightness := rgbToHSL(current.R, current.G, current.B)
	return readablePaletteColor(hue, saturation, backgroundLightness, targetContrast, lightness)
}

func recolorText(text *image.RGBA, fill color.RGBA) {
	for y := 0; y < text.Bounds().Dy(); y++ {
		for x := 0; x < text.Bounds().Dx(); x++ {
			alpha := text.RGBAAt(x, y).A
			if alpha == 0 {
				continue
			}
			text.SetRGBA(x, y, color.RGBA{
				R: uint8(uint16(fill.R) * uint16(alpha) / 255),
				G: uint8(uint16(fill.G) * uint16(alpha) / 255),
				B: uint8(uint16(fill.B) * uint16(alpha) / 255),
				A: alpha,
			})
		}
	}
}

func rgbToHSL(red, green, blue uint8) (float64, float64, float64) {
	r, g, b := float64(red)/255, float64(green)/255, float64(blue)/255
	maximum, minimum := max(r, g, b), min(r, g, b)
	delta := maximum - minimum
	lightness := (maximum + minimum) / 2
	if delta == 0 {
		return 45, 0, lightness
	}
	saturation := delta / (1 - math.Abs(2*lightness-1))
	hue := 0.0
	switch maximum {
	case r:
		hue = math.Mod((g-b)/delta, 6)
	case g:
		hue = (b-r)/delta + 2
	default:
		hue = (r-g)/delta + 4
	}
	hue *= 60
	if hue < 0 {
		hue += 360
	}
	return hue, saturation, lightness
}

func hslColor(hue, saturation, lightness float64) color.RGBA {
	chroma := (1 - math.Abs(2*lightness-1)) * saturation
	secondary := chroma * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	red, green, blue := 0.0, 0.0, 0.0
	switch {
	case hue < 60:
		red, green = chroma, secondary
	case hue < 120:
		red, green = secondary, chroma
	case hue < 180:
		green, blue = chroma, secondary
	case hue < 240:
		green, blue = secondary, chroma
	case hue < 300:
		red, blue = secondary, chroma
	default:
		red, blue = chroma, secondary
	}
	offset := lightness - chroma/2
	return color.RGBA{
		R: uint8(math.Round((red + offset) * 255)),
		G: uint8(math.Round((green + offset) * 255)),
		B: uint8(math.Round((blue + offset) * 255)),
		A: 255,
	}
}

func shadowText(backdrop, text *image.RGBA, centerX, centerY int) {
	width, height := text.Bounds().Dx(), text.Bounds().Dy()
	mask := image.NewAlpha(text.Bounds())
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			mask.SetAlpha(x, y, color.Alpha{A: text.RGBAAt(x, y).A})
		}
	}
	blurAlpha(mask, 10)
	shadow := image.NewRGBA(text.Bounds())
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			a := mask.AlphaAt(x, y).A * 204 / 255
			shadow.SetRGBA(x, y, color.RGBA{A: a})
		}
	}
	left, top := centerX-width/2, centerY-height/2
	draw.Draw(backdrop, image.Rect(left, top, left+width, top+height), shadow, image.Point{}, draw.Over)
}

func blurAlpha(mask *image.Alpha, radius int) {
	if radius <= 0 {
		return
	}
	width, height := mask.Bounds().Dx(), mask.Bounds().Dy()
	temp := make([]uint8, width*height)
	kernel := make([]float64, 2*radius+1)
	sigma := float64(radius)
	sum := 0.0
	for i := -radius; i <= radius; i++ {
		value := math.Exp(-float64(i*i) / (2 * sigma * sigma))
		kernel[i+radius] = value
		sum += value
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value := 0.0
			for k := -radius; k <= radius; k++ {
				xx := min(width-1, max(0, x+k))
				value += float64(mask.AlphaAt(xx, y).A) * kernel[k+radius]
			}
			temp[y*width+x] = uint8(value + 0.5)
		}
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value := 0.0
			for k := -radius; k <= radius; k++ {
				yy := min(height-1, max(0, y+k))
				value += float64(temp[yy*width+x]) * kernel[k+radius]
			}
			mask.SetAlpha(x, y, color.Alpha{A: uint8(value + 0.5)})
		}
	}
}

func parseColor(value string) color.RGBA {
	if len(value) != 7 || value[0] != '#' {
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	var red, green, blue uint8
	_, err := fmt.Sscanf(value, "#%02x%02x%02x", &red, &green, &blue)
	if err != nil {
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.RGBA{R: red, G: green, B: blue, A: 255}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
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
	path := filepath.Join(r.Config.Home, ".local", "state", "omarchy", "current", "theme", "colors.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return colors
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

func init() {
	image.RegisterFormat("webp", "RIFF????WEBP", webp.Decode, webp.DecodeConfig)
}
