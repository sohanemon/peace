package quran

import (
	"os"
	"strconv"
	"strings"
)

// NOTE: Every Quran.com URL and header value lives in this file. The base URL
// can be redirected through PEACE_API_BASE for a proxy or an authenticated
// endpoint without touching parsing code.
const (
	defaultBaseURL = "https://api.quran.com/api/v4"
	audioBaseURL   = "https://verses.quran.com"

	baseURLEnv = "PEACE_API_BASE"

	// NOTE: QuranFoundationTokenEnv optionally supplies a bearer token for the
	// authenticated Quran Foundation endpoint.
	QuranFoundationTokenEnv = "QURAN_FOUNDATION_TOKEN"

	userAgent = "peace (+https://github.com/sohanemon/peace)"
)

const (
	pathChapters     = "/chapters"
	pathRecitations  = "/resources/recitations"
	pathTranslations = "/resources/translations"
	pathLanguages    = "/resources/languages"
)

func apiBaseURL() string {
	if value := strings.TrimSpace(os.Getenv(baseURLEnv)); value != "" {
		return strings.TrimRight(value, "/")
	}
	return defaultBaseURL
}

func chaptersPath() string {
	return pathChapters
}

func versesPath(surah int) string {
	return "/verses/by_chapter/" + strconv.Itoa(surah)
}

func ayahAudioPath(recitationID, surah, ayah int) string {
	return "/recitations/" + strconv.Itoa(recitationID) + "/by_ayah/" + strconv.Itoa(surah) + ":" + strconv.Itoa(ayah)
}

func chapterAudioPath(recitationID, surah int) string {
	return "/recitations/" + strconv.Itoa(recitationID) + "/by_chapter/" + strconv.Itoa(surah)
}

func absoluteAudioURL(relative string) string {
	relative = strings.TrimSpace(relative)
	if relative == "" || strings.HasPrefix(relative, "http://") || strings.HasPrefix(relative, "https://") {
		return relative
	}
	trimmed := strings.TrimLeft(relative, "/")
	firstSegment := trimmed
	if slash := strings.IndexByte(trimmed, '/'); slash >= 0 {
		firstSegment = trimmed[:slash]
	}
	// NOTE: Some Quran.com audio entries already include a host without a
	// scheme (for example mirrors.quranicaudio.com/everyayah/...). Prefix those
	// directly instead of sending them through verses.quran.com.
	if strings.Contains(firstSegment, ".") {
		return "https://" + trimmed
	}
	return strings.TrimRight(audioBaseURL, "/") + "/" + trimmed
}
