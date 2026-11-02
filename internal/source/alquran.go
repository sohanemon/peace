package source

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// alQuranCloud preserves the previous AlQuran Cloud provider behind the Source
// interface so existing configurations have a fallback.
type alQuranCloud struct {
	cacheDir string
	http     *Client
	baseURL  string

	mu         sync.Mutex
	audioCache map[string][]string
}

func newAlQuranCloud(options Options) *alQuranCloud {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = alQuranCloudBaseURL
	}
	return &alQuranCloud{
		cacheDir:   options.CacheDir,
		http:       options.HTTP,
		baseURL:    baseURL,
		audioCache: map[string][]string{},
	}
}

type alQuranEdition struct {
	Number      int    `json:"number"`
	EnglishName string `json:"englishName"`
	Ayahs       []struct {
		NumberInSurah int    `json:"numberInSurah"`
		Text          string `json:"text"`
	} `json:"ayahs"`
}

func (a *alQuranCloud) Surah(ctx context.Context, surah int, translationID string) ([]Ayah, error) {
	if err := ValidateSurah(surah); err != nil {
		return nil, err
	}
	if translationID == "" {
		translationID = DefaultTranslation
	}
	if strings.ContainsAny(translationID, ":/") {
		return nil, fmt.Errorf("invalid translation %q", translationID)
	}
	path := cacheFile(a.cacheDir, fmt.Sprintf("alquran-surah-%d-%s", surah, translationID))
	var cached []Ayah
	if readCache(path, &cached) && len(cached) > 0 {
		return cached, nil
	}
	var response struct {
		Data []alQuranEdition `json:"data"`
	}
	if err := a.http.GetJSON(ctx, alQuranCloudSurahEditionsURL(a.baseURL, surah, translationID), &response); err != nil {
		return nil, translateError("alquran.cloud", err)
	}
	if len(response.Data) < 2 {
		return nil, fmt.Errorf("alquran.cloud: translation %q is unavailable", translationID)
	}
	arabic := response.Data[0]
	translated := response.Data[1]
	translations := make(map[int]string, len(translated.Ayahs))
	for _, ayah := range translated.Ayahs {
		translations[ayah.NumberInSurah] = ayah.Text
	}
	ayahs := make([]Ayah, 0, len(arabic.Ayahs))
	for _, ayah := range arabic.Ayahs {
		ayahs = append(ayahs, Ayah{
			Surah:       surah,
			SurahName:   arabic.EnglishName,
			Number:      ayah.NumberInSurah,
			Arabic:      ayah.Text,
			Translation: translations[ayah.NumberInSurah],
		})
	}
	if len(ayahs) == 0 {
		return nil, errors.New("alquran.cloud: empty surah response")
	}
	if err := writeCache(path, ayahs); err != nil {
		return nil, err
	}
	return ayahs, nil
}

func (a *alQuranCloud) AudioURL(reciter string, surah, ayah int) (string, error) {
	if err := ValidateSurah(surah); err != nil {
		return "", err
	}
	if err := ValidateAyah(ayah); err != nil {
		return "", err
	}
	reciter = strings.TrimSpace(reciter)
	if reciter == "" {
		return "", errors.New("reciter is required (example: ar.alafasy)")
	}
	if _, err := strconv.Atoi(reciter); err == nil {
		return "", fmt.Errorf("reciter %q is a Quran.com ID; use --source quran or an AlQuran Cloud identifier", reciter)
	}
	if !safePathPart(reciter) {
		return "", fmt.Errorf("invalid reciter %q", reciter)
	}
	urls, err := a.surahAudio(reciter, surah)
	if err != nil {
		return "", err
	}
	if ayah > len(urls) || urls[ayah-1] == "" {
		return "", fmt.Errorf("alquran.cloud: audio URL missing for %d:%d", surah, ayah)
	}
	return urls[ayah-1], nil
}

