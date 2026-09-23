#!/usr/bin/env bash
# Packs the Linux build into one self-contained AppImage:
#   BelMemories binary + CLIP model + libonnxruntime.so + icon + .desktop.
# WebKitGTK is NOT bundled: it must be installed on the system
# (Arch: webkit2gtk-4.1, Debian/Ubuntu: libwebkit2gtk-4.1-0).
#
# Usage: scripts/build-appimage.sh [binary] [out-dir]
#   defaults: build/bin/BelMemories, dist/
# Needs appimagetool in PATH or APPIMAGETOOL=/path; it is downloaded to
# .cache/ when missing. Set APPIMAGE_EXTRACT_AND_RUN=1 where FUSE is absent (CI).
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
bin="$(realpath "${1:-$root/build/bin/BelMemories}")"
out="$(mkdir -p "${2:-$root/dist}" && realpath "${2:-$root/dist}")"
version="$(python3 -c "import json;print(json.load(open('$root/wails.json'))['info']['productVersion'])")"
[ -x "$bin" ] || { echo "no executable: $bin" >&2; exit 1; }

tool="${APPIMAGETOOL:-$(command -v appimagetool || true)}"
if [ -z "$tool" ]; then
  tool="$root/.cache/appimagetool-x86_64.AppImage"
  if [ ! -x "$tool" ]; then
    mkdir -p "$root/.cache"
    curl -fsSL --retry 3 -o "$tool" \
      https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage
    chmod +x "$tool"
  fi
fi

app="$(mktemp -d)/BelMemories.AppDir"
trap 'rm -rf "$(dirname "$app")"' EXIT
mkdir -p "$app/usr/bin/models" "$app/usr/share/applications"

cp "$bin" "$app/usr/bin/BelMemories"
# The app looks for models/ and libonnxruntime.so next to its executable.
if [ -f "$root/models/clip-image.onnx" ]; then
  cp "$root/models/clip-image.onnx" "$root/models/clip-labels.json" "$app/usr/bin/models/"
else
  echo "WARN: models/clip-image.onnx missing — AppImage will run in rules-only mode" >&2
fi
if [ -f "$root/models/lib/linux/libonnxruntime.so" ]; then
  cp "$root/models/lib/linux/libonnxruntime.so" "$app/usr/bin/"
else
  echo "WARN: models/lib/linux/libonnxruntime.so missing (task models:fetch)" >&2
fi

cp -r "$root/build/linux/icons" "$app/usr/share/"
cp "$root/build/linux/icons/hicolor/256x256/apps/belmemories.png" "$app/belmemories.png"
ln -s belmemories.png "$app/.DirIcon"
sed -e 's|^Exec=.*|Exec=BelMemories|' "$root/build/linux/belmemories.desktop" > "$app/belmemories.desktop"
cp "$app/belmemories.desktop" "$app/usr/share/applications/"

cat > "$app/AppRun" <<'RUN'
#!/bin/sh
here="$(dirname "$(readlink -f "$0")")"
exec "$here/usr/bin/BelMemories" "$@"
RUN
chmod +x "$app/AppRun"

target="$out/BelMemories-$version-x86_64.AppImage"
ARCH=x86_64 "$tool" --no-appstream "$app" "$target" >/dev/null
echo "built: $target ($(du -h "$target" | cut -f1))"
