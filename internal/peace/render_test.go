package peace

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
	"io.github.sohanemon/peace/internal/quran"
)

func TestLayoutWBWRowsWrapsAndPreservesReadingOrder(t *testing.T) {
	columns := testWBWColumns()
	rows := layoutWBWRows(columns, 130, 6)
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	if len(rows[0].Columns) != 2 || len(rows[1].Columns) != 1 {
		t.Fatalf("row column counts = %d, %d; want 2, 1", len(rows[0].Columns), len(rows[1].Columns))
	}
	if rows[0].Columns[0].Width != 40 || rows[0].Columns[1].Width != 50 || rows[1].Columns[0].Width != 35 {
		t.Fatal("wrapping changed the input word order")
	}
	if rows[0].Height <= 0 || rows[1].Height <= 0 {
		t.Fatalf("row heights must be positive: %d, %d", rows[0].Height, rows[1].Height)
	}
}

func TestWBWColumnsShareBaselinesAndAdvanceCenters(t *testing.T) {
	rows := layoutWBWRows(testWBWColumns(), 300, 8)
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	row := rows[0]
	placements := wbwColumnPlacements(row, 300, 8)
	for index, column := range row.Columns {
		placement := placements[index]
		if placement.ArabicTop+column.Arabic.Baseline != row.ArabicBaseline {
			t.Errorf("Arabic column %d baseline = %d, want shared %d", index, placement.ArabicTop+column.Arabic.Baseline, row.ArabicBaseline)
		}
		if placement.MeaningTop+column.Meaning.Baseline != row.MeaningBaseline {
			t.Errorf("meaning column %d first baseline = %d, want shared %d", index, placement.MeaningTop+column.Meaning.Baseline, row.MeaningBaseline)
		}
		if column.Meaning.LineCount > 1 && column.Meaning.LineHeight <= 0 {
			t.Errorf("wrapped meaning column %d has invalid line height %d", index, column.Meaning.LineHeight)
		}
		if placement.ArabicLeft+column.Arabic.AdvanceCenterX != placement.Center || placement.MeaningLeft+column.Meaning.AdvanceCenterX != placement.Center {
			t.Errorf("column %d Arabic and meaning advance centers do not match at x=%d", index, placement.Center)
		}
		if index > 0 {
			nextRight := placements[index].ColumnLeft + column.Width
			previousLeft := placements[index-1].ColumnLeft
			gap := previousLeft - nextRight
			if gap != 8 {
				t.Errorf("gap before column %d = %d, want uniform gap 8", index, gap)
			}
		}
	}
}

func testWBWColumns() []wbwColumn {
	metrics := wbwLineMetrics{Ascent: 10, Descent: -4, Height: 24}
	return []wbwColumn{
		{
			Width: 40, Arabic: renderedText{Width: 28, Height: 20, AdvanceWidth: 24, AdvanceCenterX: 14, Baseline: 13},
			Meaning:       renderedText{Width: 30, Height: 12, AdvanceWidth: 30, AdvanceCenterX: 15, Baseline: 8, LineHeight: 14, LineCount: 1},
			ArabicMetrics: metrics, MeaningMetrics: metrics, MeaningGap: 6,
		},
		{
			Width: 50, Arabic: renderedText{Width: 42, Height: 26, AdvanceWidth: 34, AdvanceCenterX: 19, Baseline: 17},
			Meaning:       renderedText{Width: 46, Height: 24, AdvanceWidth: 46, AdvanceCenterX: 23, Baseline: 16, LineHeight: 14, LineCount: 2},
			ArabicMetrics: metrics, MeaningMetrics: metrics, MeaningGap: 6,
		},
		{
			Width: 35, Arabic: renderedText{Width: 30, Height: 17, AdvanceWidth: 25, AdvanceCenterX: 13, Baseline: 11},
			Meaning:       renderedText{Width: 24, Height: 11, AdvanceWidth: 20, AdvanceCenterX: 12, Baseline: 7, LineHeight: 14, LineCount: 1},
			ArabicMetrics: metrics, MeaningMetrics: metrics, MeaningGap: 6,
		},
	}
}

