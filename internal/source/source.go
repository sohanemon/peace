// Package source abstracts Quran text, translation, audio, and word data
// providers behind a single interface.
package source

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DefaultTranslation is the translation used when no identifier is configured.
const DefaultTranslation = "en.sahih"

// Ayah is one verse with its Arabic text, optional translation, and words.
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

// Source provides the core Quran data needed for playback and wallpapers.
type Source interface {
	Surah(ctx context.Context, surah int, translationID string) ([]Ayah, error)
	AudioURL(reciter string, surah, ayah int) (string, error)
	Words(ctx context.Context, surah, ayah int) ([]Word, error)
}

// Catalog lists the resources exposed by a source.
type Catalog interface {
	Chapters(ctx context.Context) ([]Chapter, error)
	Reciters(ctx context.Context) ([]Reciter, error)
	Translations(ctx context.Context) ([]Translation, error)
}

// Library is a Source that can also list its available resources.
type Library interface {
	Source
	Catalog
}

// Options configures a source implementation.
type Options struct {
	CacheDir     string
	HTTP         *Client
	BaseURL      string
	AudioBaseURL string
	// Fallback optionally selects a second source used when the primary fails.
	Fallback string
}

// New builds the source selected by name. An empty name selects alquran.cloud.
func New(name string, options Options) (Library, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = "alquran"
	}
	if options.HTTP == nil {
		options.HTTP = NewClient()
	}
	if options.CacheDir == "" {
		cacheDir, err := defaultCacheDir()
		if err != nil {
			return nil, err
		}
		options.CacheDir = cacheDir
	}
	primary, err := newProvider(name, options)
	if err != nil {
		return nil, err
	}
	fallback := strings.ToLower(strings.TrimSpace(options.Fallback))
	if fallback == "" || fallback == name {
		return primary, nil
	}
	secondary, err := newProvider(fallback, options)
	if err != nil {
		return nil, err
	}
	return newFallbackLibrary(primary, secondary), nil
}

func newProvider(name string, options Options) (Library, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "alquran", "alquran.cloud", "alqurancloud":
		return newAlQuranCloud(options), nil
	default:
		return nil, fmt.Errorf("unknown source %q (valid sources: quran, alquran)", name)
	}
}

// ValidateSurah rejects references outside the Quran.
func ValidateSurah(surah int) error {
	if surah < 1 || surah > 114 {
		return errors.New("surah must be between 1 and 114")
	}
	return nil
}

// ValidateAyah rejects non-positive ayah numbers.
func ValidateAyah(ayah int) error {
	if ayah < 1 {
		return errors.New("ayah must be at least 1")
	}
	return nil
}

func defaultCacheDir() (string, error) {
	dir, err := userCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate cache directory: %w", err)
	}
	return dir, nil
}
