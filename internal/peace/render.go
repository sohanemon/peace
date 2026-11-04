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
	"unicode"
	"unicode/utf8"

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
	"io.github.sohanemon/peace/internal/quran"
)

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
	Image          *image.RGBA
	Height         int
	Width          int
	AdvanceWidth   int
	AdvanceCenterX int
	Baseline       int
	LineHeight     int
	LineCount      int
}

const (
	wallpaperHeroWidthRatio                   = 0.78
	wallpaperFooterWidthRatio                 = 0.76
	wallpaperTopMarginRatio                   = 0.06
	wallpaperBottomMarginRatio                = 0.07
	wallpaperHeroFooterGapRatio               = 0.045
	wallpaperFooterMaxHeightRatio             = 0.32
	wallpaperFooterItemGapRatio               = 0.012
	wallpaperFooterScrimHeightRatio           = 0.28
	wallpaperFooterScrimMaxAlpha        uint8 = 76
	wallpaperReferenceOpacity           uint8 = 184
	wallpaperTranslationFontSize              = 30
	wallpaperTranslationMinimumFontSize       = 12
	wallpaperReferenceFontSize                = 17
	wallpaperReferenceMinimumFontSize         = 10
	wallpaperFooterMinimumItemGap             = 8
	wallpaperPlainArabicFontSize              = 76
	wallpaperPlainArabicMinimumFontSize       = 24
	wallpaperMinimumFontScale                 = 30
	wallpaperFontScaleStep                    = 90
)

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
	Arabic, Translation, Interface *font.Face
	err                            error
}
var systemFontFaces struct {
	mu       sync.Mutex
	byScript map[language.Script]*font.Face
}

