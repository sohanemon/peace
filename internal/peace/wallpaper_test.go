package peace

import (
	"testing"

	"io.github.sohanemon/peace/internal/quran"
)

func TestChapterForAbsoluteMapsAcrossChapters(t *testing.T) {
	chapters := []quran.Chapter{
		{Number: 1, Verses: 7},
		{Number: 2, Name: "Al-Baqarah", Verses: 286},
	}
	tests := []struct {
		name      string
		number    int
		wantSurah int
		wantAyah  int
		wantErr   bool
	}{
		{name: "first ayah", number: 1, wantSurah: 1, wantAyah: 1},
		{name: "first ayah of second surah", number: 8, wantSurah: 2, wantAyah: 1},
		{name: "ayah 255 of second surah", number: 262, wantSurah: 2, wantAyah: 255},
		{name: "out of range", number: 7000, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			surah, ayah, err := chapterForAbsolute(chapters, test.number)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error for ayah %d", test.number)
				}
				return
			}
			if err != nil {
				t.Fatalf("chapterForAbsolute: %v", err)
			}
			if surah != test.wantSurah || ayah != test.wantAyah {
				t.Errorf("mapped to %d:%d, want %d:%d", surah, ayah, test.wantSurah, test.wantAyah)
			}
		})
	}
}

func TestAyahReference(t *testing.T) {
	if got := ayahReference(quran.Ayah{Surah: 2, Number: 255, SurahName: "Al-Baqarah"}); got != "Al-Baqarah 2:255" {
		t.Errorf("reference = %q", got)
	}
	if got := ayahReference(quran.Ayah{Surah: 2, Number: 255}); got != "2:255" {
		t.Errorf("reference without name = %q", got)
	}
}