func TestWallpaperTextLayoutCentersHeroAndAnchorsFooter(t *testing.T) {
	const width, height, heroHeight, footerHeight = 1920, 1080, 240, 180
	layout := layoutWallpaperText(width, height, heroHeight, footerHeight)
	mainCenter := layout.MainTop + layout.MainHeight/2
	heroCenter := layout.HeroTop + heroHeight/2
	if abs(mainCenter-heroCenter) > 1 {
		t.Errorf("hero center y=%d, want free-area center y=%d", heroCenter, mainCenter)
	}
	if layout.HeroTop+heroHeight > layout.FooterTop-layout.HeroFooterGap {
		t.Errorf("hero bottom %d overlaps footer safe area beginning at %d", layout.HeroTop+heroHeight, layout.FooterTop-layout.HeroFooterGap)
	}
	if layout.FooterBottom != height-layout.FooterBottomMargin {
		t.Errorf("footer bottom=%d, want bottom safe margin %d", layout.FooterBottom, layout.FooterBottomMargin)
	}
	if layout.FooterTop != layout.FooterBottom-footerHeight {
		t.Errorf("footer top=%d, want %d", layout.FooterTop, layout.FooterBottom-footerHeight)
	}
	if layout.FooterWidth != ratioPixels(width, wallpaperFooterWidthRatio) {
		t.Errorf("footer width=%d, want configured width %d", layout.FooterWidth, ratioPixels(width, wallpaperFooterWidthRatio))
	}
}

func TestWallpaperTextLayoutUsesSafeCanvasCenterWithoutFooter(t *testing.T) {
	const width, height, heroHeight = 1080, 1920, 300
	layout := layoutWallpaperText(width, height, heroHeight, 0)
	if layout.FooterBottom != height-layout.FooterBottomMargin {
		t.Errorf("main area bottom=%d, want bottom safe margin %d", layout.FooterBottom, height-layout.FooterBottomMargin)
	}
	if layout.HeroTop+heroHeight > layout.FooterBottom {
		t.Errorf("hero bottom %d exceeds safe bottom %d", layout.HeroTop+heroHeight, layout.FooterBottom)
	}
	mainCenter := layout.MainTop + layout.MainHeight/2
	heroCenter := layout.HeroTop + heroHeight/2
	if abs(mainCenter-heroCenter) > 1 {
		t.Errorf("hero center y=%d, want safe-area center y=%d", heroCenter, mainCenter)
	}
}

func TestFooterTranslationWrapsWithinConfiguredWidth(t *testing.T) {
	if err := loadRenderFonts(); err != nil {
		t.Fatal(err)
	}
	block, err := renderBalancedTextBlock(textStyle{
		Text: "O you who have believed, when you contract a debt for a specified term, write it down. And let a scribe write it between you in justice.",
		Size: 18, Face: fontFaces.Translation, Direction: di.DirectionLTR, Script: language.Latin,
	}, 320)
	if err != nil {
		t.Fatal(err)
	}
	if block.LineCount < 2 {
		t.Fatalf("translation line count=%d, want wrapping", block.LineCount)
	}
	if block.AdvanceWidth > 320 {
		t.Errorf("translation advance width=%d, exceeds footer max width 320", block.AdvanceWidth)
	}
}