func (a *alQuranCloud) surahAudio(reciter string, surah int) ([]string, error) {
	key := fmt.Sprintf("%d/%s", surah, reciter)
	a.mu.Lock()
	cached, ok := a.audioCache[key]
	a.mu.Unlock()
	if ok {
		return cached, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var response struct {
		Data struct {
			Ayahs []struct {
				NumberInSurah int    `json:"numberInSurah"`
				Audio         string `json:"audio"`
			} `json:"ayahs"`
		} `json:"data"`
	}
	if err := a.http.GetJSON(ctx, alQuranCloudSurahAudioURL(a.baseURL, surah, reciter), &response); err != nil {
		return nil, translateError("alquran.cloud", err)
	}
	if len(response.Data.Ayahs) == 0 {
		return nil, errors.New("alquran.cloud: empty audio response")
	}
	urls := make([]string, len(response.Data.Ayahs))
	for _, ayah := range response.Data.Ayahs {
		if ayah.NumberInSurah >= 1 && ayah.NumberInSurah <= len(urls) {
			urls[ayah.NumberInSurah-1] = ayah.Audio
		}
	}
	a.mu.Lock()
	a.audioCache[key] = urls
	a.mu.Unlock()
	return urls, nil
}

func (a *alQuranCloud) Words(context.Context, int, int) ([]Word, error) {
	return nil, errors.New("word-by-word data is not available from alquran.cloud; use --source quran")
}

func (a *alQuranCloud) Chapters(ctx context.Context) ([]Chapter, error) {
	path := cacheFile(a.cacheDir, "alquran-chapters")
	var cached []Chapter
	if readCache(path, &cached) && len(cached) == 114 {
		return cached, nil
	}
	var response struct {
		Data []struct {
			Number        int    `json:"number"`
			Name          string `json:"name"`
			EnglishName   string `json:"englishName"`
			NumberOfAyahs int    `json:"numberOfAyahs"`
		} `json:"data"`
	}
	if err := a.http.GetJSON(ctx, alQuranCloudSurahsURL(a.baseURL), &response); err != nil {
		return nil, translateError("alquran.cloud", err)
	}
	chapters := make([]Chapter, 0, len(response.Data))
	for _, chapter := range response.Data {
		chapters = append(chapters, Chapter{
			Number:      chapter.Number,
			Name:        chapter.Name,
			EnglishName: chapter.EnglishName,
			Verses:      chapter.NumberOfAyahs,
		})
	}
	if len(chapters) != 114 {
		return nil, errors.New("alquran.cloud: incomplete surah list received")
	}
	if err := writeCache(path, chapters); err != nil {
		return nil, err
	}
	return chapters, nil
}

func (a *alQuranCloud) Reciters(ctx context.Context) ([]Reciter, error) {
	path := cacheFile(a.cacheDir, "alquran-reciters")
	var cached []Reciter
	if readCache(path, &cached) && len(cached) > 0 {
		return cached, nil
	}
	editions, err := a.editions(ctx, "format=audio")
	if err != nil {
		return nil, err
	}
	reciters := make([]Reciter, 0, len(editions))
	for _, edition := range editions {
		name := edition.EnglishName
		if name == "" {
			name = edition.Name
		}
		reciters = append(reciters, Reciter{
			ID:       edition.Identifier,
			Name:     name,
			Slug:     edition.Identifier,
			Language: edition.Language,
		})
	}
	if err := writeCache(path, reciters); err != nil {
		return nil, err
	}
	return reciters, nil
}

func (a *alQuranCloud) Translations(ctx context.Context) ([]Translation, error) {
	path := cacheFile(a.cacheDir, "alquran-translations")
	var cached []Translation
	if readCache(path, &cached) && len(cached) > 0 {
		return cached, nil
	}
	editions, err := a.editions(ctx, "type=translation")
	if err != nil {
		return nil, err
	}
	translations := make([]Translation, 0, len(editions))
	for _, edition := range editions {
		name := edition.EnglishName
		if name == "" {
			name = edition.Name
		}
		translations = append(translations, Translation{
			ID:       edition.Identifier,
			Name:     name,
			Slug:     edition.Identifier,
			Language: edition.Language,
		})
	}
	if err := writeCache(path, translations); err != nil {
		return nil, err
	}
	return translations, nil
}

type alQuranEditionInfo struct {
	Identifier  string `json:"identifier"`
	Language    string `json:"language"`
	Name        string `json:"name"`
	EnglishName string `json:"englishName"`
}

func (a *alQuranCloud) editions(ctx context.Context, query string) ([]alQuranEditionInfo, error) {
	var response struct {
		Data []alQuranEditionInfo `json:"data"`
	}
	if err := a.http.GetJSON(ctx, alQuranCloudEditionsURL(a.baseURL, query), &response); err != nil {
		return nil, translateError("alquran.cloud", err)
	}
	if len(response.Data) == 0 {
		return nil, errors.New("alquran.cloud: empty edition list")
	}
	return response.Data, nil
}

func safePathPart(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && !strings.ContainsRune("._-", character) {
			return false
		}
	}
	return true
}