func (r Runtime) renderAyah(ctx context.Context, client *quran.Client, translation, wbwLanguage string, surah, ayah int, baseImage, output string, showBasmala bool, renderOptions RenderOptions) error {
	entry, err := client.AyahWithWBW(ctx, surah, ayah, translation, wbwLanguage)
	if err != nil {
		return err
	}
	entry.Arabic = displayAyahArabic(surah, ayah, entry.Arabic, showBasmala)
	return r.renderAyahText(entry, baseImage, output, renderOptions)
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

func (r Runtime) renderAyahText(ayah quran.Ayah, baseImage, output string, renderOptions RenderOptions) error {
	if err := loadRenderFonts(); err != nil {
		return err
	}
	if !utf8.ValidString(ayah.Translation) || !utf8.ValidString(ayah.Arabic) {
		return fmt.Errorf("ayah %d:%d contains invalid UTF-8 text", ayah.Surah, ayah.Number)
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
	fontScale := min(width, height*16/9) * 100 / 2560
	fontScale = min(150, max(60, fontScale))
	footer, err := renderWallpaperFooter(ayah, renderOptions, palette, width, height, fontScale)
	if err != nil {
		return fmt.Errorf("render ayah %d:%d footer: %w", ayah.Surah, ayah.Number, err)
	}
	mainArea := layoutWallpaperText(width, height, 0, footer.Height)
	var hero renderedText
	var heroStyle *textStyle
	if renderOptions.ShowWordTranslation && len(ayah.Words) > 0 {
		hero, err = renderWordByWordBlock(ayah, palette, width, mainArea.MainHeight, fontScale)
	} else {
		var style textStyle
		hero, style, err = renderPlainArabicBlock(ayah, palette, width, mainArea.MainHeight, fontScale)
		heroStyle = &style
	}
	if err != nil {
		return fmt.Errorf("render ayah %d:%d Arabic block: %w", ayah.Surah, ayah.Number, err)
	}
	if hero.Height > mainArea.MainHeight {
		hero = scaleRenderedTextToHeight(hero, mainArea.MainHeight)
	}
	layout := layoutWallpaperText(width, height, hero.Height, footer.Height)
	return composeWallpaperText(backdrop, hero, heroStyle, footer, layout, renderOptions.UseTheme, output)
}

type wbwColumn struct {
	Arabic         renderedText
	Meaning        renderedText
	Width          int
	ArabicMetrics  wbwLineMetrics
	MeaningMetrics wbwLineMetrics
	MeaningGap     int
}

type wbwRow struct {
	Columns         []wbwColumn
	Width           int
	Height          int
	ArabicBaseline  int
	MeaningBaseline int
}

type wbwColumnPlacement struct {
	ColumnLeft      int
	ArabicLeft      int
	MeaningLeft     int
	ArabicTop       int
	MeaningTop      int
	Center          int
	ArabicBaseline  int
	MeaningBaseline int
}

type wallpaperFooterItem struct {
	Text           renderedText
	Top            int
	Color          color.RGBA
	TargetContrast float64
	Reference      bool
}

type wallpaperFooter struct {
	Width  int
	Height int
	Items  []wallpaperFooterItem
}

type wallpaperTextLayout struct {
	HeroTop            int
	MainTop            int
	MainBottom         int
	MainHeight         int
	FooterTop          int
	FooterBottom       int
	FooterBottomMargin int
	FooterWidth        int
	HeroFooterGap      int
}

func renderWordByWordBlock(ayah quran.Ayah, palette textPalette, width, maxHeight, fontScale int) (renderedText, error) {
	textWidth := width * 86 / 100
	columnMaxWidth := max(120, textWidth/4)
	var wordBlock renderedText
	var rows []wbwRow
	for attempt := 0; attempt < 20; attempt++ {
		arabicStyle := textStyle{Size: 48 * fontScale / 100, Minimum: 14, Face: fontFaces.Arabic, Color: palette.Arabic, Direction: di.DirectionRTL, Script: language.Arabic}
		meaningStyle := textStyle{Size: 22 * fontScale / 100, Minimum: 9, Face: fontFaces.Translation, Color: palette.Translation, Direction: di.DirectionLTR, Script: language.Latin}
		var err error
		columnGap := max(12, arabicStyle.Size/3)
		rows, err = renderWBWRows(ayah.Words, textWidth, columnMaxWidth, max(10, arabicStyle.Size/3), columnGap, max(10, arabicStyle.Size/3), arabicStyle, meaningStyle)
		if err != nil {
			if attempt == 19 {
				return renderedText{}, fmt.Errorf("render ayah %d:%d word meanings: %w", ayah.Surah, ayah.Number, err)
			}
			fontScale = max(wallpaperMinimumFontScale, fontScale*wallpaperFontScaleStep/100)
			continue
		}
		wordBlock = composeWBWRows(rows, textWidth, max(10, arabicStyle.Size/3), columnGap)
		if wordBlock.Height <= maxHeight {
			return wordBlock, nil
		}
		if attempt == 19 {
			return scaleRenderedTextToHeight(wordBlock, maxHeight), nil
		}
		fontScale = max(wallpaperMinimumFontScale, fontScale*wallpaperFontScaleStep/100)
	}
	return renderedText{}, fmt.Errorf("ayah %d:%d word-by-word block does not fit the main area", ayah.Surah, ayah.Number)
}

func renderWallpaperFooter(ayah quran.Ayah, options RenderOptions, palette textPalette, width, height, fontScale int) (wallpaperFooter, error) {
	footerWidth := ratioPixels(width, wallpaperFooterWidthRatio)
	maxHeight := ratioPixels(height, wallpaperFooterMaxHeightRatio)
	itemGap := max(wallpaperFooterMinimumItemGap, ratioPixels(height, wallpaperFooterItemGapRatio))
	for attempt := 0; attempt < 24; attempt++ {
		footer := wallpaperFooter{Width: footerWidth}
		if options.ShowTranslation && strings.TrimSpace(ayah.Translation) != "" {
			style := textStyleForText(textStyle{
				Size: max(wallpaperTranslationMinimumFontSize, wallpaperTranslationFontSize*fontScale/100), Minimum: wallpaperTranslationMinimumFontSize,
				Face: fontFaces.Translation, Color: palette.Translation, TargetContrast: 4.5, Direction: di.DirectionLTR, Script: language.Latin,
			}, trimEdgeQuote(ayah.Translation))
			block, err := renderBalancedTextBlock(style, footerWidth)
			if err != nil {
				if attempt == 23 {
					return wallpaperFooter{}, err
				}
				fontScale = max(wallpaperMinimumFontScale, fontScale*wallpaperFontScaleStep/100)
				continue
			}
			footer.Items = append(footer.Items, wallpaperFooterItem{Text: block, Color: style.Color, TargetContrast: style.TargetContrast})
		}
		if options.ShowReference {
			style := textStyle{
				Text: ayahReference(ayah), Size: max(wallpaperReferenceMinimumFontSize, wallpaperReferenceFontSize*fontScale/100), Minimum: wallpaperReferenceMinimumFontSize, Face: fontFaces.Interface,
				Color: palette.Reference, TargetContrast: 3, Direction: di.DirectionLTR, Script: language.Latin,
			}
			block, err := renderTextBlock(style, footerWidth)
			if err != nil {
				if attempt == 23 {
					return wallpaperFooter{}, err
				}
				fontScale = max(wallpaperMinimumFontScale, fontScale*wallpaperFontScaleStep/100)
				continue
			}
			footer.Items = append(footer.Items, wallpaperFooterItem{Text: block, Color: style.Color, TargetContrast: style.TargetContrast, Reference: true})
		}
		for index := range footer.Items {
			if index > 0 {
				footer.Height += itemGap
				footer.Items[index].Top = footer.Height
			}
			footer.Height += footer.Items[index].Text.Height
		}
		if footer.Height <= maxHeight {
			if len(footer.Items) == 0 {
				footer.Width = 0
			}
			return footer, nil
		}
		if attempt == 23 {
			return wallpaperFooter{}, fmt.Errorf("footer height %d exceeds available height %d", footer.Height, maxHeight)
		}
		fontScale = max(wallpaperMinimumFontScale, fontScale*wallpaperFontScaleStep/100)
	}
	return wallpaperFooter{}, fmt.Errorf("footer does not fit the display")
}

func layoutWallpaperText(width, height, heroHeight, footerHeight int) wallpaperTextLayout {
	layout := wallpaperTextLayout{
		FooterBottomMargin: ratioPixels(height, wallpaperBottomMarginRatio),
		MainTop:            ratioPixels(height, wallpaperTopMarginRatio),
		HeroFooterGap:      ratioPixels(height, wallpaperHeroFooterGapRatio),
		FooterWidth:        ratioPixels(width, wallpaperFooterWidthRatio),
	}
	layout.FooterBottom = height - layout.FooterBottomMargin
	layout.FooterTop = layout.FooterBottom - footerHeight
	layout.MainBottom = layout.FooterBottom
	if footerHeight > 0 {
		layout.MainBottom = layout.FooterTop - layout.HeroFooterGap
	}
	layout.MainBottom = max(layout.MainTop, layout.MainBottom)
	layout.MainHeight = layout.MainBottom - layout.MainTop
	layout.HeroTop = layout.MainTop + max(0, layout.MainHeight-heroHeight)/2
	return layout
}

func renderPlainArabicBlock(ayah quran.Ayah, palette textPalette, width, maxHeight, fontScale int) (renderedText, textStyle, error) {
	style := textStyle{
		Text: ayah.Arabic, Size: wallpaperPlainArabicFontSize * fontScale / 100, Minimum: wallpaperPlainArabicMinimumFontSize, Face: fontFaces.Arabic,
		Color: palette.Arabic, TargetContrast: 3, Outline: true, Direction: di.DirectionRTL, Script: language.Arabic,
	}
	maxWidth := ratioPixels(width, wallpaperHeroWidthRatio)
	for attempt := 0; attempt < 20; attempt++ {
		block, err := renderTextBlock(style, maxWidth)
		if err == nil && block.Height <= maxHeight {
			return block, style, nil
		}
		if attempt == 19 || style.Size <= style.Minimum {
			if err != nil {
				return renderedText{}, style, err
			}
			return scaleRenderedTextToHeight(block, maxHeight), style, nil
		}
		style.Size = max(style.Minimum, style.Size*wallpaperFontScaleStep/100)
	}
	return renderedText{}, style, fmt.Errorf("Arabic block does not fit the main area")
}

func composeWallpaperText(backdrop *image.RGBA, hero renderedText, heroStyle *textStyle, footer wallpaperFooter, layout wallpaperTextLayout, useTheme bool, output string) error {
	drawFooterScrim(backdrop, footer.Height)
	heroLeft := (backdrop.Bounds().Dx() - hero.Width) / 2
	heroTop := layout.HeroTop
	if heroStyle != nil && !useTheme {
		heroStyle.Color = adjustTextColor(backdrop, hero.Image, heroLeft, heroTop, heroStyle.Color, heroStyle.TargetContrast)
		recolorText(hero.Image, heroStyle.Color)
	}
	shadowText(backdrop, hero.Image, backdrop.Bounds().Dx()/2, heroTop+hero.Height/2)
	if heroStyle != nil && heroStyle.Outline {
		outlineColor, needed := adaptiveOutlineColor(backdrop, hero.Image, heroLeft, heroTop)
		if needed {
			outlineRadius := max(1, min(2, min(backdrop.Bounds().Dx(), backdrop.Bounds().Dy())/1200))
			outline := outlineText(hero.Image, outlineColor, outlineRadius)
			draw.Draw(backdrop, image.Rect(heroLeft, heroTop, heroLeft+outline.Bounds().Dx(), heroTop+outline.Bounds().Dy()), outline, image.Point{}, draw.Over)
		}
	}
	draw.Draw(backdrop, image.Rect(heroLeft, heroTop, heroLeft+hero.Width, heroTop+hero.Height), hero.Image, image.Point{}, draw.Over)
	if footer.Height > 0 {
		footerCanvas := image.NewRGBA(image.Rect(0, 0, footer.Width, footer.Height))
		footerLeft := (backdrop.Bounds().Dx() - footer.Width) / 2
		for _, item := range footer.Items {
			itemTop := layout.FooterTop + item.Top
			fill := item.Color
			fill = adjustTextColor(backdrop, item.Text.Image, footerLeft, itemTop, fill, item.TargetContrast)
			if item.Reference {
				fill.A = 255
			}
			recolorText(item.Text.Image, fill)
			if item.Reference {
				scaleTextOpacity(item.Text.Image, wallpaperReferenceOpacity)
			}
			outlineColor, needsOutline := adaptiveOutlineColor(backdrop, item.Text.Image, footerLeft, itemTop)
			if needsOutline {
				outlineRadius := max(1, min(2, min(backdrop.Bounds().Dx(), backdrop.Bounds().Dy())/1200))
				outline := outlineText(item.Text.Image, outlineColor, outlineRadius)
				draw.Draw(footerCanvas, image.Rect(0, item.Top, footer.Width, item.Top+item.Text.Height), outline, image.Point{}, draw.Over)
			}
			draw.Draw(footerCanvas, image.Rect(0, item.Top, footer.Width, item.Top+item.Text.Height), item.Text.Image, image.Point{}, draw.Over)
		}
		shadowText(backdrop, footerCanvas, backdrop.Bounds().Dx()/2, layout.FooterTop+footer.Height/2)
		compositeCentered(backdrop, footerCanvas, backdrop.Bounds().Dx()/2, layout.FooterTop+footer.Height/2)
	}
	return saveRenderedWallpaper(backdrop, output)
}

func drawFooterScrim(backdrop *image.RGBA, footerHeight int) {
	if footerHeight == 0 {
		return
	}
	height := ratioPixels(backdrop.Bounds().Dy(), wallpaperFooterScrimHeightRatio)
	start := max(0, backdrop.Bounds().Dy()-height)
	for y := start; y < backdrop.Bounds().Dy(); y++ {
		progress := float64(y-start) / float64(max(1, backdrop.Bounds().Dy()-start))
		alpha := uint8(math.Round(float64(wallpaperFooterScrimMaxAlpha) * progress * progress))
		row := image.Rect(0, y, backdrop.Bounds().Dx(), y+1)
		draw.Draw(backdrop, row, image.NewUniform(color.RGBA{A: alpha}), image.Point{}, draw.Over)
	}
}

func scaleRenderedTextToHeight(text renderedText, height int) renderedText {
	if height <= 0 {
		height = 1
	}
	if text.Height <= height {
		return text
	}
	width := max(1, text.Width*height/text.Height)
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), text.Image, text.Image.Bounds(), xdraw.Over, nil)
	text.Image, text.Width, text.Height = canvas, width, height
	return text
}

