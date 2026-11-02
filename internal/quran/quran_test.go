package quran

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

	mu                   sync.Mutex
	verseRequests        int
	audioChapterRequests int
	audioAyahRequests    int
	perPage              string
	wordQuery            string
	wordFields           string
}

func newFixtureServer(t *testing.T) *fixtureServer {
	t.Helper()
	server := &fixtureServer{}
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
		server.mu.Lock()
		server.verseRequests++
		server.perPage = request.URL.Query().Get("per_page")
		server.wordQuery = request.URL.Query().Get("words")
		server.wordFields = request.URL.Query().Get("word_fields")
		server.mu.Unlock()
		writeFixture(t, w, "surah_1.json")
	})
	mux.HandleFunc("/verses/by_chapter/2", func(w http.ResponseWriter, request *http.Request) {
		server.mu.Lock()
		server.verseRequests++
		server.perPage = request.URL.Query().Get("per_page")
		server.wordQuery = request.URL.Query().Get("words")
		server.wordFields = request.URL.Query().Get("word_fields")
		server.mu.Unlock()
		switch request.URL.Query().Get("page") {
		case "1", "":
			writeFixture(t, w, "surah_2_page_1.json")
		case "2":
			writeFixture(t, w, "surah_2_page_2.json")
		default:
			http.NotFound(w, request)
		}
	})
	mux.HandleFunc("/recitations/7/by_chapter/2", func(w http.ResponseWriter, _ *http.Request) {
		server.mu.Lock()
		server.audioChapterRequests++
		server.mu.Unlock()
		writeFixture(t, w, "audio_2_255.json")
	})
	mux.HandleFunc("/recitations/7/by_ayah/2:255", func(w http.ResponseWriter, _ *http.Request) {
		server.mu.Lock()
		server.audioAyahRequests++
		server.mu.Unlock()
		writeFixture(t, w, "audio_2_255.json")
	})
	server.Server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
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

func newFixtureClient(t *testing.T, server *fixtureServer) *Client {
	t.Helper()
	client := newClient(server.URL, t.TempDir(), server.Client())
	client.attempts = 1
	client.delay = time.Millisecond
	return client
}

