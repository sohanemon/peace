// Package quran is the only data layer for peace. It owns every Quran.com API
// request, URL, cache file, and the shared Ayah/Word types used by the CLI.
package quran

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	// DefaultReciterID is Mishary Rashid Alafasy.
	DefaultReciterID = "7"
	// DefaultTranslationID is Saheeh International.
	DefaultTranslationID = "20"

	perPage = 50
)

var (
	supPattern   = regexp.MustCompile(`(?s)<sup[^>]*>.*?</sup>`)
	htmlPattern  = regexp.MustCompile(`<[^>]*>`)
	spacePattern = regexp.MustCompile(`\s+`)
)

// Ayah is one verse with its Arabic text, translation, and word data.
type Ayah struct {
	Surah       int
	SurahName   string
	Number      int
	Arabic      string
	Translation string
	Words       []Word
}

// Word is one word-by-word entry for an ayah.
type Word struct {
	Arabic          string
	Transliteration string
	Translation     string
}

// Reciter describes an available recitation resource.
type Reciter struct {
	ID       string
	Name     string
	Slug     string
	Language string
}

// Translation describes an available translation resource.
type Translation struct {
	ID       string
	Name     string
	Slug     string
	Language string
}

// Chapter describes a surah and its ayah count.
type Chapter struct {
	Number      int
	Name        string
	EnglishName string
	Verses      int
}

// Client talks to the Quran.com API and caches the responses it reads.
type Client struct {
	baseURL  string
	cacheDir string
	http     *http.Client
	attempts int
	delay    time.Duration

	mu           sync.Mutex
	reciters     []Reciter
	translations []Translation
	chapters     []Chapter
	audioURLs    map[string]string
	chapterAudio map[string]map[int]string
	chapterFail  map[string]bool
}

// New returns a client that writes its cache under cacheDir.
func New(cacheDir string) *Client {
	return newClient(apiBaseURL(), cacheDir, nil)
}

func newClient(baseURL, cacheDir string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		cacheDir:     cacheDir,
		http:         httpClient,
		attempts:     maxAttempts,
		delay:        retryBaseDelay,
		audioURLs:    map[string]string{},
		chapterAudio: map[string]map[int]string{},
		chapterFail:  map[string]bool{},
	}
}

type word struct {
	CharTypeName    string `json:"char_type_name"`
	TextUthmani     string `json:"text_uthmani"`
	Text            string `json:"text"`
	Transliteration struct {
		Text string `json:"text"`
	} `json:"transliteration"`
	Translation struct {
		Text string `json:"text"`
	} `json:"translation"`
}

type verse struct {
	VerseKey     string `json:"verse_key"`
	TextUthmani  string `json:"text_uthmani"`
	Translations []struct {
		Text string `json:"text"`
	} `json:"translations"`
	Words []word `json:"words"`
}

type versePage struct {
	Verses     []verse `json:"verses"`
	Pagination struct {
		NextPage *int `json:"next_page"`
	} `json:"pagination"`
}

type audioFile struct {
	VerseKey string `json:"verse_key"`
	URL      string `json:"url"`
}

type audioResponse struct {
	AudioFiles []audioFile `json:"audio_files"`
	AudioFile  audioFile   `json:"audio_file"`
}

type audioPage struct {
	AudioFiles []audioFile `json:"audio_files"`
	Pagination struct {
		NextPage *int `json:"next_page"`
	} `json:"pagination"`
}