func ratioPixels(size int, ratio float64) int {
	return max(1, int(math.Round(float64(size)*ratio)))
}

func renderWBWRows(words []quran.Word, maxWidth, maxColumnWidth, columnPadding, columnGap, rowGap int, arabicStyle, meaningStyle textStyle) ([]wbwRow, error) {
	arabicMetrics, err := measureWBWLineMetrics(words, arabicStyle, maxColumnWidth, false)
	if err != nil {
		return nil, err
	}
	meaningMetrics, err := measureWBWLineMetrics(words, meaningStyle, maxColumnWidth, true)
	if err != nil {
		return nil, err
	}
	columns := make([]wbwColumn, 0, len(words))
	for _, word := range words {
		if strings.TrimSpace(word.Arabic) == "" {
			continue
		}
		arabicStyle.Text = word.Arabic
		arabic, err := renderTextBlockWithMetrics(arabicStyle, maxColumnWidth, &arabicMetrics)
		if err != nil {
			return nil, err
		}
		arabic = cropRenderedText(arabic)
		meaning := renderedText{Image: image.NewRGBA(image.Rect(0, 0, 1, 1)), Width: 1, Height: 1, AdvanceCenterX: 1, LineCount: 1}
		if strings.TrimSpace(word.Translation) != "" {
			wordMeaningStyle := textStyleForText(meaningStyle, strings.TrimSpace(word.Translation))
			meaning, err = renderTextBlockWithMetrics(wordMeaningStyle, maxColumnWidth, &meaningMetrics)
			if err != nil {
				return nil, err
			}
			meaning = cropRenderedText(meaning)
		}
		columnWidth := max(arabic.AdvanceWidth, meaning.AdvanceWidth) + columnPadding*2
		columns = append(columns, wbwColumn{
			Arabic: arabic, Meaning: meaning, Width: columnWidth,
			ArabicMetrics: arabicMetrics, MeaningMetrics: meaningMetrics, MeaningGap: max(8, arabicMetrics.Height/5),
		})
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("ayah has no renderable words")
	}
	rows := layoutWBWRows(columns, maxWidth, columnGap)
	for _, row := range rows {
		if row.Width > maxWidth {
			return nil, fmt.Errorf("word column width %d exceeds available width %d", row.Width, maxWidth)
		}
	}
	return rows, nil
}

