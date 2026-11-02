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

    peace listen SURAH[:AYAH|:START-END|:START-] [--basmala] [--follow] [--random] [--use-theme] [--show-translation=false] [--show-reference=false] [--reciter ID|NUMBER] [--buffer N] [--lead N] [--jobs N] [--source quran|alquran] [--fallback alquran] [--translation ID]
    peace words SURAH:AYAH [--source quran|alquran]
    peace reciters [--lang en|ar|ur|...]
    peace translations [--lang en|ar|ur|...]
    peace pause
    peace stop
    peace status
    peace wallpaper [random|next|prev] [--random] [--use-theme] [--show-translation=false] [--show-reference=false] [--source quran|alquran] [--translation ID]

    peace listen 55
    peace listen 55:5-10 --follow --buffer 6 --lead 2 --jobs 2
    peace listen 110 --follow --random --basmala --reciter ar.alafasy
    peace listen 2:255 --source quran --translation en.sahih
    peace words 2:255
    peace reciters
    peace translations --lang en
    peace wallpaper --random
    peace wallpaper next
    peace wallpaper prev
    peace pause
    peace stop
    peace status

Follow works with any active Omarchy theme. It starts audio immediately, then caches upcoming audio and renders upcoming ayah wallpapers in parallel within the configured buffer. If an audio cache is late, playback streams that ayah while caching continues. Translation and reference text are visible by default and can be hidden with `--show-translation=false` and `--show-reference=false`. Text colors are adapted to each wallpaper by default; pass `--use-theme` to use the active Omarchy theme's text colors instead. Pass `--random` with `--follow` to shuffle through the available active theme, Peace theme, and bundled backgrounds between ayahs. Use `peace wallpaper --random` to choose one random background for a manual wallpaper. Listening requires mpv.

Quran text, translations, words, and recitation URLs come from the Quran.com API v4 by default (`--source quran`, also available as `PEACE_SOURCE=quran`). Pass `--source alquran` to use the previous AlQuran Cloud provider. Add `--fallback alquran` (or `PEACE_FALLBACK_SOURCE=alquran`) to retry AlQuran Cloud when the primary source fails. Existing AlQuran Cloud identifiers such as `--reciter ar.alafasy` and `--translation en.sahih` keep working: they are resolved against the live Quran.com resource lists, and `peace reciters` / `peace translations` print the numeric IDs you can use instead. `--translation` defaults to `en.sahih` (`PEACE_TRANSLATION` overrides it).

`peace words 2:255` prints each word with Arabic text, transliteration, and English meaning. Word-by-word data is only available from the Quran.com source. Surah text and word data are cached as one JSON file per surah and translation in the peace cache directory, so repeated playback does not refetch them; audio prefetching and caching are unchanged. The open Quran.com endpoint may require Quran Foundation credentials in the future; set `QURAN_FOUNDATION_TOKEN` to send a bearer token to Quran.com hosts. The token is never sent to AlQuran Cloud.

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