// Surah returns every ayah of one surah, including translation and words.
// The result is cached as one JSON file per surah and translation.
func (c *Client) Surah(ctx context.Context, surah int, translationID string) ([]Ayah, error) {
	if err := validateSurah(surah); err != nil {
		return nil, err
	}
	translation, err := c.Translation(ctx, translationID)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("surah-%d-%s", surah, translation.ID)
	var cached []Ayah
	if loadCache(c.cacheDir, key, surahCacheTTL, &cached) && len(cached) > 0 {
		return cached, nil
	}
	verses, err := c.fetchVerses(ctx, surah, translation.ID)
	if err != nil {
		return nil, err
	}
	surahName := c.chapterName(ctx, surah)
	ayahs := make([]Ayah, 0, len(verses))
	for _, entry := range verses {
		number := verseNumber(entry.VerseKey, surah)
		if number == 0 {
			continue
		}
		translated := ""
		if len(entry.Translations) > 0 {
			translated = cleanTranslation(entry.Translations[0].Text)
		}
		ayahs = append(ayahs, Ayah{
			Surah:       surah,
			SurahName:   surahName,
			Number:      number,
			Arabic:      strings.TrimSpace(entry.TextUthmani),
			Translation: translated,
			Words:       parseWords(entry.Words),
		})
	}
	if len(ayahs) == 0 {
		return nil, errors.New("quran.com API: empty surah response")
	}
	if err := saveCache(c.cacheDir, key, ayahs); err != nil {
		return nil, err
	}
	return ayahs, nil
}

// Ayah returns one verse. It reuses the cached surah response.
func (c *Client) Ayah(ctx context.Context, surah, ayah int, translationID string) (Ayah, error) {
	if err := validateSurah(surah); err != nil {
		return Ayah{}, err
	}
	if err := validateAyah(ayah); err != nil {
		return Ayah{}, err
	}
	ayahs, err := c.Surah(ctx, surah, translationID)
	if err != nil {
		return Ayah{}, err
	}
	for _, entry := range ayahs {
		if entry.Number == ayah {
			return entry, nil
		}
	}
	return Ayah{}, fmt.Errorf("ayah %d:%d is unavailable", surah, ayah)
}

// Words returns word-by-word data for one ayah.
func (c *Client) Words(ctx context.Context, surah, ayah int, translationID string) ([]Word, error) {
	entry, err := c.Ayah(ctx, surah, ayah, translationID)
	if err != nil {
		return nil, err
	}
	if len(entry.Words) == 0 {
		return nil, fmt.Errorf("quran.com API: no word-by-word data for %d:%d", surah, ayah)
	}
	return entry.Words, nil
}

func (c *Client) fetchVerses(ctx context.Context, surah int, translationID string) ([]verse, error) {
	query := queryValues(
		"fields", "text_uthmani",
		"translations", translationID,
		"words", "true",
		"word_fields", "text_uthmani,transliteration",
		"per_page", strconv.Itoa(perPage),
	)
	return paginate(ctx, func(ctx context.Context, page int) ([]verse, *int, error) {
		query.Set("page", strconv.Itoa(page))
		var response versePage
		if err := c.getJSON(ctx, versesPath(surah), query, &response); err != nil {
			return nil, nil, err
		}
		return response.Verses, response.Pagination.NextPage, nil
	})
}

// AudioURL resolves the absolute audio URL for one ayah.
func (c *Client) AudioURL(ctx context.Context, reciter string, surah, ayah int) (string, error) {
	if err := validateSurah(surah); err != nil {
		return "", err
	}
	if err := validateAyah(ayah); err != nil {
		return "", err
	}
	resolved, err := c.Reciter(ctx, reciter)
	if err != nil {
		return "", err
	}
	recitationID, err := strconv.Atoi(resolved.ID)
	if err != nil {
		return "", fmt.Errorf("quran.com API: invalid recitation ID %q", resolved.ID)
	}
	key := fmt.Sprintf("%s/%d:%d", resolved.ID, surah, ayah)
	chapterKey := fmt.Sprintf("%s/%d", resolved.ID, surah)
	c.mu.Lock()
	cached := c.audioURLs[key]
	chapter := c.chapterAudio[chapterKey]
	chapterFailed := c.chapterFail[chapterKey]
	c.mu.Unlock()
	if cached != "" {
		return cached, nil
	}
	if chapter != nil && chapter[ayah] != "" {
		return chapter[ayah], nil
	}
	// NOTE: The chapter endpoint is paginated (10 files by default), so collect
	// every page once instead of requesting one URL per ayah. Fall back to
	// by_ayah only when the chapter list is unavailable.
	if !chapterFailed {
		audio, err := c.fetchChapterAudio(ctx, recitationID, surah)
		if err == nil && len(audio) > 0 {
			c.mu.Lock()
			c.chapterAudio[chapterKey] = audio
			for number, url := range audio {
				c.audioURLs[fmt.Sprintf("%s/%d:%d", resolved.ID, surah, number)] = url
			}
			c.mu.Unlock()
			if url := audio[ayah]; url != "" {
				return url, nil
			}
		} else {
			c.mu.Lock()
			c.chapterFail[chapterKey] = true
			c.mu.Unlock()
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
		}
	}
	var response audioResponse
	if err := c.getJSON(ctx, ayahAudioPath(recitationID, surah, ayah), nil, &response); err != nil {
		return "", err
	}
	relative := ""
	if len(response.AudioFiles) > 0 {
		relative = response.AudioFiles[0].URL
	}
	if relative == "" {
		relative = response.AudioFile.URL
	}
	if relative == "" {
		return "", fmt.Errorf("quran.com API: audio URL missing for %d:%d", surah, ayah)
	}
	absolute := absoluteAudioURL(relative)
	c.mu.Lock()
	c.audioURLs[key] = absolute
	c.mu.Unlock()
	return absolute, nil
}