func layoutWBWRows(columns []wbwColumn, maxWidth, columnGap int) []wbwRow {
	rows := make([]wbwRow, 0, 4)
	row := wbwRow{}
	for _, column := range columns {
		addedWidth := column.Width
		if len(row.Columns) > 0 {
			addedWidth += columnGap
		}
		if len(row.Columns) > 0 && row.Width+addedWidth > maxWidth {
			setWBWRowMetrics(&row)
			rows = append(rows, row)
			row = wbwRow{}
			addedWidth = column.Width
		}
		row.Columns = append(row.Columns, column)
		row.Width += addedWidth
	}
	if len(row.Columns) > 0 {
		setWBWRowMetrics(&row)
		rows = append(rows, row)
	}
	return rows
}

func measureWBWLineMetrics(words []quran.Word, style textStyle, maxWidth int, meanings bool) (wbwLineMetrics, error) {
	pixelSize := style.Size * 4 / 3
	input := shaping.Input{Direction: style.Direction, Face: style.Face, Size: fixed.I(pixelSize), Script: style.Script}
	shaper := &shaping.HarfbuzzShaper{}
	shape := func(line string) shaping.Output {
		input.Text = []rune(line)
		input.RunEnd = len(input.Text)
		return shaper.Shape(input)
	}
	var ascent, descent int
	measured := false
	for _, word := range words {
		text := word.Arabic
		if meanings {
			text = word.Translation
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		wordStyle := textStyleForText(style, text)
		input.Face = wordStyle.Face
		input.Script = wordStyle.Script
		input.Direction = wordStyle.Direction
		for _, line := range wrapShapedText(text, maxWidth, shape) {
			out := shape(line)
			lineAscent, lineDescent := out.LineBounds.Ascent.Round(), out.LineBounds.Descent.Round()
			for _, glyph := range out.Glyphs {
				lineAscent = max(lineAscent, (glyph.YBearing + glyph.YOffset).Round())
				lineDescent = min(lineDescent, (glyph.YBearing + glyph.Height + glyph.YOffset).Round())
			}
			if !measured {
				ascent, descent = lineAscent, lineDescent
				measured = true
			} else {
				ascent = max(ascent, lineAscent)
				descent = min(descent, lineDescent)
			}
		}
	}
	if !measured && meanings {
		out := shape("M")
		ascent, descent = out.LineBounds.Ascent.Round(), out.LineBounds.Descent.Round()
		for _, glyph := range out.Glyphs {
			ascent = max(ascent, (glyph.YBearing + glyph.YOffset).Round())
			descent = min(descent, (glyph.YBearing + glyph.Height + glyph.YOffset).Round())
		}
		measured = true
	}
	if !measured {
		return wbwLineMetrics{}, fmt.Errorf("no text available for shared line metrics")
	}
	return wbwLineMetrics{Ascent: ascent, Descent: descent, Height: max(pixelSize*6/5, ascent-descent) + 8}, nil
}

func setWBWRowMetrics(row *wbwRow) {
	first := row.Columns[0]
	arabicMetrics, meaningMetrics := first.ArabicMetrics, first.MeaningMetrics
	row.ArabicBaseline = (arabicMetrics.Height-(arabicMetrics.Ascent-arabicMetrics.Descent))/2 + arabicMetrics.Ascent
	row.MeaningBaseline = row.ArabicBaseline - arabicMetrics.Descent + first.MeaningGap + meaningMetrics.Ascent
	maxMeaningLines := 1
	for _, column := range row.Columns {
		maxMeaningLines = max(maxMeaningLines, column.Meaning.LineCount)
	}
	row.Height = max(arabicMetrics.Height, row.MeaningBaseline+(maxMeaningLines-1)*meaningMetrics.Height-meaningMetrics.Descent+4)
}

func composeWBWRows(rows []wbwRow, width, rowGap, columnGap int) renderedText {
	height := 0
	for index, row := range rows {
		if index > 0 {
			height += rowGap
		}
		height += row.Height
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, max(1, height)))
	rowY := 0
	for rowIndex, row := range rows {
		if rowIndex > 0 {
			rowY += rowGap
		}
		placements := wbwColumnPlacements(row, width, columnGap)
		for columnIndex, column := range row.Columns {
			placement := placements[columnIndex]
			arabicX := placement.ArabicLeft
			arabicY := rowY + placement.ArabicTop
			draw.Draw(canvas, image.Rect(arabicX, arabicY, arabicX+column.Arabic.Width, arabicY+column.Arabic.Height), column.Arabic.Image, image.Point{}, draw.Over)
			meaningX := placement.MeaningLeft
			meaningY := rowY + placement.MeaningTop
			draw.Draw(canvas, image.Rect(meaningX, meaningY, meaningX+column.Meaning.Width, meaningY+column.Meaning.Height), column.Meaning.Image, image.Point{}, draw.Over)
		}
		rowY += row.Height
	}
	return renderedText{Image: canvas, Width: width, Height: height}
}

