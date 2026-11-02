package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fixtureServer struct {
	*httptest.Server

	mu            sync.Mutex
	verseRequests int
	audioRequests int
	perPage       string
	lastWordQuery string
}

func newFixtureServer(t *testing.T) *fixtureServer {
	t.Helper()
	fixtures := fixtureServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/resources/translations", func(w http.ResponseWriter, _ *http.Request) {
		writeFixture(t, w, "translations.json")
	})
	mux.HandleFunc("/resources/recitations", func(w http.ResponseWriter, _ *http.Request) {
		writeFixture(t, w, "recitations.json")
	})
	mux.HandleFunc("/chapters", func(w http.ResponseWriter, _ *http.Request) {
		writeFixture(t, w, "chapters.json")
	})
	mux.HandleFunc("/verses/by_chapter/1", func(w http.ResponseWriter, request *http.Request) {
		fixtures.mu.Lock()
		fixtures.verseRequests++
		fixtures.perPage = request.URL.Query().Get("per_page")
		fixtures.mu.Unlock()
		writeFixture(t, w, "surah_1.json")
	})
	mux.HandleFunc("/verses/by_chapter/2", func(w http.ResponseWriter, request *http.Request) {
		fixtures.mu.Lock()
		fixtures.verseRequests++
		fixtures.perPage = request.URL.Query().Get("per_page")
		fixtures.lastWordQuery = request.URL.Query().Get("words")
		fixtures.mu.Unlock()
		if request.URL.Query().Get("words") == "true" {
			writeFixture(t, w, "words_2.json")
			return
		}
		switch request.URL.Query().Get("page") {
		case "1", "":
			writeFixture(t, w, "surah_2_page_1.json")
		case "2":
			writeFixture(t, w, "surah_2_page_2.json")
		default:
			http.NotFound(w, request)
		}
	})
	mux.HandleFunc("/recitations/7/by_chapter/2", func(w http.ResponseWriter, request *http.Request) {
		fixtures.mu.Lock()
		fixtures.audioRequests++
		fixtures.mu.Unlock()
		writeFixture(t, w, "audio_2_255.json")
	})
	mux.HandleFunc("/recitations/7/by_ayah/2:255", func(w http.ResponseWriter, request *http.Request) {
		fixtures.mu.Lock()
		fixtures.audioRequests++
		fixtures.mu.Unlock()
		writeFixture(t, w, "audio_2_255.json")
	})
	fixtures.Server = httptest.NewServer(mux)
	t.Cleanup(fixtures.Close)
	return &fixtures
}

func writeFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Errorf("read fixture %s: %v", name, err)
		http.Error(w, "fixture missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func newFixtureSource(t *testing.T, server *fixtureServer) *quranCom {
	t.Helper()
	return newQuranCom(Options{
		CacheDir:     t.TempDir(),
		BaseURL:      server.URL,
		AudioBaseURL: quranComAudioBaseURL,
		HTTP:         &Client{HTTP: server.Client(), Attempts: 1, Delay: time.Millisecond},
	})
}

func TestQuranComSurahParsing(t *testing.T) {
	tests := []struct {
		name              string
		surah             int
		wantCount         int
		wantName          string
		wantFirstArabic   string
		wantFirstEnglish  string
		wantLastNumber    int
		wantVerseRequests int
	}{
		{
			name: "short surah", surah: 1, wantCount: 7, wantName: "Al-Fatihah",
			wantFirstArabic:  "بِسْمِ ٱللَّهِ ٱلرَّحْمَٰنِ ٱلرَّحِيمِ",
			wantFirstEnglish: "In the name of Allah, the Entirely Merciful, the Especially Merciful.",
			wantLastNumber:   7, wantVerseRequests: 1,
		},
		{
			name: "paginated long surah", surah: 2, wantCount: 5, wantName: "Al-Baqarah",
			wantFirstArabic: "الٓمٓ", wantFirstEnglish: "Alif, Lam, Meem.",
			wantLastNumber: 5, wantVerseRequests: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newFixtureServer(t)
			dataSource := newFixtureSource(t, server)
			ayahs, err := dataSource.Surah(context.Background(), test.surah, "en.sahih")
			if err != nil {
				t.Fatalf("Surah: %v", err)
			}
			if len(ayahs) != test.wantCount {
				t.Fatalf("ayah count = %d, want %d", len(ayahs), test.wantCount)
			}
			if ayahs[0].SurahName != test.wantName {
				t.Errorf("surah name = %q, want %q", ayahs[0].SurahName, test.wantName)
			}
			if ayahs[0].Arabic != test.wantFirstArabic {
				t.Errorf("first Arabic = %q, want %q", ayahs[0].Arabic, test.wantFirstArabic)
			}
			if ayahs[0].Translation != test.wantFirstEnglish {
				t.Errorf("first translation = %q, want %q", ayahs[0].Translation, test.wantFirstEnglish)
			}
			if ayahs[len(ayahs)-1].Number != test.wantLastNumber {
				t.Errorf("last ayah = %d, want %d", ayahs[len(ayahs)-1].Number, test.wantLastNumber)
			}
			server.mu.Lock()
			requests, perPage := server.verseRequests, server.perPage
			server.mu.Unlock()
			if requests != test.wantVerseRequests {
				t.Errorf("verse requests = %d, want %d", requests, test.wantVerseRequests)
			}
			if perPage != "50" {
				t.Errorf("per_page = %q, want 50", perPage)
			}
		})
	}
}