func (c *Client) fetchChapterAudio(ctx context.Context, recitationID, surah int) (map[int]string, error) {
	query := queryValues("per_page", strconv.Itoa(perPage))
	files, err := paginate(ctx, func(ctx context.Context, page int) ([]audioFile, *int, error) {
		query.Set("page", strconv.Itoa(page))
		var response audioPage
		if err := c.getJSON(ctx, chapterAudioPath(recitationID, surah), query, &response); err != nil {
			return nil, nil, err
		}
		return response.AudioFiles, response.Pagination.NextPage, nil
	})
	if err != nil {
		return nil, err
	}
	audio := make(map[int]string, len(files))
	for _, file := range files {
		number := verseNumber(file.VerseKey, surah)
		if number == 0 || file.URL == "" {
			continue
		}
		audio[number] = absoluteAudioURL(file.URL)
	}
	return audio, nil
}

// Chapters returns the 114 surahs and their ayah counts.
func (c *Client) Chapters(ctx context.Context) ([]Chapter, error) {
	c.mu.Lock()
	if len(c.chapters) == 114 {
		chapters := append([]Chapter(nil), c.chapters...)
		c.mu.Unlock()
		return chapters, nil
	}
	c.mu.Unlock()
	var cached []Chapter
	if loadCache(c.cacheDir, "chapters", resourceCacheTTL, &cached) && len(cached) == 114 {
		c.storeChapters(cached)
		return cached, nil
	}
	var response struct {
		Chapters []struct {
			ID             int    `json:"id"`
			NameSimple     string `json:"name_simple"`
			VersesCount    int    `json:"verses_count"`
			TranslatedName struct {
				Name string `json:"name"`
			} `json:"translated_name"`
		} `json:"chapters"`
	}
	if err := c.getJSON(ctx, chaptersPath(), queryValues("language", "en"), &response); err != nil {
		return nil, err
	}
	chapters := make([]Chapter, 0, len(response.Chapters))
	for _, chapter := range response.Chapters {
		chapters = append(chapters, Chapter{
			Number:      chapter.ID,
			Name:        chapter.NameSimple,
			EnglishName: chapter.TranslatedName.Name,
			Verses:      chapter.VersesCount,
		})
	}
	if len(chapters) != 114 {
		return nil, fmt.Errorf("quran.com API: incomplete chapter list (%d)", len(chapters))
	}
	if err := saveCache(c.cacheDir, "chapters", chapters); err != nil {
		return nil, err
	}
	c.storeChapters(chapters)
	return chapters, nil
}

