package source

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const quranComPerPage = 50

var (
	htmlTagPattern = regexp.MustCompile(`<[^>]*>`)
	supPattern     = regexp.MustCompile(`(?s)<sup[^>]*>.*?</sup>`)
	spacePattern   = regexp.MustCompile(`\s+`)
)

type quranCom struct {
	cacheDir     string
	http         *Client
	baseURL      string
	audioBaseURL string

	mu           sync.Mutex
	recitations  []Reciter
	translations []Translation
	chapters     []Chapter
	audioURLs    map[string]string
	chapterAudio map[string]map[int]string
}

func newQuranCom(options Options) *quranCom {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = quranComBaseURL
	}
	audioBaseURL := options.AudioBaseURL
	if audioBaseURL == "" {
		audioBaseURL = quranComAudioBaseURL
	}
	return &quranCom{
		cacheDir:     options.CacheDir,
		http:         options.HTTP,
		baseURL:      baseURL,
		audioBaseURL: audioBaseURL,
		audioURLs:    map[string]string{},
		chapterAudio: map[string]map[int]string{},
	}
}

type quranComWord struct {
	Position        int    `json:"position"`
	CharTypeName    string `json:"char_type_name"`
	TextUthmani     string `json:"text_uthmani"`
	Text            string `json:"text"`
	Transliteration struct {
		Text string `json:"text"`
	} `json:"transliteration"`
	Translation struct {
		Text         string `json:"text"`
		LanguageName string `json:"language_name"`
	} `json:"translation"`
}

type quranComVerse struct {
	VerseKey     string `json:"verse_key"`
	TextUthmani  string `json:"text_uthmani"`
	Translations []struct {
		Text string `json:"text"`
	} `json:"translations"`
	Words []quranComWord `json:"words"`
}

type quranComAudioFile struct {
	VerseKey string `json:"verse_key"`
	URL      string `json:"url"`
}

type quranComAudioResponse struct {
	AudioFiles []quranComAudioFile `json:"audio_files"`
	AudioFile  quranComAudioFile   `json:"audio_file"`
}

type quranComPage struct {
	Verses     []quranComVerse `json:"verses"`
	Pagination struct {
		PerPage      int  `json:"per_page"`
		CurrentPage  int  `json:"current_page"`
		NextPage     *int `json:"next_page"`
		TotalPages   int  `json:"total_pages"`
		TotalRecords int  `json:"total_records"`
	} `json:"pagination"`
}

func (q *quranCom) Surah(ctx context.Context, surah int, translationID string) ([]Ayah, error) {
	if err := ValidateSurah(surah); err != nil {
		return nil, err
	}
	translation, err := q.resolveTranslationID(ctx, translationID)
	if err != nil {
		return nil, err
	}
	path := cacheFile(q.cacheDir, fmt.Sprintf("quran-surah-%d-%s", surah, translation.ID))
	var cached []Ayah
	if readCache(path, &cached) && len(cached) > 0 {
		return cached, nil
	}
	verses, err := q.fetchVerses(ctx, surah, translation.ID, false)
	if err != nil {
		return nil, err
	}
	surahName := fmt.Sprintf("Surah %d", surah)
	if chapters, chapterErr := q.Chapters(ctx); chapterErr == nil {
		for _, chapter := range chapters {
			if chapter.Number == surah {
				surahName = chapter.Name
				if surahName == "" {
					surahName = chapter.EnglishName
				}
				break
			}
		}
	}
	ayahs := make([]Ayah, 0, len(verses))
	for _, verse := range verses {
		number := verseNumber(verse.VerseKey, surah)
		if number == 0 {
			continue
		}
		translationText := ""
		if len(verse.Translations) > 0 {
			translationText = cleanTranslation(verse.Translations[0].Text)
		}
		ayahs = append(ayahs, Ayah{
			Surah:       surah,
			SurahName:   surahName,
			Number:      number,
			Arabic:      verse.TextUthmani,
			Translation: translationText,
			Words:       parseWords(verse.Words),
		})
	}
	if len(ayahs) == 0 {
		return nil, errors.New("quran.com: empty surah response")
	}
	if err := writeCache(path, ayahs); err != nil {
		return nil, err
	}
	return ayahs, nil
}

