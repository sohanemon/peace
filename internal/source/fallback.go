package source

import (
	"context"
	"fmt"
)

// NOTE: fallbackLibrary queries the primary provider first and retries the
// fallback provider only when the primary returns an error.
type fallbackLibrary struct {
	primary  Library
	fallback Library
}

func newFallbackLibrary(primary, fallback Library) Library {
	return &fallbackLibrary{primary: primary, fallback: fallback}
}

func (f *fallbackLibrary) Surah(ctx context.Context, surah int, translationID string) ([]Ayah, error) {
	result, err := f.primary.Surah(ctx, surah, translationID)
	if err == nil {
		return result, nil
	}
	fallbackResult, fallbackErr := f.fallback.Surah(ctx, surah, translationID)
	if fallbackErr != nil {
		return nil, fallbackError(err, fallbackErr)
	}
	return fallbackResult, nil
}

func (f *fallbackLibrary) AudioURL(reciter string, surah, ayah int) (string, error) {
	result, err := f.primary.AudioURL(reciter, surah, ayah)
	if err == nil {
		return result, nil
	}
	fallbackResult, fallbackErr := f.fallback.AudioURL(reciter, surah, ayah)
	if fallbackErr != nil {
		return "", fallbackError(err, fallbackErr)
	}
	return fallbackResult, nil
}

func (f *fallbackLibrary) Words(ctx context.Context, surah, ayah int) ([]Word, error) {
	result, err := f.primary.Words(ctx, surah, ayah)
	if err == nil {
		return result, nil
	}
	fallbackResult, fallbackErr := f.fallback.Words(ctx, surah, ayah)
	if fallbackErr != nil {
		return nil, fallbackError(err, fallbackErr)
	}
	return fallbackResult, nil
}

func (f *fallbackLibrary) Chapters(ctx context.Context) ([]Chapter, error) {
	result, err := f.primary.Chapters(ctx)
	if err == nil {
		return result, nil
	}
	fallbackResult, fallbackErr := f.fallback.Chapters(ctx)
	if fallbackErr != nil {
		return nil, fallbackError(err, fallbackErr)
	}
	return fallbackResult, nil
}

func (f *fallbackLibrary) Reciters(ctx context.Context) ([]Reciter, error) {
	result, err := f.primary.Reciters(ctx)
	if err == nil {
		return result, nil
	}
	fallbackResult, fallbackErr := f.fallback.Reciters(ctx)
	if fallbackErr != nil {
		return nil, fallbackError(err, fallbackErr)
	}
	return fallbackResult, nil
}

func (f *fallbackLibrary) Translations(ctx context.Context) ([]Translation, error) {
	result, err := f.primary.Translations(ctx)
	if err == nil {
		return result, nil
	}
	fallbackResult, fallbackErr := f.fallback.Translations(ctx)
	if fallbackErr != nil {
		return nil, fallbackError(err, fallbackErr)
	}
	return fallbackResult, nil
}

func fallbackError(primaryErr, fallbackErr error) error {
	return fmt.Errorf("primary source failed: %v; fallback source failed: %w", primaryErr, fallbackErr)
}
