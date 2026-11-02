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
	if relative == "" || strings.HasPrefix(relative, "http://") || strings.HasPrefix(relative, "https://") {
		return relative
	}
	return strings.TrimRight(audioBaseURL, "/") + "/" + strings.TrimLeft(relative, "/")
}
