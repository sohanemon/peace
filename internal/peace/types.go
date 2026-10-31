package peace

import (
	"encoding/json"
)

// Config holds the runtime configuration for the peace application.
type Config struct {
	Home      string
	PluginDir string
	StateDir  string
	CacheDir  string
	Socket    string
}

// Options holds the configurable options for the listen command.
type Options struct {
	Reference string
	Reciter   string
	Follow    bool
	Random    bool
	Buffer    int
	Lead      int
	Jobs      int
}

// PlayState represents the current playback state of a recitation session.
type PlayState struct {
	Surah     int      `json:"surah"`
	Start     int      `json:"start"`
	End       int      `json:"end"`
	Reciter   string   `json:"reciter"`
	Follow    bool     `json:"follow"`
	BaseImage string   `json:"base_image,omitempty"`
	AudioURLs []string `json:"audio_urls,omitempty"`
	Buffer    int      `json:"buffer,omitempty"`
	Lead      int      `json:"lead,omitempty"`
	Jobs      int      `json:"jobs,omitempty"`
	Preparing bool     `json:"preparing"`
	Ready     int      `json:"ready"`
	Appended  int      `json:"appended"`
}

// APIResponse is the envelope returned by the Quran API.
type APIResponse struct {
	Data json.RawMessage `json:"data"`
}

// Surah describes a single surah from the Quran API.
type Surah struct {
	Number        int `json:"number"`
	NumberOfAyahs int `json:"numberOfAyahs"`
}

// SurahAudio groups the audio URLs for the ayahs of a surah.
type SurahAudio struct {
	Ayahs []struct {
		NumberInSurah int    `json:"numberInSurah"`
		Audio         string `json:"audio"`
	} `json:"ayahs"`
}

// Runtime ties configuration to the running peace instance.
type Runtime struct {
	Config Config
}
