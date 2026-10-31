# Omapeace

An Omarchy plugin for Quran recitation and ayah wallpaper follow. The plugin owns the peace listening commands and top-bar controls. The Peace theme remains responsible for rendering wallpapers.

## Develop locally

Copy the plugin into Omarchy's plugin directory, validate it, and install the peace commands:

    mkdir -p ~/.config/omarchy/plugins/io.github.sohanemon.omapeace
    cp -a "$HOME/Code/omapeace/." ~/.config/omarchy/plugins/io.github.sohanemon.omapeace/
    omarchy plugin validate ~/.config/omarchy/plugins/io.github.sohanemon.omapeace
    qmllint -I "$OMARCHY_PATH/shell" ~/.config/omarchy/plugins/io.github.sohanemon.omapeace/BarWidget.qml ~/.config/omarchy/plugins/io.github.sohanemon.omapeace/Panel.qml
    ~/.config/omarchy/plugins/io.github.sohanemon.omapeace/install.sh
    omarchy plugin enable io.github.sohanemon.omapeace

For changes during development, run the checks against the source folder:

    omarchy plugin validate "$HOME/Code/omapeace"
    qmllint -I "$OMARCHY_PATH/shell" "$HOME/Code/omapeace/BarWidget.qml" "$HOME/Code/omapeace/Panel.qml"

The installer links files named peace and peace-* into ~/.local/bin. It does not install or replace the Peace theme.

## Commands

    peace listen 55
    peace listen 55:5-10 --follow --buffer 6 --lead 2 --jobs 2
    peace pause
    peace stop
    peace status
    peace wallpaper next

The follow option requires the Peace theme to be active and installed at ~/.config/omarchy/themes/peace, or set PEACE_THEME_DIR. Audio and wallpaper preparation use AlQuran Cloud. Listening requires mpv; pause, status, and follow require socat.

## Remove

    rm -f ~/.local/bin/peace ~/.local/bin/peace-listen ~/.local/bin/peace-pause ~/.local/bin/peace-stop ~/.local/bin/peace-status
    omarchy plugin disable io.github.sohanemon.omapeace