// Reciters returns every available recitation resource.
func (c *Client) Reciters(ctx context.Context) ([]Reciter, error) {
	c.mu.Lock()
	if len(c.reciters) > 0 {
		reciters := append([]Reciter(nil), c.reciters...)
		c.mu.Unlock()
		return reciters, nil
	}
	c.mu.Unlock()
	var cached []Reciter
	if loadCache(c.cacheDir, recitersCacheKey, resourceCacheTTL, &cached) && len(cached) > 0 {
		c.storeReciters(cached)
		return cached, nil
	}
	var response struct {
		Recitations []struct {
			ID             int     `json:"id"`
			ReciterName    string  `json:"reciter_name"`
			Style          *string `json:"style"`
			TranslatedName struct {
				Name string `json:"name"`
			} `json:"translated_name"`
		} `json:"recitations"`
	}
	if err := c.getJSON(ctx, pathRecitations, nil, &response); err != nil {
		return nil, err
	}
	reciters := make([]Reciter, 0, len(response.Recitations))
	for _, recitation := range response.Recitations {
		name := recitation.TranslatedName.Name
		if name == "" {
			name = recitation.ReciterName
		}
		style := ""
		if recitation.Style != nil {
			style = strings.TrimSpace(*recitation.Style)
		}
		if style != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(style)) {
			name += " (" + style + ")"
		}
		slugSource := recitation.ReciterName
		if slugSource == "" {
			slugSource = name
		}
		if style != "" {
			slugSource += " " + style
		}
		reciters = append(reciters, Reciter{
			ID:       strconv.Itoa(recitation.ID),
			Name:     name,
			Slug:     slugify(slugSource),
			Language: "arabic",
		})
	}
	if len(reciters) == 0 {
		return nil, errors.New("quran.com API: empty recitation list")
	}
	sort.SliceStable(reciters, func(first, second int) bool {
		left, _ := strconv.Atoi(reciters[first].ID)
		right, _ := strconv.Atoi(reciters[second].ID)
		return left < right
	})
	if err := saveCache(c.cacheDir, recitersCacheKey, reciters); err != nil {
		return nil, err
	}
	c.storeReciters(reciters)
	return append([]Reciter(nil), reciters...), nil
}

// Translations returns every available translation resource.
func (c *Client) Translations(ctx context.Context) ([]Translation, error) {
	c.mu.Lock()
	if len(c.translations) > 0 {
		translations := append([]Translation(nil), c.translations...)
		c.mu.Unlock()
		return translations, nil
	}
	c.mu.Unlock()
	var cached []Translation
	if loadCache(c.cacheDir, "translations", resourceCacheTTL, &cached) && len(cached) > 0 {
		c.storeTranslations(cached)
		return cached, nil
	}
	var response struct {
		Translations []struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			Slug         string `json:"slug"`
			LanguageName string `json:"language_name"`
			LanguageCode string `json:"language_code"`
		} `json:"translations"`
	}
	if err := c.getJSON(ctx, pathTranslations, nil, &response); err != nil {
		return nil, err
	}
	translations := make([]Translation, 0, len(response.Translations))
	for _, entry := range response.Translations {
		language := entry.LanguageName
		if language == "" {
			language = entry.LanguageCode
		}
		translations = append(translations, Translation{
			ID:       strconv.Itoa(entry.ID),
			Name:     entry.Name,
			Slug:     entry.Slug,
			Language: language,
		})
	}
	if len(translations) == 0 {
		return nil, errors.New("quran.com API: empty translation list")
	}
	sort.SliceStable(translations, func(first, second int) bool {
		left, _ := strconv.Atoi(translations[first].ID)
		right, _ := strconv.Atoi(translations[second].ID)
		return left < right
	})
	if err := saveCache(c.cacheDir, "translations", translations); err != nil {
		return nil, err
	}
	c.storeTranslations(translations)
	return append([]Translation(nil), translations...), nil
}

// Reciter resolves a numeric ID or migrates a legacy identifier.
func (c *Client) Reciter(ctx context.Context, identifier string) (Reciter, error) {
	reciters, err := c.Reciters(ctx)
	if err != nil {
		return Reciter{}, err
	}
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		identifier = DefaultReciterID
	}
	identifier = migrateReciter(identifier)
	if id, ok := parseResourceID(identifier); ok {
		for _, reciter := range reciters {
			if reciter.ID == strconv.Itoa(id) {
				return reciter, nil
			}
		}
		return Reciter{}, fmt.Errorf("unknown reciter ID %d; valid options: %s\nrun `peace reciters` for the full list", id, reciterOptions(reciters, 12))
	}
	for _, reciter := range reciters {
		if strings.EqualFold(reciter.Slug, identifier) || strings.EqualFold(reciter.Name, identifier) {
			return reciter, nil
		}
	}
	return Reciter{}, fmt.Errorf("unknown reciter %q; migrated legacy IDs: %s\nvalid options: %s\nrun `peace reciters` for the full list", identifier, legacyReciterIDs(), reciterOptions(reciters, 12))
}

