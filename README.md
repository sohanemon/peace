# Omapeace

An Omarchy plugin for Quran recitation and ayah wallpaper follow. The plugin owns the listening commands and follow renderer. Follow can use the current background and colors from any active Omarchy theme.

## Develop locally

Copy the plugin into Omarchy's plugin directory, validate it, and install the peace commands:

    mkdir -p ~/.config/omarchy/plugins/io.github.sohanemon.omapeace
    cp -a "$HOME/Code/omapeace/." ~/.config/omarchy/plugins/io.github.sohanemon.omapeace/
    omarchy plugin validate ~/.config/omarchy/plugins/io.github.sohanemon.omapeace
    qmllint -I "$OMARCHY_PATH/shell" ~/.config/omarchy/plugins/io.github.sohanemon.omapeace/BarWidget.qml ~/.config/omarchy/plugins/io.github.sohanemon.omapeace/Panel.qml
    ~/.config/omarchy/plugins/io.github.sohanemon.omapeace/bin/peace install
    omarchy plugin enable io.github.sohanemon.omapeace

For changes during development, run the checks against the source folder:

    omarchy plugin validate "$HOME/Code/omapeace"
    qmllint -I "$OMARCHY_PATH/shell" "$HOME/Code/omapeace/BarWidget.qml" "$HOME/Code/omapeace/Panel.qml"

The Go command links the packaged Omapeace executable into ~/.local/bin. It does not install or replace the Peace theme.

The repository ships a statically linked Linux x86_64 Go command at `bin/peace`; users do not need Go, `curl`, `jq`, `socat`, ImageMagick, Pango, or system fonts. To rebuild it from source on Linux x86_64, run `CGO_ENABLED=0 go build -o bin/peace ./cmd/peace`. Audio playback uses `mpv`, and wallpaper changes use Omarchy's wallpaper command. Text shaping, font data, image decoding, and wallpaper composition are built into the command.

## Commands

    peace listen 55
    peace listen 55:5-10 --follow --buffer 6 --lead 2 --jobs 2
    peace pause
    peace stop
    peace status
    peace wallpaper next

Follow works with any active Omarchy theme. It starts audio immediately, then caches upcoming audio and renders upcoming ayah wallpapers in parallel within the configured buffer. If an audio cache is late, playback streams that ayah while caching continues. Wallpapers use the current background and active theme colors, with bundled artwork in `assets/backgrounds` as fallback. Audio text and recitation URLs come from AlQuran Cloud. Listening requires mpv.

## Remove

    rm -f ~/.local/bin/peace ~/.local/bin/peace-listen ~/.local/bin/peace-pause ~/.local/bin/peace-stop ~/.local/bin/peace-status
    omarchy plugin disable io.github.sohanemon.omapeace
