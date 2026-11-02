package quran

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// NOTE: One-time migration for configurations written before peace used
// Quran.com resource IDs. These values point at well-known resources; every
// ID is still validated against the live /resources lists before use.
var legacyReciters = map[string]string{
	"ar.alafasy":            "7",
	"ar.husary":             "6",
	"ar.abdulbasitmurattal": "10",
	"ar.abdulbasitmujawwad": "9",
	"ar.husarymuallim":      "12",
}

var legacyTranslations = map[string]string{
	"en.sahih":      "20",
	"en.pickthall":  "19",
	"en.clearquran": "131",
	"ur.jalandhry":  "85",
}

func migrateReciter(identifier string) string {
	if migrated, ok := legacyReciters[strings.ToLower(identifier)]; ok {
		return migrated
	}
	return identifier
}

func migrateTranslation(identifier string) string {
	if migrated, ok := legacyTranslations[strings.ToLower(identifier)]; ok {
		return migrated
	}
	return identifier
}

func reciterOptions(reciters []Reciter, limit int) string {
	options := make([]string, 0, min(limit, len(reciters)))
	for index, reciter := range reciters {
		if index >= limit {
			break
		}
		options = append(options, fmt.Sprintf("%s (%s)", reciter.ID, reciter.Name))
	}
	if len(reciters) > limit {
		options = append(options, fmt.Sprintf("… (%d total)", len(reciters)))
	}
	return strings.Join(options, ", ")
}

func translationOptions(translations []Translation, limit int) string {
	options := make([]string, 0, min(limit, len(translations)))
	for index, translation := range translations {
		if index >= limit {
			break
		}
		options = append(options, fmt.Sprintf("%s (%s)", translation.ID, translation.Name))
	}
	if len(translations) > limit {
		options = append(options, fmt.Sprintf("… (%d total)", len(translations)))
	}
	return strings.Join(options, ", ")
}

func legacyReciterIDs() string {
	return legacyKeys(legacyReciters)
}

func legacyTranslationIDs() string {
	return legacyKeys(legacyTranslations)
}

func legacyKeys(table map[string]string) string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func parseResourceID(identifier string) (int, bool) {
	id, err := strconv.Atoi(identifier)
	return id, err == nil
}
