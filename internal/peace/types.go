package peace

import (
	"encoding/json"
	"time"
)

type Config struct {
	Home      string
	PluginDir string
	StateDir  string
	CacheDir  string
	Socket    string
}

type Options struct {
	Reference string
	Reciter   string
	Follow    bool
	Buffer    int
	Lead      int
	Jobs      int
}

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

type APIResponse struct {
	Data json.RawMessage `json:"data"`
}

type Surah struct {
	Number        int `json:"number"`
	NumberOfAyahs int `json:"numberOfAyahs"`
}

type SurahAudio struct {
	Ayahs []struct {
		NumberInSurah int    `json:"numberInSurah"`
		Audio         string `json:"audio"`
	} `json:"ayahs"`
}

type Preparation struct {
	Ayah int
	PID  int
}

type Runtime struct {
	Config Config
	Now    func() time.Time
}
