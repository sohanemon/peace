package source

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeLibrary struct {
	surahResult        []Ayah
	surahErr           error
	audioResult        string
	audioErr           error
	wordsResult        []Word
	wordsErr           error
	chaptersResult     []Chapter
	chaptersErr        error
	recitersResult     []Reciter
	recitersErr        error
	translationsResult []Translation
	translationsErr    error
}

func (f fakeLibrary) Surah(context.Context, int, string) ([]Ayah, error) {
	return f.surahResult, f.surahErr
}

func (f fakeLibrary) AudioURL(string, int, int) (string, error) {
	return f.audioResult, f.audioErr
}

func (f fakeLibrary) Words(context.Context, int, int) ([]Word, error) {
	return f.wordsResult, f.wordsErr
}

func (f fakeLibrary) Chapters(context.Context) ([]Chapter, error) {
	return f.chaptersResult, f.chaptersErr
}

func (f fakeLibrary) Reciters(context.Context) ([]Reciter, error) {
	return f.recitersResult, f.recitersErr
}

func (f fakeLibrary) Translations(context.Context) ([]Translation, error) {
	return f.translationsResult, f.translationsErr
}

func TestFallbackLibraryUsesSecondaryOnFailure(t *testing.T) {
	primaryErr := errors.New("primary down")
	primary := fakeLibrary{surahErr: primaryErr, audioErr: primaryErr, wordsErr: primaryErr, chaptersErr: primaryErr, recitersErr: primaryErr, translationsErr: primaryErr}
	secondary := fakeLibrary{
		surahResult:        []Ayah{{Surah: 1, Number: 1}},
		audioResult:        "https://example.test/audio.mp3",
		wordsResult:        []Word{{Arabic: "a"}},
		chaptersResult:     []Chapter{{Number: 1, Verses: 7}},
		recitersResult:     []Reciter{{ID: "1"}},
		translationsResult: []Translation{{ID: "20"}},
	}
	dataSource := newFallbackLibrary(primary, secondary)
	ctx := context.Background()
	if ayahs, err := dataSource.Surah(ctx, 1, "en.sahih"); err != nil || len(ayahs) != 1 {
		t.Errorf("Surah = %v, %v", ayahs, err)
	}
	if url, err := dataSource.AudioURL("ar.alafasy", 1, 1); err != nil || url != secondary.audioResult {
		t.Errorf("AudioURL = %q, %v", url, err)
	}
	if words, err := dataSource.Words(ctx, 1, 1); err != nil || len(words) != 1 {
		t.Errorf("Words = %v, %v", words, err)
	}
	if chapters, err := dataSource.Chapters(ctx); err != nil || len(chapters) != 1 {
		t.Errorf("Chapters = %v, %v", chapters, err)
	}
	if reciters, err := dataSource.Reciters(ctx); err != nil || len(reciters) != 1 {
		t.Errorf("Reciters = %v, %v", reciters, err)
	}
	if translations, err := dataSource.Translations(ctx); err != nil || len(translations) != 1 {
		t.Errorf("Translations = %v, %v", translations, err)
	}
}

func TestFallbackLibraryReportsBothFailures(t *testing.T) {
	primary := fakeLibrary{surahErr: errors.New("primary down")}
	secondary := fakeLibrary{surahErr: errors.New("fallback down")}
	_, err := newFallbackLibrary(primary, secondary).Surah(context.Background(), 1, "en.sahih")
	if err == nil || !strings.Contains(err.Error(), "primary source failed") || !strings.Contains(err.Error(), "fallback source failed") {
		t.Fatalf("combined error = %v", err)
	}
}

func TestNewRejectsUnknownFallback(t *testing.T) {
	_, err := New("quran", Options{CacheDir: t.TempDir(), Fallback: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown source") {
		t.Fatalf("expected unknown fallback error, got %v", err)
	}
}
