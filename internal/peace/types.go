package peace

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
	Reference   string
	Reciter     string
	Translation string
	Follow      bool
	Random      bool
	Basmala     bool
	Render      RenderOptions
	Buffer      int
	Lead        int
	Jobs        int
}

type RenderOptions struct {
	UseTheme            bool `json:"use_theme,omitempty"`
	ShowTranslation     bool `json:"show_translation"`
	ShowWordTranslation bool `json:"show_word_translation,omitempty"`
	ShowReference       bool `json:"show_reference"`
}

// PlayState represents the current playback state of a recitation session.
type PlayState struct {
	Surah         int           `json:"surah"`
	Start         int           `json:"start"`
	End           int           `json:"end"`
	Reciter       string        `json:"reciter"`
	Translation   string        `json:"translation"`
	Follow        bool          `json:"follow"`
	Random        bool          `json:"random,omitempty"`
	RandomSources []string      `json:"random_sources,omitempty"`
	Render        RenderOptions `json:"render"`
	Basmala       bool          `json:"basmala"`
	BaseImage     string        `json:"base_image,omitempty"`
	AudioURLs     []string      `json:"audio_urls,omitempty"`
	IntroAudioURL string        `json:"intro_audio_url,omitempty"`
	Buffer        int           `json:"buffer,omitempty"`
	Lead          int           `json:"lead,omitempty"`
	Jobs          int           `json:"jobs,omitempty"`
	Preparing     bool          `json:"preparing"`
	Ready         int           `json:"ready"`
	Appended      int           `json:"appended"`
}

// Runtime ties configuration to the running peace instance.
type Runtime struct {
	Config Config
}
