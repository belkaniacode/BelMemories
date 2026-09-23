#!/usr/bin/env bash
# Downloads the ONNX Runtime shared library for Linux and/or Windows into
# models/lib/<os>/ so it can be packaged next to the executable.
#
# Usage: scripts/fetch-onnxruntime.sh [linux|windows|all]
# The version must provide C API >= 29 (required by onnxruntime_go v1.36).
set -euo pipefail

ORT_VERSION="${ORT_VERSION:-1.29.1}"
TARGET="${1:-all}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/models/lib"
BASE="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

fetch_linux() {
  local name="onnxruntime-linux-x64-${ORT_VERSION}"
  echo "→ $name.tgz"
  curl -fL --retry 3 -o "$TMP/$name.tgz" "$BASE/$name.tgz"
  tar -xzf "$TMP/$name.tgz" -C "$TMP"
  mkdir -p "$OUT/linux"
  # Keep the real versioned file (not the symlink) and name it plainly.
  cp -L "$TMP/$name/lib/libonnxruntime.so.${ORT_VERSION}" "$OUT/linux/libonnxruntime.so"
  echo "   saved $OUT/linux/libonnxruntime.so"
}

fetch_windows() {
  local name="onnxruntime-win-x64-${ORT_VERSION}"
  echo "→ $name.zip"
  curl -fL --retry 3 -o "$TMP/$name.zip" "$BASE/$name.zip"
  (cd "$TMP" && unzip -q "$name.zip")
  mkdir -p "$OUT/windows"
  cp "$TMP/$name/lib/onnxruntime.dll" "$OUT/windows/onnxruntime.dll"
  echo "   saved $OUT/windows/onnxruntime.dll"
}

case "$TARGET" in
  linux) fetch_linux ;;
  windows) fetch_windows ;;
  all) fetch_linux; fetch_windows ;;
  *) echo "usage: $0 [linux|windows|all]" >&2; exit 2 ;;
esac