func wbwColumnPlacements(row wbwRow, width, columnGap int) []wbwColumnPlacement {
	lefts := wbwColumnLefts(row, width, columnGap)
	placements := make([]wbwColumnPlacement, len(row.Columns))
	for index, column := range row.Columns {
		center := lefts[index] + column.Width/2
		placements[index] = wbwColumnPlacement{
			ColumnLeft: lefts[index],
			ArabicLeft: center - column.Arabic.AdvanceCenterX, MeaningLeft: center - column.Meaning.AdvanceCenterX,
			ArabicTop: row.ArabicBaseline - column.Arabic.Baseline, MeaningTop: row.MeaningBaseline - column.Meaning.Baseline,
			Center: center, ArabicBaseline: row.ArabicBaseline, MeaningBaseline: row.MeaningBaseline,
		}
	}
	return placements
}

func wbwColumnLefts(row wbwRow, width, gap int) []int {
	lefts := make([]int, len(row.Columns))
	right := (width-row.Width)/2 + row.Width
	for index, column := range row.Columns {
		lefts[index] = right - column.Width
		right = lefts[index] - gap
	}
	return lefts
}

func cropRenderedText(text renderedText) renderedText {
	alphaBounds := image.Rectangle{}
	for y := 0; y < text.Height; y++ {
		for x := 0; x < text.Width; x++ {
			if text.Image.RGBAAt(x, y).A == 0 {
				continue
			}
			point := image.Pt(x, y)
			if alphaBounds.Empty() {
				alphaBounds = image.Rectangle{Min: point, Max: point.Add(image.Pt(1, 1))}
			} else {
				alphaBounds = alphaBounds.Union(image.Rectangle{Min: point, Max: point.Add(image.Pt(1, 1))})
			}
		}
	}
	if alphaBounds.Empty() {
		return renderedText{Image: image.NewRGBA(image.Rect(0, 0, 1, 1)), Width: 1, Height: 1}
	}
	alphaBounds = alphaBounds.Inset(-2).Intersect(text.Image.Bounds())
	canvas := image.NewRGBA(image.Rect(0, 0, alphaBounds.Dx(), alphaBounds.Dy()))
	draw.Draw(canvas, canvas.Bounds(), text.Image, alphaBounds.Min, draw.Src)
	text.Image = canvas
	text.Width, text.Height = canvas.Bounds().Dx(), canvas.Bounds().Dy()
	text.AdvanceCenterX -= alphaBounds.Min.X
	text.Baseline -= alphaBounds.Min.Y
	return text
}

