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

# CLIP model + onnxruntime into the per-user data dir: the menu entry starts
# the app with $HOME as working directory, so a models/ folder next to the
# repo or the binary is not enough. Sources: repo (models/, models/lib/linux)
# or a dist folder (models/ and libonnxruntime.so next to the binary).
models_dst="$data/BelMemories/models"
for src in "$here/../models" "$here/models" "$(dirname "$exe")/models"; do
  if [ -f "$src/clip-image.onnx" ]; then
    mkdir -p "$models_dst/lib/linux"
    cp -f "$src/clip-image.onnx" "$src/clip-labels.json" "$models_dst/"
    for lib in "$src/lib/linux/libonnxruntime.so" "$(dirname "$exe")/libonnxruntime.so"; do
      [ -f "$lib" ] && cp -f "$lib" "$models_dst/lib/linux/" && break
    done
    echo "model:     $models_dst"
    break
  fi
done
[ -f "$models_dst/clip-image.onnx" ] || echo "note: CLIP model not found — the app will use rules only (see docs/build.md)"

gtk-update-icon-cache -q -t "$data/icons/hicolor" 2>/dev/null || true
update-desktop-database -q "$data/applications" 2>/dev/null || true
command -v kbuildsycoca6 >/dev/null && kbuildsycoca6 >/dev/null 2>&1 || true

echo "installed: $data/applications/belmemories.desktop -> $exe"
