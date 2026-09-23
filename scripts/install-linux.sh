#!/usr/bin/env bash
# Registers BelMemories in the desktop menu with its icon (per user, no root).
# Usage: scripts/install-linux.sh [path/to/BelMemories]   (default: build/bin/BelMemories)
# For a packaged build run it from the dist folder: ./install-linux.sh ./BelMemories
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
# Assets live in build/linux in the repo, next to the script in a dist package.
assets="$here/../build/linux"
[ -d "$assets/icons" ] || assets="$here/linux"

exe="$(realpath "${1:-$here/../build/bin/BelMemories}")"
[ -x "$exe" ] || { echo "not an executable: $exe" >&2; exit 1; }

data="${XDG_DATA_HOME:-$HOME/.local/share}"
mkdir -p "$data/applications"
cp -r "$assets/icons/." "$data/icons/"
sed "s|@EXEC@|\"$exe\"|" "$assets/belmemories.desktop" > "$data/applications/belmemories.desktop"

gtk-update-icon-cache -q -t "$data/icons/hicolor" 2>/dev/null || true
update-desktop-database -q "$data/applications" 2>/dev/null || true
command -v kbuildsycoca6 >/dev/null && kbuildsycoca6 >/dev/null 2>&1 || true

echo "installed: $data/applications/belmemories.desktop -> $exe"