func saveRenderedWallpaper(backdrop *image.RGBA, output string) error {
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
			fontFaces.Translation, fontFaces.err = parseEmbeddedFont("fonts/DejaVuSerif.ttf")
		}
		if fontFaces.err == nil {
			fontFaces.Interface, fontFaces.err = parseEmbeddedFont("fonts/DejaVuSans.ttf")
		}
	})
	return fontFaces.err
}

func selectFontFace(text string, preferred *font.Face) *font.Face {
	if faceCoversText(preferred, text) {
		return preferred
	}
	script := textScript(text, language.Latin)
	return systemFontForScript(script, text)
}

func textStyleForText(style textStyle, text string) textStyle {
	style.Text = text
	style.Face = selectFontFace(text, style.Face)
	style.Script = textScript(text, style.Script)
	style.Direction = textDirection(style.Script, style.Direction)
	return style
}

func textDirection(script language.Script, fallback di.Direction) di.Direction {
	switch script.String() {
	case "Arab", "Hebr", "Syrc", "Thaa", "Nkoo", "Adlm", "Rohg", "Sogd", "Sogo", "Mand", "Mend", "Phnx":
		return di.DirectionRTL
	default:
		return fallback
	}
}

func faceCoversText(face *font.Face, text string) bool {
	if face == nil {
		return false
	}
	for _, current := range text {
		if unicode.IsSpace(current) || unicode.Is(unicode.Mn, current) || unicode.Is(unicode.Cf, current) || current == '\ufe0f' {
			continue
		}
		if _, ok := face.NominalGlyph(current); !ok {
			return false
		}
	}
	return true
}

