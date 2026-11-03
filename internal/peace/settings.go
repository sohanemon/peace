package peace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"io.github.sohanemon/peace/internal/quran"
)

// UserSettings represents persisted user preferences for peace.
type UserSettings struct {
	Translation string `json:"translation"`
	WBWLanguage string `json:"wbw_language"`
}

func defaultSettings() UserSettings {
	return UserSettings{
		Translation: quran.DefaultTranslationID,
		WBWLanguage: quran.DefaultWBWLanguage,
	}
}

func (r Runtime) settingsFile() string {
	return filepath.Join(r.Config.StateDir, "settings.json")
}

func (r Runtime) readSettings() UserSettings {
	settings := defaultSettings()
	data, err := os.ReadFile(r.settingsFile())
	if err == nil {
		_ = json.Unmarshal(data, &settings)
	} else if r.Config.Home != "" {
		// If settings.json doesn't exist, check Omarchy shell.json for inline plugin settings
		shellPath := filepath.Join(r.Config.Home, ".config", "omarchy", "shell.json")
		if shellData, err := os.ReadFile(shellPath); err == nil {
			var shellConfig struct {
				Bar struct {
					Sections struct {
						Right  []any `json:"right"`
						Left   []any `json:"left"`
						Center []any `json:"center"`
					} `json:"sections"`
				} `json:"bar"`
				Plugins []any `json:"plugins"`
			}
			if json.Unmarshal(shellData, &shellConfig) == nil {
				findPlugin := func(items []any) map[string]any {
					for _, item := range items {
						if m, ok := item.(map[string]any); ok {
							if id, ok := m["id"].(string); ok && id == "io.github.sohanemon.omapeace" {
								return m
							}
						}
					}
					return nil
				}
				m := findPlugin(shellConfig.Bar.Sections.Right)
				if m == nil {
					m = findPlugin(shellConfig.Bar.Sections.Left)
				}
				if m == nil {
					m = findPlugin(shellConfig.Bar.Sections.Center)
				}
				if m == nil {
					m = findPlugin(shellConfig.Plugins)
				}
				if m != nil {
					if tr, ok := m["translation"].(string); ok && strings.TrimSpace(tr) != "" {
						settings.Translation = strings.TrimSpace(tr)
					}
					if wbw, ok := m["wbwLanguage"].(string); ok && strings.TrimSpace(wbw) != "" {
						settings.WBWLanguage = strings.TrimSpace(wbw)
					}
				}
			}
		}
	}

	if settings.Translation == "" {
		settings.Translation = quran.DefaultTranslationID
	}
	if settings.WBWLanguage == "" {
		settings.WBWLanguage = quran.DefaultWBWLanguage
	}
	return settings
}

func (r Runtime) writeSettings(settings UserSettings) error {
	if r.Config.StateDir == "" {
		return nil
	}
	if err := os.MkdirAll(r.Config.StateDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(r.settingsFile(), append(data, '\n'), 0o600)
}

func (r Runtime) updateSettings(translation, wbwLanguage string) (UserSettings, error) {
	current := r.readSettings()
	translation = strings.TrimSpace(translation)
	wbwLanguage = strings.TrimSpace(wbwLanguage)
	if translation != "" {
		current.Translation = translation
	}
	if wbwLanguage != "" {
		current.WBWLanguage = wbwLanguage
	}
	return current, r.writeSettings(current)
}