func (q *quranCom) AudioURL(reciter string, surah, ayah int) (string, error) {
	if err := ValidateSurah(surah); err != nil {
		return "", err
	}
	if err := ValidateAyah(ayah); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resolved, err := q.resolveReciterID(ctx, reciter)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s/%d:%d", resolved.ID, surah, ayah)
	chapterKey := fmt.Sprintf("%s/%d", resolved.ID, surah)
	q.mu.Lock()
	cached, ok := q.audioURLs[key]
	chapter, chapterCached := q.chapterAudio[chapterKey]
	q.mu.Unlock()
	if ok {
		return cached, nil
	}
	if chapterCached && chapter[ayah] != "" {
		return chapter[ayah], nil
	}
	recitationID, err := strconv.Atoi(resolved.ID)
	if err != nil {
		return "", fmt.Errorf("quran.com: invalid recitation ID %q", resolved.ID)
	}
	// NOTE: Fetch the whole chapter once so a long surah does not require one network
	// request per ayah. Fall back to the by_ayah endpoint if that is unavailable.
	var chapterResponse quranComAudioResponse
	if err := q.http.GetJSON(ctx, quranComChapterAudioURL(q.baseURL, recitationID, surah), &chapterResponse); err == nil {
		audio := make(map[int]string, len(chapterResponse.AudioFiles))
		for _, file := range chapterResponse.AudioFiles {
			number := verseNumber(file.VerseKey, surah)
			if number == 0 || file.URL == "" {
				continue
			}
			audio[number] = quranComAudioPath(q.audioBaseURL, file.URL)
		}
		if len(audio) > 0 {
			q.mu.Lock()
			q.chapterAudio[chapterKey] = audio
			for number, url := range audio {
				q.audioURLs[fmt.Sprintf("%s/%d:%d", resolved.ID, surah, number)] = url
			}
			q.mu.Unlock()
			if url := audio[ayah]; url != "" {
				return url, nil
			}
		}
	}
	var response quranComAudioResponse
	if err := q.http.GetJSON(ctx, quranComAudioURL(q.baseURL, recitationID, surah, ayah), &response); err != nil {
		return "", translateError("quran.com", err)
	}
	relative := ""
	if len(response.AudioFiles) > 0 {
		relative = response.AudioFiles[0].URL
	}
	if relative == "" {
		relative = response.AudioFile.URL
	}
	if relative == "" {
		return "", fmt.Errorf("quran.com: audio URL missing for %d:%d", surah, ayah)
	}
	absolute := quranComAudioPath(q.audioBaseURL, relative)
	q.mu.Lock()
	q.audioURLs[key] = absolute
	q.mu.Unlock()
	return absolute, nil
}

func (q *quranCom) Words(ctx context.Context, surah, ayah int) ([]Word, error) {
	if err := ValidateSurah(surah); err != nil {
		return nil, err
	}
	if err := ValidateAyah(ayah); err != nil {
		return nil, err
	}
	path := cacheFile(q.cacheDir, fmt.Sprintf("quran-words-%d", surah))
	var cached map[string][]Word
	if readCache(path, &cached) {
		if words, ok := cached[strconv.Itoa(ayah)]; ok && len(words) > 0 {
			return words, nil
		}
	}
	verses, err := q.fetchVerses(ctx, surah, "", true)
	if err != nil {
		return nil, err
	}
	wordsByAyah := make(map[string][]Word, len(verses))
	for _, verse := range verses {
		number := verseNumber(verse.VerseKey, surah)
		if number == 0 {
			continue
		}
		words := parseWords(verse.Words)
		if len(words) > 0 {
			wordsByAyah[strconv.Itoa(number)] = words
		}
	}
	if len(wordsByAyah) == 0 {
		return nil, errors.New("quran.com: empty word-by-word response")
	}
	if err := writeCache(path, wordsByAyah); err != nil {
		return nil, err
	}
	if words, ok := wordsByAyah[strconv.Itoa(ayah)]; ok && len(words) > 0 {
		return words, nil
	}
	return nil, fmt.Errorf("quran.com: no words for %d:%d", surah, ayah)
}