func textScript(text string, fallback language.Script) language.Script {
	for _, current := range text {
		if script := language.LookupScript(current); script.Strong() {
			return script
		}
	}
	return fallback
}

func systemFontForScript(script language.Script, text string) *font.Face {
	systemFontFaces.mu.Lock()
	defer systemFontFaces.mu.Unlock()
	if systemFontFaces.byScript == nil {
		systemFontFaces.byScript = make(map[language.Script]*font.Face)
	}
	if face := systemFontFaces.byScript[script]; face != nil && faceCoversText(face, text) {
		return face
	}
	representative := scriptRepresentative(script)
	if representative == 0 {
		return fontFaces.Translation
	}
	pattern := fmt.Sprintf("serif:charset=%04x:weight=80", representative)
	output, err := exec.Command("fc-match", "--format", "%{file}", pattern).Output()
	if err != nil {
		return fontFaces.Translation
	}
	path := strings.TrimSpace(string(output))
	data, err := os.ReadFile(path)
	if err != nil {
		return fontFaces.Translation
	}
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil || !faceCoversText(face, text) {
		return fontFaces.Translation
	}
	systemFontFaces.byScript[script] = face
	return face
}

func scriptRepresentative(script language.Script) rune {
	for current := rune(0x20); current <= unicode.MaxRune; current++ {
		if unicode.IsLetter(current) && language.LookupScript(current) == script {
			return current
		}
	}
	return 0
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

type wbwLineMetrics struct {
	Ascent  int
	Descent int
	Height  int
}

func renderTextBlock(style textStyle, maxWidth int) (renderedText, error) {
	return renderTextBlockWithWrapping(style, maxWidth, nil, wrapShapedText)
}

func renderBalancedTextBlock(style textStyle, maxWidth int) (renderedText, error) {
	return renderTextBlockWithWrapping(style, maxWidth, nil, wrapBalancedShapedText)
}

func renderTextBlockWithMetrics(style textStyle, maxWidth int, fixedMetrics *wbwLineMetrics) (renderedText, error) {
	return renderTextBlockWithWrapping(style, maxWidth, fixedMetrics, wrapShapedText)
}

func renderTextBlockWithWrapping(style textStyle, maxWidth int, fixedMetrics *wbwLineMetrics, wrap func(string, int, func(string) shaping.Output) []string) (renderedText, error) {
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
	lines := wrap(text, maxWidth, shape)
	type textLine struct {
		Output   shaping.Output
		Ascent   int
		Descent  int
		Height   int
		Baseline int
	}
	lineLayouts := make([]textLine, len(lines))
	imageHeight := 0
	advanceWidth := 0
	for row, line := range lines {
		out := shape(line)
		ascent, descent := out.LineBounds.Ascent.Round(), out.LineBounds.Descent.Round()
		if fixedMetrics != nil {
			ascent, descent = fixedMetrics.Ascent, fixedMetrics.Descent
		} else {
			for _, glyph := range out.Glyphs {
				glyphAscent := (glyph.YBearing + glyph.YOffset).Round()
				glyphDescent := (glyph.YBearing + glyph.Height + glyph.YOffset).Round()
				ascent = max(ascent, glyphAscent)
				descent = min(descent, glyphDescent)
			}
		}
		lineHeight := max(pixelSize*6/5, ascent-descent) + 8
		if fixedMetrics != nil {
			lineHeight = fixedMetrics.Height
		}
		baseline := (lineHeight-(ascent-descent))/2 + ascent
		lineLayouts[row] = textLine{Output: out, Ascent: ascent, Descent: descent, Height: lineHeight, Baseline: baseline}
		imageHeight += lineHeight
		advanceWidth = max(advanceWidth, abs(out.Advance.Round()))
	}
	imageHeight = max(1, imageHeight)
	mask := image.NewAlpha(image.Rect(0, 0, maxWidth, imageHeight))
	rowY := 0
	firstBaseline := 0
	for rowIndex, line := range lineLayouts {
		out := line.Output
		lineWidth := abs(out.Advance.Round())
		if lineWidth > maxWidth {
			return renderedText{}, fmt.Errorf("line width %d exceeds available width %d", lineWidth, maxWidth)
		}
		penX := (maxWidth - lineWidth) / 2
		if out.Direction == di.DirectionRTL && out.Advance < 0 {
			penX += lineWidth
		}
		baseline := rowY + line.Baseline
		if rowIndex == 0 {
			firstBaseline = baseline
		}
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
	lineHeight := 0
	if len(lineLayouts) > 0 {
		lineHeight = lineLayouts[0].Height
	}
	return renderedText{
		Image: canvas, Width: maxWidth, Height: imageHeight, AdvanceWidth: advanceWidth,
		AdvanceCenterX: maxWidth / 2, Baseline: firstBaseline, LineHeight: lineHeight, LineCount: len(lineLayouts),
	}, nil
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

func wrapBalancedShapedText(text string, maxWidth int, shape func(string) shaping.Output) []string {
	lines := wrapShapedText(text, maxWidth, shape)
	for pass := 0; pass < len(lines)*2; pass++ {
		changed := false
		for index := 0; index+1 < len(lines); index++ {
			previousWords := strings.Fields(lines[index])
			nextWords := strings.Fields(lines[index+1])
			if len(previousWords) < 2 {
				continue
			}
			balancedPrevious := strings.Join(previousWords[:len(previousWords)-1], " ")
			balancedNext := strings.Join(append([]string{previousWords[len(previousWords)-1]}, nextWords...), " ")
			if abs(shape(balancedNext).Advance.Round()) > maxWidth {
				continue
			}
			before := abs(abs(shape(lines[index]).Advance.Round()) - abs(shape(lines[index+1]).Advance.Round()))
			after := abs(abs(shape(balancedPrevious).Advance.Round()) - abs(shape(balancedNext).Advance.Round()))
			if after < before {
				lines[index], lines[index+1] = balancedPrevious, balancedNext
				changed = true
			}
		}
		if !changed {
			break
		}
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

func scaleTextOpacity(text *image.RGBA, opacity uint8) {
	for y := 0; y < text.Bounds().Dy(); y++ {
		for x := 0; x < text.Bounds().Dx(); x++ {
			pixel := text.RGBAAt(x, y)
			text.SetRGBA(x, y, color.RGBA{
				R: uint8(uint16(pixel.R) * uint16(opacity) / 255),
				G: uint8(uint16(pixel.G) * uint16(opacity) / 255),
				B: uint8(uint16(pixel.B) * uint16(opacity) / 255),
				A: uint8(uint16(pixel.A) * uint16(opacity) / 255),
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

func ayahReference(ayah quran.Ayah) string {
	if ayah.SurahName == "" {
		return fmt.Sprintf("%d:%d", ayah.Surah, ayah.Number)
	}
	return fmt.Sprintf("%s %d:%d", ayah.SurahName, ayah.Surah, ayah.Number)
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
