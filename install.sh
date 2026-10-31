#!/usr/bin/env bash
set -euo pipefail

PLUGIN_DIR="$(dirname "$(readlink -f "$0")")"
BIN_DIR="$HOME/.local/bin"

if ! command -v mpv >/dev/null 2>&1; then
  echo "peace: listening unavailable; install mpv with: omarchy pkg add mpv" >&2
fi
if ! command -v socat >/dev/null 2>&1; then
  echo "peace: warning: socat is missing; pause, status, and wallpaper follow are unavailable" >&2
fi
for dependency in curl jq; do
  if ! command -v "$dependency" >/dev/null 2>&1; then
    echo "peace: missing $dependency; install with: omarchy pkg add $dependency" >&2
    exit 1
  fi
done

mkdir -p "$BIN_DIR"
for command_path in "$PLUGIN_DIR"/bin/peace "$PLUGIN_DIR"/bin/peace-*; do
  [[ -x $command_path ]] || continue
  [[ $(basename "$command_path") == peace-wallpaper ]] && continue
  ln -sfn "$command_path" "$BIN_DIR/$(basename "$command_path")"
done
echo "Installed peace commands in $BIN_DIR"