func TestSurah(t *testing.T) {
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
			client := newFixtureClient(t, server)
			ayahs, err := client.Surah(context.Background(), test.surah, "en.sahih")
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

func TestSurahUsesDiskCache(t *testing.T) {
	server := newFixtureServer(t)
	client := newFixtureClient(t, server)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := client.Surah(context.Background(), 1, "en.sahih"); err != nil {
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

func TestWords(t *testing.T) {
	server := newFixtureServer(t)
	client := newFixtureClient(t, server)
	words, err := client.Words(context.Background(), 2, 1, "en.sahih")
	if err != nil {
		t.Fatalf("Words: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("word count = %d, want 2 (end marker must be skipped)", len(words))
	}
	if words[0].Arabic != "الٓ" || words[0].Transliteration != "Alif Lam" || words[0].Translation != "Alif, Lam" {
		t.Errorf("first word = %+v", words[0])
	}
	if _, err := client.Words(context.Background(), 2, 2, "en.sahih"); err != nil {
		t.Fatalf("Words ayah 2: %v", err)
	}
	server.mu.Lock()
	requests := server.verseRequests
	wordQuery := server.wordQuery
	wordFields := server.wordFields
	server.mu.Unlock()
	if requests != 2 {
		t.Fatalf("verse requests = %d, want 2 (paginated surah fetched once)", requests)
	}
	if wordQuery != "true" {
		t.Errorf("words query = %q, want true", wordQuery)
	}
	if wordFields != "text_uthmani,transliteration,translation" {
		t.Errorf("word_fields = %q", wordFields)
	}
}

func TestLegacyIDMigration(t *testing.T) {
	server := newFixtureServer(t)
	client := newFixtureClient(t, server)
	ctx := context.Background()
	reciters := []struct {
		identifier string
		wantID     string
	}{
		{identifier: "ar.alafasy", wantID: "7"},
		{identifier: "ar.husary", wantID: "6"},
		{identifier: "ar.abdulbasitmurattal", wantID: "10"},
		{identifier: "7", wantID: "7"},
		{identifier: "Mishary Rashid Alafasy", wantID: "7"},
	}
	for _, test := range reciters {
		resolved, err := client.Reciter(ctx, test.identifier)
		if err != nil {
			t.Errorf("Reciter(%q): %v", test.identifier, err)
			continue
		}
		if resolved.ID != test.wantID {
			t.Errorf("Reciter(%q) = %s, want %s", test.identifier, resolved.ID, test.wantID)
		}
	}
	if _, err := client.Reciter(ctx, "ar.unknown"); err == nil ||
		!strings.Contains(err.Error(), "valid options") || !strings.Contains(err.Error(), "peace reciters") {
		t.Errorf("unknown reciter error = %v", err)
	}
	translations := []struct {
		identifier string
		wantID     string
	}{
		{identifier: "en.sahih", wantID: "20"},
		{identifier: "en.clearquran", wantID: "131"},
		{identifier: "ur.jalandhry", wantID: "85"},
		{identifier: "20", wantID: "20"},
	}
	for _, test := range translations {
		resolved, err := client.Translation(ctx, test.identifier)
		if err != nil {
			t.Errorf("Translation(%q): %v", test.identifier, err)
			continue
		}
		if resolved.ID != test.wantID {
			t.Errorf("Translation(%q) = %s, want %s", test.identifier, resolved.ID, test.wantID)
		}
	}
	if _, err := client.Translation(ctx, "xx.unknown"); err == nil ||
		!strings.Contains(err.Error(), "peace translations") {
		t.Errorf("unknown translation error = %v", err)
	}
}

func TestAudioURLUsesChapterList(t *testing.T) {
	server := newFixtureServer(t)
	client := newFixtureClient(t, server)
	url, err := client.AudioURL(context.Background(), "ar.alafasy", 2, 255)
	if err != nil {
		t.Fatalf("AudioURL: %v", err)
	}
	want := "https://verses.quran.com/Mishary_Rashid_al-Afasy_128/002255.mp3"
	if url != want {
		t.Errorf("AudioURL = %q, want %q", url, want)
	}
	if _, err := client.AudioURL(context.Background(), "ar.alafasy", 2, 255); err != nil {
		t.Fatalf("AudioURL cached: %v", err)
	}
	server.mu.Lock()
	chapterRequests := server.audioChapterRequests
	ayahRequests := server.audioAyahRequests
	server.mu.Unlock()
	if chapterRequests != 1 || ayahRequests != 0 {
		t.Errorf("audio requests = chapter %d, ayah %d; want 1, 0", chapterRequests, ayahRequests)
	}
}

func TestAudioURLPaginatesChapterList(t *testing.T) {
	var mu sync.Mutex
	var pages []string
	var perPage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/recitations/7/by_chapter/2" {
			http.NotFound(w, request)
			return
		}
		page := request.URL.Query().Get("page")
		mu.Lock()
		pages = append(pages, page)
		perPage = request.URL.Query().Get("per_page")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			_, _ = w.Write([]byte(`{"audio_files":[{"verse_key":"2:1","url":"Alafasy/mp3/002001.mp3"},{"verse_key":"2:2","url":"Alafasy/mp3/002002.mp3"}],"pagination":{"per_page":50,"current_page":1,"next_page":2,"total_pages":2,"total_records":3}}`))
		case "2":
			_, _ = w.Write([]byte(`{"audio_files":[{"verse_key":"2:255","url":"Alafasy/mp3/002255.mp3"}],"pagination":{"per_page":50,"current_page":2,"next_page":null,"total_pages":2,"total_records":3}}`))
		default:
			http.NotFound(w, request)
		}
	}))
	t.Cleanup(server.Close)
	client := newClient(server.URL, t.TempDir(), server.Client())
	client.attempts = 1
	client.delay = time.Millisecond
	client.storeReciters([]Reciter{{ID: "7", Name: "Mishary Rashid Alafasy", Slug: "mishary-rashid-alafasy", Language: "arabic"}})

	url, err := client.AudioURL(context.Background(), "7", 2, 1)
	if err != nil {
		t.Fatalf("AudioURL 2:1: %v", err)
	}
	if url != "https://verses.quran.com/Alafasy/mp3/002001.mp3" {
		t.Errorf("AudioURL 2:1 = %q", url)
	}
	url, err = client.AudioURL(context.Background(), "7", 2, 255)
	if err != nil {
		t.Fatalf("AudioURL 2:255: %v", err)
	}
	if url != "https://verses.quran.com/Alafasy/mp3/002255.mp3" {
		t.Errorf("AudioURL 2:255 = %q", url)
	}
	mu.Lock()
	gotPages := append([]string(nil), pages...)
	gotPerPage := perPage
	mu.Unlock()
	if len(gotPages) != 2 || gotPages[0] != "1" || gotPages[1] != "2" {
		t.Errorf("chapter pages = %v, want [1 2]", gotPages)
	}
	if gotPerPage != "50" {
		t.Errorf("per_page = %q, want 50", gotPerPage)
	}
}

func TestAbsoluteAudioURL(t *testing.T) {
	tests := []struct {
		name     string
		relative string
		want     string
	}{
		{name: "path", relative: "Alafasy/mp3/002001.mp3", want: "https://verses.quran.com/Alafasy/mp3/002001.mp3"},
		{name: "leading slash", relative: "/Alafasy/mp3/002001.mp3", want: "https://verses.quran.com/Alafasy/mp3/002001.mp3"},
		{name: "host without scheme", relative: "mirrors.quranicaudio.com/everyayah/Husary_64kbps/002001.mp3", want: "https://mirrors.quranicaudio.com/everyayah/Husary_64kbps/002001.mp3"},
		{name: "absolute", relative: "https://example.com/audio.mp3", want: "https://example.com/audio.mp3"},
		{name: "empty", relative: "", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := absoluteAudioURL(test.relative); got != test.want {
				t.Errorf("absoluteAudioURL(%q) = %q, want %q", test.relative, got, test.want)
			}
		})
	}
}