func (q *quranCom) fetchVerses(ctx context.Context, surah int, translationID string, words bool) ([]quranComVerse, error) {
	var verses []quranComVerse
	for page := 1; page <= 200; page++ {
		var response quranComPage
		target := quranComVersesURL(q.baseURL, surah, page, quranComPerPage, translationID, words)
		if err := q.http.GetJSON(ctx, target, &response); err != nil {
			return nil, translateError("quran.com", err)
		}
		verses = append(verses, response.Verses...)
		next := response.Pagination.NextPage
		if next == nil || *next <= page {
			return verses, nil
		}
	}
	return nil, errors.New("quran.com: pagination did not terminate")
}

func (q *quranCom) Chapters(ctx context.Context) ([]Chapter, error) {
	path := cacheFile(q.cacheDir, "quran-chapters")
	q.mu.Lock()
	if len(q.chapters) == 114 {
		chapters := append([]Chapter(nil), q.chapters...)
		q.mu.Unlock()
		return chapters, nil
	}
	q.mu.Unlock()
	var cached []Chapter
	if readCache(path, &cached) && len(cached) == 114 {
		q.mu.Lock()
		q.chapters = cached
		q.mu.Unlock()
		return cached, nil
	}
	var response struct {
		Chapters []struct {
			ID             int    `json:"id"`
			NameSimple     string `json:"name_simple"`
			NameArabic     string `json:"name_arabic"`
			VersesCount    int    `json:"verses_count"`
			TranslatedName struct {
				Name string `json:"name"`
			} `json:"translated_name"`
		} `json:"chapters"`
	}
	if err := q.http.GetJSON(ctx, quranComChaptersURL(q.baseURL), &response); err != nil {
		return nil, translateError("quran.com", err)
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
		return nil, fmt.Errorf("quran.com: incomplete chapter list (%d)", len(chapters))
	}
	if err := writeCache(path, chapters); err != nil {
		return nil, err
	}
	q.mu.Lock()
	q.chapters = chapters
	q.mu.Unlock()
	return chapters, nil
}

func (q *quranCom) Reciters(ctx context.Context) ([]Reciter, error) {
	path := cacheFile(q.cacheDir, "quran-reciters")
	q.mu.Lock()
	if len(q.recitations) > 0 {
		reciters := append([]Reciter(nil), q.recitations...)
		q.mu.Unlock()
		return reciters, nil
	}
	q.mu.Unlock()
	var cached []Reciter
	if readCache(path, &cached) && len(cached) > 0 {
		q.mu.Lock()
		q.recitations = cached
		q.mu.Unlock()
		return cached, nil
	}
	var response struct {
		Recitations []struct {
			ID             int     `json:"id"`
			ReciterName    string  `json:"reciter_name"`
			Style          *string `json:"style"`
			TranslatedName struct {
				Name         string `json:"name"`
				LanguageName string `json:"language_name"`
			} `json:"translated_name"`
		} `json:"recitations"`
	}
	if err := q.http.GetJSON(ctx, quranComRecitationsURL(q.baseURL), &response); err != nil {
		return nil, translateError("quran.com", err)
	}
	reciters := make([]Reciter, 0, len(response.Recitations))
	for _, recitation := range response.Recitations {
		name := recitation.TranslatedName.Name
		if name == "" {
			name = recitation.ReciterName
		}
		slugSource := recitation.ReciterName
		if slugSource == "" {
			slugSource = name
		}
		if recitation.Style != nil && *recitation.Style != "" {
			slugSource += " " + *recitation.Style
		}
		reciters = append(reciters, Reciter{
			ID:       strconv.Itoa(recitation.ID),
			Name:     name,
			Slug:     slugify(slugSource),
			Language: "arabic",
		})
	}
	if len(reciters) == 0 {
		return nil, errors.New("quran.com: empty recitation list")
	}
	sort.SliceStable(reciters, func(first, second int) bool {
		left, _ := strconv.Atoi(reciters[first].ID)
		right, _ := strconv.Atoi(reciters[second].ID)
		return left < right
	})
	if err := writeCache(path, reciters); err != nil {
		return nil, err
	}
	q.mu.Lock()
	q.recitations = reciters
	q.mu.Unlock()
	return append([]Reciter(nil), reciters...), nil
}

