# Omapeace

Quran recitation and ayah wallpaper follow. The plugin owns the listening commands and follow renderer. Follow can use the current background and colors from any active Omarchy theme.

## Install in Omarchy

Install the plugin from its GitHub repository, then enable its bar widget:

```sh
omarchy plugin add https://github.com/sohanemon/peace --enable
```

The bar widget runs the compiled binary directly; no extra setup is needed to use it. The binary is at `~/.config/omarchy/plugins/io.github.sohanemon.omapeace/bin/peace` after installation.

Remove the plugin from Omarchy with:

```sh
omarchy plugin remove io.github.sohanemon.omapeace
```

## Commands

    peace listen SURAH[:AYAH|:START-END|:START-] [--basmala] [--follow] [--random] [--use-theme] [--show-translation=false] [--show-reference=false] [--reciter ID] [--buffer N] [--lead N] [--jobs N] [--translation ID]
    peace words SURAH:AYAH [--translation ID]
    peace reciters [--lang en|ar|ur|...]
    peace translations [--lang en|ar|ur|...]
    peace pause
    peace stop
    peace status
    peace wallpaper [random|next|prev] [--random] [--use-theme] [--show-translation=false] [--show-reference=false] [--translation ID]
    peace settings [--translation ID] [--wbw-language CODE] [--json]
    peace config path
    peace config get KEY
    peace config set KEY VALUE
    peace config edit

    peace listen 55
    peace listen 55:5-10 --follow --buffer 6 --lead 2 --jobs 2
    peace listen 110 --follow --random --basmala --reciter 7
    peace listen 2:255 --translation 20
    peace words 2:255
    peace reciters
    peace translations --lang en
    peace wallpaper --random
    peace wallpaper next
    peace wallpaper prev
    peace pause
    peace stop
    peace status
    peace config path
    peace config set translation 131
    peace config set wbw_enabled true
    peace config edit

## Settings

Persistent settings live in `$XDG_CONFIG_HOME/peace/config.toml`, which defaults to `~/.config/peace/config.toml`. The path is resolved with `os.UserConfigDir`, so `XDG_CONFIG_HOME` is respected. Run `peace config path` to print it.

```toml
# Peace user settings.
# Precedence: CLI flags > PEACE_* environment variables > this file > built-in defaults.

# Configuration schema version. Do not edit.
version = 1

# Quran.com numeric reciter ID. List them with `peace reciters`.
reciter = "7"

# Quran.com numeric translation ID. List them with `peace translations`.
translation = "20"

# Show the ayah translation on wallpapers by default.
translation_enabled = true

# Show word-by-word meanings on wallpapers by default.
wbw_enabled = false

# Word-by-word meaning language (for example en, ur, bn).
wbw_language = "en"

# Show word-by-word transliteration on wallpapers by default.
wbw_transliteration = false
```

A missing file or missing keys use the built-in defaults. Unknown keys are ignored. A malformed file returns an error with the file path and line and is never overwritten. Writes use a same-directory temporary file, `fsync`, and `rename`, with a process-wide mutex; unchanged values do not rewrite the file. The directory is created with mode `0700` and the file with mode `0600`.

Keys accepted by `peace config get` and `peace config set` are `version` (read-only), `reciter`, `translation`, `translation_enabled`, `wbw_enabled`, `wbw_language`, and `wbw_transliteration`.

Precedence is command flags first, then `PEACE_*` environment variables, then `config.toml`, then defaults. String overrides are `PEACE_RECITER`, `PEACE_TRANSLATION`, and `PEACE_WBW_LANGUAGE`; boolean overrides are `PEACE_TRANSLATION_ENABLED`, `PEACE_WBW_ENABLED`, and `PEACE_WBW_TRANSLITERATION`.

`config.toml` is the only settings store. The legacy `peace settings` command still works and now reads and writes `config.toml`.

Cache data stays under `os.UserCacheDir()/peace` (`~/.cache/peace` by default). Transient playback state stays under `$XDG_STATE_HOME/peace`, falling back to `~/.local/state/peace`; runtime state is not stored in `config.toml`.

Follow works with any active Omarchy theme. It starts audio immediately, then caches upcoming audio and renders upcoming ayah wallpapers in parallel within the configured buffer. If an audio cache is late, playback streams that ayah while caching continues. Translation and reference text are visible by default and can be hidden with `--show-translation=false` and `--show-reference=false`. Text colors are adapted to each wallpaper by default; pass `--use-theme` to use the active Omarchy theme's text colors instead. Pass `--random` with `--follow` to shuffle through the available active theme, Peace theme, and bundled backgrounds between ayahs. Use `peace wallpaper --random` to choose one random background for a manual wallpaper. Listening requires mpv.

Quran text, translations, words, and recitation audio come from the Quran.com API v4. Reciters and translations use numeric Quran.com resource IDs. The defaults are reciter `7` (Mishary Rashid Alafasy) and translation `20` (Saheeh International); persist different values with `peace config set`, or override them with `PEACE_RECITER` and `PEACE_TRANSLATION`. Run `peace reciters` and `peace translations` to list valid IDs, optionally filtered with `--lang`. Old identifiers such as `ar.alafasy` and `en.sahih` are migrated automatically once to their numeric IDs; unknown values fail with a list of valid options.

`peace words 2:255` prints each word with Arabic text, transliteration, and English meaning. Surah text and word data are cached as one JSON file per surah and translation in the peace cache directory, so repeated playback does not refetch them. Reciter, translation, and chapter lists use a long cache lifetime. Audio prefetching, caching, and mpv playback are unchanged.

Set `PEACE_API_BASE` to point the client at a proxy or the authenticated Quran Foundation endpoint (default `https://api.quran.com/api/v4`). Set `QURAN_FOUNDATION_TOKEN` to send a bearer token with API requests.

The introductory Basmala is hidden and not played by default. Pass `--basmala` to show it on the first ayah wallpaper and play it before that ayah. `--basmala=false` explicitly keeps it hidden and silent.

## Build and install

The repository ships a statically linked Linux x86_64 Go command at `bin/peace`; users do not need Go, `curl`, `jq`, `socat`, ImageMagick, Pango, or system fonts. To rebuild it from source on Linux x86_64:

```sh
make build
make install
make clean
make uninstall
```

Audio playback uses `mpv`, and wallpaper changes use Omarchy's wallpaper command. Text shaping, font data, image decoding, and wallpaper composition are built into the command.

