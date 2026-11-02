package source

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// NOTE: All provider URLs, path builders, and authentication headers live here so
// the open quran.com endpoint can be swapped for the authenticated Quran
// Foundation API without touching the parsing code.
const (
	quranComBaseURL      = "https://api.quran.com/api/v4"
	quranComAudioBaseURL = "https://verses.quran.com/"
	alQuranCloudBaseURL  = "https://api.alquran.cloud/v1"

	// NOTE: QuranFoundationTokenEnv optionally supplies a bearer token.
	// The open endpoint ignores it; the authenticated endpoint will require it later.
	QuranFoundationTokenEnv = "QURAN_FOUNDATION_TOKEN"

	userAgent = "peace (+https://github.com/sohanemon/peace)"
)

func setProviderHeaders(request *http.Request) {
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "application/json")
	// NOTE: Never leak the Quran Foundation token to third-party providers.
	if !isQuranHost(request.URL.Host) {
		return
	}
	if token := strings.TrimSpace(getenv(QuranFoundationTokenEnv)); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
}

func isQuranHost(host string) bool {
	host = strings.ToLower(host)
	return strings.Contains(host, "quran.com") || strings.Contains(host, "quran.foundation")
}

func quranComChaptersURL(base string) string {
	return strings.TrimRight(base, "/") + "/chapters?language=en"
}

func quranComRecitationsURL(base string) string {
	return strings.TrimRight(base, "/") + "/resources/recitations"
}

func quranComTranslationsURL(base string) string {
	return strings.TrimRight(base, "/") + "/resources/translations"
}

func quranComVersesURL(base string, surah, page, perPage int, translationID string, words bool) string {
	query := url.Values{}
	query.Set("fields", "text_uthmani")
	query.Set("per_page", fmt.Sprintf("%d", perPage))
	query.Set("page", fmt.Sprintf("%d", page))
	if translationID != "" {
		query.Set("translations", translationID)
	}
	if words {
		query.Set("words", "true")
		query.Set("word_fields", "text_uthmani,transliteration,translation")
	}
	return fmt.Sprintf("%s/verses/by_chapter/%d?%s", strings.TrimRight(base, "/"), surah, query.Encode())
}

func quranComAudioURL(base string, recitationID, surah, ayah int) string {
	return fmt.Sprintf("%s/recitations/%d/by_ayah/%d:%d", strings.TrimRight(base, "/"), recitationID, surah, ayah)
}

func quranComChapterAudioURL(base string, recitationID, surah int) string {
	return fmt.Sprintf("%s/recitations/%d/by_chapter/%d", strings.TrimRight(base, "/"), recitationID, surah)
}

func quranComAudioPath(audioBase, relative string) string {
	if relative == "" || strings.HasPrefix(relative, "http://") || strings.HasPrefix(relative, "https://") {
		return relative
	}
	return strings.TrimRight(audioBase, "/") + "/" + strings.TrimLeft(relative, "/")
}

func alQuranCloudSurahEditionsURL(base string, surah int, translations string) string {
	return fmt.Sprintf("%s/surah/%d/editions/quran-uthmani,%s", strings.TrimRight(base, "/"), surah, translations)
}

func alQuranCloudSurahsURL(base string) string {
	return strings.TrimRight(base, "/") + "/surah"
}

func alQuranCloudSurahAudioURL(base string, surah int, reciter string) string {
	return fmt.Sprintf("%s/surah/%d/%s", strings.TrimRight(base, "/"), surah, reciter)
}

func alQuranCloudEditionsURL(base, query string) string {
	return strings.TrimRight(base, "/") + "/edition?" + query
}