func TestQuranComSurahUsesDiskCache(t *testing.T) {
	server := newFixtureServer(t)
	dataSource := newFixtureSource(t, server)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := dataSource.Surah(context.Background(), 1, "en.sahih"); err != nil {
			t.Fatalf("Surah attempt %d: %v", attempt, err)
		}
	}
	server.mu.Lock()
	requests := server.verseRequests
	server.mu.Unlock()
	if requests != 1 {
		t.Fatalf("verse requests across two calls = %d, want 1", requests)
	}
}

func TestQuranComWords(t *testing.T) {
	server := newFixtureServer(t)
	dataSource := newFixtureSource(t, server)
	words, err := dataSource.Words(context.Background(), 2, 1)
	if err != nil {
		t.Fatalf("Words: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("word count = %d, want 2 (end marker must be skipped)", len(words))
	}
	if words[0].Arabic != "الٓ" || words[0].Transliteration != "Alif Lam" || words[0].Translation != "Alif, Lam" {
		t.Errorf("first word = %+v", words[0])
	}
	if _, err := dataSource.Words(context.Background(), 2, 2); err != nil {
		t.Fatalf("Words ayah 2: %v", err)
	}
	server.mu.Lock()
	requests := server.verseRequests
	query := server.lastWordQuery
	server.mu.Unlock()
	if requests != 1 || query != "true" {
		t.Fatalf("word requests = %d, words query = %q; want 1 and true", requests, query)
	}
}

func TestQuranComLegacyIDMapping(t *testing.T) {
	server := newFixtureServer(t)
	dataSource := newFixtureSource(t, server)
	ctx := context.Background()
	reciters, err := dataSource.Reciters(ctx)
	if err != nil {
		t.Fatalf("Reciters: %v", err)
	}
	translations, err := dataSource.Translations(ctx)
	if err != nil {
		t.Fatalf("Translations: %v", err)
	}
	reciterTests := []struct {
		identifier string
		wantID     string
	}{
		{identifier: "ar.alafasy", wantID: "7"},
		{identifier: "ar.husary", wantID: "6"},
		{identifier: "ar.abdulbasitmurattal", wantID: "10"},
		{identifier: "7", wantID: "7"},
		{identifier: "Mishary Rashid Alafasy", wantID: "7"},
	}
	for _, test := range reciterTests {
		resolved, resolveErr := resolveReciter(reciters, test.identifier)
		if resolveErr != nil {
			t.Errorf("resolveReciter(%q): %v", test.identifier, resolveErr)
			continue
		}
		if resolved.ID != test.wantID {
			t.Errorf("resolveReciter(%q) = %s, want %s", test.identifier, resolved.ID, test.wantID)
		}
	}
	if _, err := resolveReciter(reciters, "ar.unknown"); err == nil ||
		!strings.Contains(err.Error(), "valid legacy IDs") || !strings.Contains(err.Error(), "ar.alafasy") {
		t.Errorf("unknown reciter error = %v", err)
	}
	translationTests := []struct {
		identifier string
		wantID     string
	}{
		{identifier: "en.sahih", wantID: "20"},
		{identifier: "en.clearquran", wantID: "131"},
		{identifier: "ur.jalandhry", wantID: "85"},
		{identifier: "20", wantID: "20"},
	}
	for _, test := range translationTests {
		resolved, resolveErr := resolveTranslation(translations, test.identifier)
		if resolveErr != nil {
			t.Errorf("resolveTranslation(%q): %v", test.identifier, resolveErr)
			continue
		}
		if resolved.ID != test.wantID {
			t.Errorf("resolveTranslation(%q) = %s, want %s", test.identifier, resolved.ID, test.wantID)
		}
	}
	if _, err := resolveTranslation(translations, "xx.unknown"); err == nil ||
		!strings.Contains(err.Error(), "peace translations") {
		t.Errorf("unknown translation error = %v", err)
	}
}

func TestQuranComAudioURL(t *testing.T) {
	server := newFixtureServer(t)
	dataSource := newFixtureSource(t, server)
	url, err := dataSource.AudioURL("ar.alafasy", 2, 255)
	if err != nil {
		t.Fatalf("AudioURL: %v", err)
	}
	want := "https://verses.quran.com/Mishary_Rashid_al-Afasy_128/002255.mp3"
	if url != want {
		t.Errorf("AudioURL = %q, want %q", url, want)
	}
	if _, err := dataSource.AudioURL("ar.alafasy", 2, 255); err != nil {
		t.Fatalf("AudioURL cached: %v", err)
	}
	server.mu.Lock()
	requests := server.audioRequests
	server.mu.Unlock()
	if requests != 1 {
		t.Errorf("audio requests = %d, want 1", requests)
	}
}

func TestQuranComRetriesRateLimit(t *testing.T) {
	var requests int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		requests++
		current := requests
		mu.Unlock()
		if current == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeFixture(t, w, "translations.json")
	}))
	t.Cleanup(server.Close)
	dataSource := newQuranCom(Options{
		CacheDir: t.TempDir(), BaseURL: server.URL,
		HTTP: &Client{HTTP: server.Client(), Attempts: 3, Delay: time.Millisecond},
	})
	if _, err := dataSource.Translations(context.Background()); err != nil {
		t.Fatalf("Translations after retry: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if requests != 2 {
		t.Errorf("requests = %d, want 2", requests)
	}
}

func TestQuranComRejectsUnknownLegacyTranslationBeforeNetwork(t *testing.T) {
	server := newFixtureServer(t)
	dataSource := newFixtureSource(t, server)
	_, err := dataSource.Surah(context.Background(), 1, "xx.unknown")
	if err == nil || !strings.Contains(err.Error(), "peace translations") {
		t.Fatalf("expected unknown translation error, got %v", err)
	}
	server.mu.Lock()
	requests := server.verseRequests
	server.mu.Unlock()
	if requests != 0 {
		t.Errorf("verse requests = %d, want 0", requests)
	}
}