func TestWallpaperPipelineRendersMultilingualText(t *testing.T) {
	if err := loadRenderFonts(); err != nil {
		t.Fatal(err)
	}
	bengali := "পরম করুণাময়"
	if !utf8.ValidString(bengali) {
		t.Fatal("Bengali fixture is not valid UTF-8")
	}
	face := selectFontFace(bengali, fontFaces.Translation)
	if face == fontFaces.Translation || !faceCoversText(face, bengali) {
		t.Fatal("system font fallback did not select a face covering the Bengali text")
	}
	if got := textScript(bengali, language.Latin); got != language.LookupScript('প') {
		t.Fatalf("Bengali script = %s, want %s", got, language.LookupScript('প'))
	}
	directory := t.TempDir()
	basePath := filepath.Join(directory, "base.png")
	base := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	draw.Draw(base, base.Bounds(), image.NewUniform(color.RGBA{R: 20, G: 30, B: 28, A: 255}), image.Point{}, draw.Src)
	baseFile, err := os.Create(basePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(baseFile, base); err != nil {
		t.Fatal(err)
	}
	if err := baseFile.Close(); err != nil {
		t.Fatal(err)
	}
	runtime := Runtime{Config: Config{Home: directory}}
	cases := []struct {
		translation string
		words       []quran.Word
		wordByWord  bool
	}{
		{translation: bengali},
		{translation: "The Most Merciful"},
		{translation: "نہایت مہربان"},
		{translation: "بخشندهٔ مهربان"},
		{words: []quran.Word{
			{Arabic: "الرَّحْمَٰن", Translation: bengali},
			{Arabic: "الرَّحِيم", Translation: "The Most Merciful"},
		}, wordByWord: true},
	}
	for index, testCase := range cases {
		outputPath := filepath.Join(directory, string(rune('a'+index))+".png")
		ayah := quran.Ayah{Surah: 1, Number: 1, SurahName: "Al-Fatihah", Arabic: "الرَّحْمَٰن", Translation: testCase.translation, Words: testCase.words}
		options := RenderOptions{ShowTranslation: !testCase.wordByWord, ShowWordTranslation: testCase.wordByWord}
		if err := runtime.renderAyahText(ayah, basePath, outputPath, options); err != nil {
			t.Fatalf("render wallpaper case %d: %v", index, err)
		}
		file, err := os.Open(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := png.Decode(file)
		_ = file.Close()
		if err != nil {
			t.Fatalf("decode generated wallpaper: %v", err)
		}
		if decoded.Bounds().Dx() != 1920 || decoded.Bounds().Dy() != 1080 {
			t.Fatalf("generated wallpaper size = %v", decoded.Bounds())
		}
		changedPixels := 0
		for y := 0; y < decoded.Bounds().Dy() && changedPixels == 0; y++ {
			for x := 0; x < decoded.Bounds().Dx(); x++ {
				if decoded.At(x, y) != base.At(x, y) {
					changedPixels++
					break
				}
			}
		}
		if changedPixels == 0 {
			t.Fatalf("generated wallpaper case %d contains no rendered text", index)
		}
	}
	wordStyle := textStyleForText(textStyle{Face: fontFaces.Translation, Script: language.Latin}, bengali)
	fullTranslationStyle := textStyleForText(textStyle{Face: fontFaces.Translation, Script: language.Latin}, bengali+" এবং")
	if wordStyle.Face == fontFaces.Translation || !faceCoversText(wordStyle.Face, bengali) || wordStyle.Script != language.LookupScript('প') {
		t.Fatal("word-by-word Bengali meaning did not select a covering font and script")
	}
	if wordStyle.Face != fullTranslationStyle.Face {
		t.Fatal("Bengali word meanings and full translation selected different font faces")
	}
	for _, text := range []string{"نہایت مہربان", "بخشندهٔ مهربان"} {
		style := textStyleForText(textStyle{Face: fontFaces.Translation, Script: language.Latin}, text)
		if !faceCoversText(style.Face, text) || style.Script != language.Arabic || style.Direction != di.DirectionRTL {
			t.Fatalf("font selection failed for Arabic-script translation %q", text)
		}
	}
}

func TestBalancedTranslationWrapKeepsLinesWithinWidth(t *testing.T) {
	shape := func(text string) shaping.Output {
		width := 0
		for _, value := range text {
			if value == ' ' {
				width += 4
			} else {
				width += 10
			}
		}
		return shaping.Output{Advance: fixed.I(width)}
	}
	lines := wrapBalancedShapedText("alpha beta gamma delta epsilon zeta eta", 140, shape)
	if len(lines) < 2 {
		t.Fatalf("line count=%d, want wrapping", len(lines))
	}
	for _, line := range lines {
		if got := abs(shape(line).Advance.Round()); got > 140 {
			t.Errorf("line %q width=%d, exceeds max 140", line, got)
		}
	}
}
