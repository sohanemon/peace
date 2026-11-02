package peace

import (
	"context"
	"testing"

	"io.github.sohanemon/peace/internal/source"
)

type mappingLibrary struct {
	chapters []source.Chapter
	ayahs    map[int][]source.Ayah
}

func (m mappingLibrary) Surah(_ context.Context, surah int, _ string) ([]source.Ayah, error) {
	return m.ayahs[surah], nil
}

func (m mappingLibrary) AudioURL(string, int, int) (string, error) { return "", nil }
func (m mappingLibrary) Words(context.Context, int, int) ([]source.Word, error) {
	return nil, nil
}
func (m mappingLibrary) Chapters(context.Context) ([]source.Chapter, error) {
	return m.chapters, nil
}
func (m mappingLibrary) Reciters(context.Context) ([]source.Reciter, error) { return nil, nil }
func (m mappingLibrary) Translations(context.Context) ([]source.Translation, error) {
	return nil, nil
}

func TestFetchAyahByAbsoluteMapsAcrossChapters(t *testing.T) {
	library := mappingLibrary{
		chapters: []source.Chapter{{Number: 1, Verses: 7}, {Number: 2, Name: "Al-Baqarah", Verses: 286}},
		ayahs: map[int][]source.Ayah{
			1: {{Surah: 1, SurahName: "Al-Fatihah", Number: 1, Arabic: "a", Translation: "t"}},
			2: {{Surah: 2, SurahName: "Al-Baqarah", Number: 255, Arabic: "arabic", Translation: "translation"}},
		},
	}
	surah, ayah, text, err := fetchAyahByAbsolute(context.Background(), library, 262, "en.sahih")
	if err != nil {
		t.Fatalf("fetchAyahByAbsolute: %v", err)
	}
	if surah != 2 || ayah != 255 {
		t.Errorf("mapped to %d:%d, want 2:255", surah, ayah)
	}
	if text.Arabic != "arabic" || text.English != "translation" || text.Reference != "Al-Baqarah 2:255" {
		t.Errorf("text = %+v", text)
	}
	if _, _, _, err := fetchAyahByAbsolute(context.Background(), library, 7000, "en.sahih"); err == nil {
		t.Error("expected out-of-range error")
	}
}