func (q *quranCom) Translations(ctx context.Context) ([]Translation, error) {
	path := cacheFile(q.cacheDir, "quran-translations")
	q.mu.Lock()
	if len(q.translations) > 0 {
		translations := append([]Translation(nil), q.translations...)
		q.mu.Unlock()
		return translations, nil
	}
	q.mu.Unlock()
	var cached []Translation
	if readCache(path, &cached) && len(cached) > 0 {
		q.mu.Lock()
		q.translations = cached
		q.mu.Unlock()
		return cached, nil
	}
	var response struct {
		Translations []struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			AuthorName   string `json:"author_name"`
			Slug         string `json:"slug"`
			LanguageName string `json:"language_name"`
			LanguageCode string `json:"language_code"`
		} `json:"translations"`
	}
	if err := q.http.GetJSON(ctx, quranComTranslationsURL(q.baseURL), &response); err != nil {
		return nil, translateError("quran.com", err)
	}
	translations := make([]Translation, 0, len(response.Translations))
	for _, translation := range response.Translations {
		language := translation.LanguageName
		if language == "" {
			language = translation.LanguageCode
		}
		translations = append(translations, Translation{
			ID:       strconv.Itoa(translation.ID),
			Name:     translation.Name,
			Slug:     translation.Slug,
			Language: language,
		})
	}
	if len(translations) == 0 {
		return nil, errors.New("quran.com: empty translation list")
	}
	sort.SliceStable(translations, func(first, second int) bool {
		left, _ := strconv.Atoi(translations[first].ID)
		right, _ := strconv.Atoi(translations[second].ID)
		return left < right
	})
	if err := writeCache(path, translations); err != nil {
		return nil, err
	}
	q.mu.Lock()
	q.translations = translations
	q.mu.Unlock()
	return append([]Translation(nil), translations...), nil
}

func (q *quranCom) resolveReciterID(ctx context.Context, identifier string) (Reciter, error) {
	reciters, err := q.Reciters(ctx)
	if err != nil {
		return Reciter{}, err
	}
	return resolveReciter(reciters, identifier)
}

func (q *quranCom) resolveTranslationID(ctx context.Context, identifier string) (Translation, error) {
	translations, err := q.Translations(ctx)
	if err != nil {
		return Translation{}, err
	}
	return resolveTranslation(translations, identifier)
}

func parseWords(words []quranComWord) []Word {
	parsed := make([]Word, 0, len(words))
	for _, word := range words {
		if word.CharTypeName != "" && word.CharTypeName != "word" {
			continue
		}
		arabic := word.TextUthmani
		if arabic == "" {
			arabic = word.Text
		}
		if arabic == "" {
			continue
		}
		parsed = append(parsed, Word{
			Arabic:          arabic,
			Transliteration: strings.TrimSpace(word.Transliteration.Text),
			Translation:     strings.TrimSpace(word.Translation.Text),
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
	text = htmlTagPattern.ReplaceAllString(text, " ")
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