// Translation resolves a numeric ID or migrates a legacy identifier.
func (c *Client) Translation(ctx context.Context, identifier string) (Translation, error) {
	translations, err := c.Translations(ctx)
	if err != nil {
		return Translation{}, err
	}
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		identifier = DefaultTranslationID
	}
	identifier = migrateTranslation(identifier)
	if id, ok := parseResourceID(identifier); ok {
		for _, translation := range translations {
			if translation.ID == strconv.Itoa(id) {
				return translation, nil
			}
		}
		return Translation{}, fmt.Errorf("unknown translation ID %d; valid options: %s\nrun `peace translations` for the full list", id, translationOptions(translations, 12))
	}
	for _, translation := range translations {
		if strings.EqualFold(translation.Slug, identifier) || strings.EqualFold(translation.Name, identifier) {
			return translation, nil
		}
	}
	return Translation{}, fmt.Errorf("unknown translation %q; migrated legacy IDs: %s\nvalid options: %s\nrun `peace translations` for the full list", identifier, legacyTranslationIDs(), translationOptions(translations, 12))
}

func (c *Client) chapterName(ctx context.Context, surah int) string {
	chapters, err := c.Chapters(ctx)
	if err == nil {
		for _, chapter := range chapters {
			if chapter.Number != surah {
				continue
			}
			if chapter.Name != "" {
				return chapter.Name
			}
			if chapter.EnglishName != "" {
				return chapter.EnglishName
			}
		}
	}
	return fmt.Sprintf("Surah %d", surah)
}

func (c *Client) storeChapters(chapters []Chapter) {
	c.mu.Lock()
	c.chapters = append([]Chapter(nil), chapters...)
	c.mu.Unlock()
}

func (c *Client) storeReciters(reciters []Reciter) {
	c.mu.Lock()
	c.reciters = append([]Reciter(nil), reciters...)
	c.mu.Unlock()
}

func (c *Client) storeTranslations(translations []Translation) {
	c.mu.Lock()
	c.translations = append([]Translation(nil), translations...)
	c.mu.Unlock()
}

func queryValues(pairs ...string) url.Values {
	values := url.Values{}
	for index := 0; index+1 < len(pairs); index += 2 {
		values.Set(pairs[index], pairs[index+1])
	}
	return values
}

func parseWords(words []word) []Word {
	parsed := make([]Word, 0, len(words))
	for _, entry := range words {
		if entry.CharTypeName != "" && entry.CharTypeName != "word" {
			continue
		}
		arabic := strings.TrimSpace(entry.TextUthmani)
		if arabic == "" {
			arabic = strings.TrimSpace(entry.Text)
		}
		if arabic == "" {
			continue
		}
		parsed = append(parsed, Word{
			Arabic:          arabic,
			Transliteration: strings.TrimSpace(entry.Transliteration.Text),
			Translation:     cleanTranslation(entry.Translation.Text),
		})
	}
	return parsed
}

func verseNumber(verseKey string, surah int) int {
	parts := strings.Split(verseKey, ":")
	if len(parts) != 2 {
		return 0
	}
	keySurah, err := strconv.Atoi(parts[0])
	if err != nil || keySurah != surah {
		return 0
	}
	number, err := strconv.Atoi(parts[1])
	if err != nil || number < 1 {
		return 0
	}
	return number
}

func cleanTranslation(text string) string {
	text = supPattern.ReplaceAllString(text, "")
	text = htmlPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	return strings.TrimSpace(spacePattern.ReplaceAllString(text, " "))
}

func slugify(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
			lastDash = false
		} else if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func validateSurah(surah int) error {
	if surah < 1 || surah > 114 {
		return errors.New("surah must be between 1 and 114")
	}
	return nil
}

func validateAyah(ayah int) error {
	if ayah < 1 {
		return errors.New("ayah must be at least 1")
	}
	return nil
}
