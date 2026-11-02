package source

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// NOTE: The Quran.com API exposes numeric resource IDs. Existing peace configs
// use AlQuran Cloud identifiers, so resolve those identifiers against the live
// /resources lists by name/slug instead of hardcoding numeric IDs here.
var legacyReciterAliases = map[string][]string{
	"ar.alafasy":            {"alafasy", "al afasy", "afasy"},
	"ar.husary":             {"husary", "husari"},
	"ar.husarymujawwad":     {"husary mujawwad", "husari mujawwad"},
	"ar.abdulbasitmurattal": {"abdulbaset murattal", "abdul basit murattal", "abdulbaset"},
	"ar.abdurrahmaansudais": {"sudais", "sudays"},
	"ar.saoodshuraym":       {"shuraim", "shuraym"},
	"ar.minshawi":           {"minshawi murattal", "minshawy murattal", "minshawi", "minshawy"},
	"ar.muhammadayyoub":     {"ayyoub", "ayyub"},
	"ar.muhammadjibreel":    {"jibreel", "jibril"},
	"ar.ahmedajamy":         {"ajamy", "ajmi"},
	"ar.hudhaify":           {"hudhaify", "hudhaifi"},
	"ar.shaatree":           {"shaatree", "shatri"},
}

var legacyTranslationAliases = map[string][]string{
	"en.sahih":      {"saheeh international", "sahih international", "saheeh"},
	"en.pickthall":  {"pickthall"},
	"en.yusufali":   {"yusuf ali"},
	"en.hilali":     {"hilali", "khan"},
	"en.asad":       {"muhammad asad", "asad"},
	"en.clearquran": {"clear quran", "mustafa khattab", "khattab"},
	"en.daryabadi":  {"daryabadi"},
	"ur.jalandhry":  {"jalandhry", "jalandhari"},
	"ur.junagarhi":  {"junagarhi"},
	"id.indonesian": {"indonesian"},
	"fr.hamidullah": {"hamidullah"},
	"tr.diyanet":    {"diyanet"},
	"bn.bengali":    {"bengali"},
	"ru.kuliev":     {"kuliev"},
	"es.cortes":     {"cortes"},
	"de.aburida":    {"aburida", "abu rida"},
	"nl.keyzer":     {"keyzer"},
	"pt.elhayek":    {"elhayek"},
	"hi.hindi":      {"hindi"},
	"zh.jian":       {"jian"},
	"ja.japanese":   {"japanese"},
	"ko.korean":     {"korean"},
	"th.thai":       {"thai"},
	"ms.basmeih":    {"basmeih"},
	"sw.barwani":    {"barwani"},
	"ha.gumi":       {"gumi"},
	"fa.ansarian":   {"ansarian"},
}

func resolveReciter(reciters []Reciter, identifier string) (Reciter, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return Reciter{}, fmt.Errorf("reciter is required (example: ar.alafasy); run `peace reciters` for options")
	}
	if id, err := strconv.Atoi(identifier); err == nil {
		for _, reciter := range reciters {
			if reciter.ID == strconv.Itoa(id) {
				return reciter, nil
			}
		}
		return Reciter{}, fmt.Errorf("unknown reciter ID %d; run `peace reciters` for options", id)
	}
	if terms, ok := legacyReciterAliases[strings.ToLower(identifier)]; ok {
		for _, term := range terms {
			var matches []Reciter
			for _, reciter := range reciters {
				if containsNormalized(reciterHaystack(reciter), term) {
					matches = append(matches, reciter)
				}
			}
			if len(matches) > 0 {
				return preferredReciter(matches), nil
			}
		}
	}
	// NOTE: Accept a slug or exact display name from `peace reciters`.
	for _, reciter := range reciters {
		if strings.EqualFold(reciter.Slug, identifier) || strings.EqualFold(reciter.Name, identifier) {
			return reciter, nil
		}
	}
	return Reciter{}, fmt.Errorf("unknown reciter %q\nvalid legacy IDs: %s\nQuran.com options: %s\nrun `peace reciters` for the full list", identifier, legacyKeys(legacyReciterAliases), reciterOptions(reciters, 12))
}

func resolveTranslation(translations []Translation, identifier string) (Translation, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		identifier = DefaultTranslation
	}
	if id, err := strconv.Atoi(identifier); err == nil {
		for _, translation := range translations {
			if translation.ID == strconv.Itoa(id) {
				return translation, nil
			}
		}
		return Translation{}, fmt.Errorf("unknown translation ID %d; run `peace translations` for options", id)
	}
	if terms, ok := legacyTranslationAliases[strings.ToLower(identifier)]; ok {
		for _, term := range terms {
			for _, translation := range translations {
				if containsNormalized(translationHaystack(translation), term) {
					return translation, nil
				}
			}
		}
	}
	for _, translation := range translations {
		if strings.EqualFold(translation.Slug, identifier) || strings.EqualFold(translation.Name, identifier) {
			return translation, nil
		}
	}
	return Translation{}, fmt.Errorf("unknown translation %q\nvalid legacy IDs: %s\nQuran.com options: %s\nrun `peace translations` for the full list", identifier, legacyKeys(legacyTranslationAliases), translationOptions(translations, 12))
}

func reciterHaystack(reciter Reciter) string {
	return normalize(reciter.Name + " " + reciter.Slug + " " + reciter.Language)
}

func translationHaystack(translation Translation) string {
	return normalize(translation.Name + " " + translation.Slug + " " + translation.Language)
}

func containsNormalized(haystack, needle string) bool {
	needle = normalize(needle)
	if needle == "" {
		return false
	}
	for _, token := range strings.Fields(needle) {
		if !strings.Contains(haystack, token) {
			return false
		}
	}
	return true
}

func preferredReciter(matches []Reciter) Reciter {
	styles := []string{"mujawwad", "murattal", "muallim", "warsh", "qalun", "duri", "sousi", "shoba"}
	for _, reciter := range matches {
		hasStyle := false
		for _, style := range styles {
			if strings.Contains(reciter.Slug, style) {
				hasStyle = true
				break
			}
		}
		if !hasStyle {
			return reciter
		}
	}
	return matches[0]
}

func normalize(value string) string {
	var builder strings.Builder
	lastSpace := true
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func legacyKeys(aliases map[string][]string) string {
	keys := make([]string, 0, len(aliases))
	for key := range aliases {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func reciterOptions(reciters []Reciter, limit int) string {
	options := make([]string, 0, min(limit, len(reciters)))
	for index, reciter := range reciters {
		if index >= limit {
			break
		}
		options = append(options, fmt.Sprintf("%s (%s)", reciter.ID, reciter.Name))
	}
	suffix := ""
	if len(reciters) > limit {
		suffix = fmt.Sprintf(", … (%d total)", len(reciters))
	}
	return strings.Join(options, ", ") + suffix
}

func translationOptions(translations []Translation, limit int) string {
	options := make([]string, 0, min(limit, len(translations)))
	for index, translation := range translations {
		if index >= limit {
			break
		}
		options = append(options, fmt.Sprintf("%s (%s)", translation.ID, translation.Name))
	}
	suffix := ""
	if len(translations) > limit {
		suffix = fmt.Sprintf(", … (%d total)", len(translations))
	}
	return strings.Join(options, ", ") + suffix
}

func min(first, second int) int {
	if first < second {
		return first
	}
	return second
}
