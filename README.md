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

    peace listen SURAH[:AYAH|:START-END|:START-] [--follow] [--random] [--reciter ID] [--buffer N] [--lead N] [--jobs N]
    peace pause
    peace stop
    peace status
    peace wallpaper [random|next|prev] [--random]

    peace listen 55
    peace listen 55:5-10 --follow --buffer 6 --lead 2 --jobs 2
    peace listen 110 --follow --random --reciter ar.alafasy
    peace wallpaper --random
    peace wallpaper next
    peace wallpaper prev
    peace pause
    peace stop
    peace status

Follow works with any active Omarchy theme. It starts audio immediately, then caches upcoming audio and renders upcoming ayah wallpapers in parallel within the configured buffer. If an audio cache is late, playback streams that ayah while caching continues. Wallpapers use the current background and active theme colors by default. Pass `--random` with `--follow` or `peace wallpaper --random` to choose a random background from the active theme, Peace theme, and bundled backgrounds. Audio text and recitation URLs come from AlQuran Cloud. Listening requires mpv.

## Build and install

The repository ships a statically linked Linux x86_64 Go command at `bin/peace`; users do not need Go, `curl`, `jq`, `socat`, ImageMagick, Pango, or system fonts. To rebuild it from source on Linux x86_64:

```sh
make build
make install
make clean
make uninstall
```

Audio playback uses `mpv`, and wallpaper changes use Omarchy's wallpaper command. Text shaping, font data, image decoding, and wallpaper composition are built into the command.

