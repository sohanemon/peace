package peace

import (
	"reflect"
	"testing"
)

func TestNeedsIntroductoryBasmala(t *testing.T) {
	for _, test := range []struct {
		surah int
		start int
		want  bool
	}{
		{surah: 2, start: 1, want: true},
		{surah: 8, start: 1, want: true},
		{surah: 10, start: 1, want: true},
		{surah: 114, start: 1, want: true},
		{surah: 2, start: 2},
		{surah: 1, start: 1},
		{surah: 9, start: 1},
	} {
		if got := needsIntroductoryBasmala(test.surah, test.start); got != test.want {
			t.Errorf("needsIntroductoryBasmala(%d, %d) = %t, want %t", test.surah, test.start, got, test.want)
		}
	}
}

func TestShouldUseBasmala(t *testing.T) {
	for _, test := range []struct {
		enabled bool
		surah   int
		start   int
		want    bool
	}{
		{enabled: true, surah: 2, start: 1, want: true},
		{enabled: false, surah: 2, start: 1},
		{enabled: true, surah: 2, start: 2},
		{enabled: true, surah: 1, start: 1},
		{enabled: true, surah: 9, start: 1},
	} {
		if got := shouldUseBasmala(test.enabled, test.surah, test.start); got != test.want {
			t.Errorf("shouldUseBasmala(%t, %d, %d) = %t, want %t", test.enabled, test.surah, test.start, got, test.want)
		}
	}
}

func TestDisplayAyahArabicUsesBasmalaFlag(t *testing.T) {
	text := "بِسْمِ اللَّهِ الرَّحْمَٰنِ الرَّحِيمِ الم"
	if got := displayAyahArabic(2, 1, text, false); got != "الم" {
		t.Errorf("hidden Basmala text = %q, want %q", got, "الم")
	}
	if got := displayAyahArabic(2, 1, text, true); got != text {
		t.Errorf("shown Basmala text = %q, want original", got)
	}
	if got := displayAyahArabic(1, 1, text, false); got != text {
		t.Errorf("Fatiha Basmala should remain as ayah 1, got %q", got)
	}
}

func TestPlaybackAudioURLsPutsBasmalaBeforeAyahs(t *testing.T) {
	got := playbackAudioURLs([]string{"ayah-1", "ayah-2"}, "selected-reciter-basmala")
	want := []string{"selected-reciter-basmala", "ayah-1", "ayah-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("playbackAudioURLs() = %v, want %v", got, want)
	}
	if got := playbackAudioURLs([]string{"ayah-1"}, ""); !reflect.DeepEqual(got, []string{"ayah-1"}) {
		t.Fatalf("playbackAudioURLs() without intro = %v", got)
	}
}

func TestFollowTrackAyahDoesNotCountBasmalaAsAyah(t *testing.T) {
	state := PlayState{Surah: 2, Start: 1, IntroAudioURL: "basmala"}
	if ayah, ok := followTrackAyah("Introductory Basmala", 0, true, state); ok {
		t.Fatalf("Basmala mapped to ayah %d", ayah)
	}
	if ayah, ok := followTrackAyah("", 0, true, state); ok {
		t.Fatalf("intro playlist position mapped to ayah %d", ayah)
	}
	if ayah, ok := followTrackAyah("", 1, true, state); !ok || ayah != 1 {
		t.Fatalf("first ayah position = %d, %t; want 1, true", ayah, ok)
	}
	if ayah, ok := followTrackAyah("2:1", 1, true, state); !ok || ayah != 1 {
		t.Fatalf("titled first ayah = %d, %t; want 1, true", ayah, ok)
	}
	if ayah, ok := followTrackAyah("", 2, true, state); !ok || ayah != 2 {
		t.Fatalf("second ayah position = %d, %t; want 2, true", ayah, ok)
	}
}