func TestReciterNamesIncludeStyle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != pathRecitations {
			http.NotFound(w, request)
			return
		}
		_, _ = w.Write([]byte(`{"recitations":[{"id":1,"reciter_name":"AbdulBaset AbdulSamad","style":"Mujawwad","translated_name":{"name":"AbdulBaset AbdulSamad"}},{"id":12,"reciter_name":"Mahmoud Khalil Al-Husary","style":"Muallim","translated_name":{"name":"Mahmoud Khalil Al-Husary (Muallim)"}}]}`))
	}))
	t.Cleanup(server.Close)
	client := newClient(server.URL, t.TempDir(), server.Client())
	client.attempts = 1
	reciters, err := client.Reciters(context.Background())
	if err != nil {
		t.Fatalf("Reciters: %v", err)
	}
	if len(reciters) != 2 {
		t.Fatalf("reciter count = %d, want 2", len(reciters))
	}
	if reciters[0].Name != "AbdulBaset AbdulSamad (Mujawwad)" {
		t.Errorf("reciter 1 name = %q", reciters[0].Name)
	}
	if reciters[1].Name != "Mahmoud Khalil Al-Husary (Muallim)" {
		t.Errorf("reciter 12 name = %q", reciters[1].Name)
	}
}

func TestAudioURLFallsBackToAyahEndpoint(t *testing.T) {
	var ayahRequests, chapterRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/recitations/7/by_chapter/") {
			chapterRequests++
			http.NotFound(w, request)
			return
		}
		switch request.URL.Path {
		case "/recitations/7/by_ayah/2:1":
			ayahRequests++
			_, _ = w.Write([]byte(`{"audio_files":[{"verse_key":"2:1","url":"Alafasy/mp3/002001.mp3"}]}`))
		case "/recitations/7/by_ayah/2:255":
			ayahRequests++
			writeFixture(t, w, "audio_2_255.json")
		default:
			http.NotFound(w, request)
		}
	}))
	t.Cleanup(server.Close)
	client := newClient(server.URL, t.TempDir(), server.Client())
	client.attempts = 1
	client.delay = time.Millisecond
	client.storeReciters([]Reciter{{ID: "7", Name: "Mishary Rashid Alafasy", Slug: "mishary-rashid-alafasy", Language: "arabic"}})

	if _, err := client.AudioURL(context.Background(), "7", 2, 1); err != nil {
		t.Fatalf("AudioURL 2:1: %v", err)
	}
	url, err := client.AudioURL(context.Background(), "7", 2, 255)
	if err != nil {
		t.Fatalf("AudioURL 2:255: %v", err)
	}
	if url != "https://verses.quran.com/Mishary_Rashid_al-Afasy_128/002255.mp3" {
		t.Errorf("AudioURL = %q", url)
	}
	if chapterRequests != 1 {
		t.Errorf("chapter requests = %d, want 1", chapterRequests)
	}
	if ayahRequests != 2 {
		t.Errorf("ayah requests = %d, want 2", ayahRequests)
	}
}

func TestRetriesRateLimit(t *testing.T) {
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
	client := newClient(server.URL, t.TempDir(), server.Client())
	client.attempts = 3
	client.delay = time.Millisecond
	if _, err := client.Translations(context.Background()); err != nil {
		t.Fatalf("Translations after retry: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if requests != 2 {
		t.Errorf("requests = %d, want 2", requests)
	}
}

func TestUnknownTranslationStopsBeforeVerseRequest(t *testing.T) {
	server := newFixtureServer(t)
	client := newFixtureClient(t, server)
	_, err := client.Surah(context.Background(), 1, "xx.unknown")
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

func TestAPIFailureIsReported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	client := newClient(server.URL, t.TempDir(), server.Client())
	client.attempts = 1
	client.delay = time.Millisecond
	_, err := client.Translations(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("expected HTTP 502 error, got %v", err)
	}
}

func TestAPIBaseURLFromEnvironment(t *testing.T) {
	t.Setenv(baseURLEnv, " http://localhost:9999/api/ ")
	if got := apiBaseURL(); got != "http://localhost:9999/api" {
		t.Errorf("apiBaseURL() = %q", got)
	}
}

func TestDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("audio"))
	}))
	t.Cleanup(server.Close)
	client := newClient(server.URL, t.TempDir(), server.Client())
	client.attempts = 1
	destination := filepath.Join(t.TempDir(), "ayah.mp3")
	if err := client.Download(context.Background(), server.URL+"/audio.mp3", destination); err != nil {
		t.Fatalf("Download: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if string(data) != "audio" {
		t.Errorf("downloaded %q", data)
	}
}
