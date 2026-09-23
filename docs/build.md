# Building

**English** · [Русский](ru/build.md)

[← Configuration and files](configuration.md) · [Back to README](../README.md) · [Architecture →](architecture.md)

## Dependencies (Arch Linux)

```bash
sudo pacman -S --needed go nodejs npm webkit2gtk-4.1 unzip zip
go install github.com/wailsapp/wails/v2/cmd/wails@latest   # Wails v2 CLI
go install github.com/go-task/task/v3/cmd/task@latest      # optional: Taskfile runner
# to build the Windows .exe on Linux:
sudo pacman -S --needed mingw-w64-gcc
# optional: capture dates for AVI/MKV/MTS
sudo pacman -S --needed ffmpeg
# optional: regenerating icons (task icons)
sudo pacman -S --needed python-pillow
```

- Go ≥ 1.25 (`Taskfile.yml` sets `GOTOOLCHAIN=local`, so newer Go toolchains are not downloaded).
- Node.js ≥ 18 (the frontend is built with Vite 6).
- WebKitGTK: the build uses `webkit2gtk-4.1` by default (tag `webkit2_41`). For the older `webkit2gtk` (API 4.0) set `WEBKIT_TAGS=""`.

> **A linker error `undefined reference to uloc_…_78`** means `webkit2gtk` was built against a different ICU version than the one installed (a partial system upgrade). Run `sudo pacman -Syu` or install `webkit2gtk-4.1`.

## Commands

| Command | What it does |
|---|---|
| `task dev` | run in development mode with hot reload |
| `task test` | all Go tests with the race detector |
| `task lint` | `go vet` + `svelte-check` |
| `task models:fetch` | download `onnxruntime` 1.29.1 for Linux and Windows into `models/lib/` |
| `task models:export` | export the CLIP model into `models/` |
| `task build:linux` | build `build/bin/BelMemories` |
| `task build:windows` | build `build/bin/BelMemories.exe` (needs `mingw-w64-gcc`) |
| `task package:linux` / `package:windows` | assemble a ready-to-ship folder and zip in `dist/` |
| `task install:linux` | build and add the app to the Linux menu with its icon (`~/.local/share`, no root) |
| `task icons -- icon.png` | regenerate all icons from one PNG |

Without `task`: `wails build -platform linux/amd64 -tags webkit2_41`.

## Building for Windows

The neural network is loaded through [`onnxruntime_go`](https://github.com/yalue/onnxruntime_go), which uses cgo (the library itself is loaded at runtime; only its headers are needed at build time). Cross-compiling therefore requires a C compiler that targets Windows.

**mingw-w64** (what `task build:windows` uses):

```bash
sudo pacman -S --needed mingw-w64-gcc
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc wails build -platform windows/amd64
```

**Alternative without root: Zig** as the C compiler:

```bash
CC="zig cc -target x86_64-windows-gnu" CXX="zig c++ -target x86_64-windows-gnu" CGO_ENABLED=1 \
  wails build -platform windows/amd64 -ldflags "-extldflags=-Wl,--subsystem,windows"
```

The `-extldflags` part matters: `zig cc` ignores the `-mwindows` flag Go passes for GUI apps, and without it the `.exe` opens an extra console window. Check with `file build/bin/BelMemories.exe` — it must say `(GUI)`. Zig is also available without a system install from PyPI (`ziglang` wheel).

You can also build natively on Windows (Go + Node.js + [MSYS2 mingw-w64](https://www.msys2.org/)).

## Icons and Linux desktop integration

All icons are generated from a single square PNG with a transparent background:

```bash
task icons -- icon.png        # or: python tools/make-icons.py icon.png
```

`tools/make-icons.py` (needs Python and Pillow) writes:

- `build/appicon.png` (1024 px, used by `wails build`);
- `build/windows/icon.ico` (16–256 px, Windows executable and taskbar);
- `build/linux/icons/hicolor/NxN/apps/belmemories.png` (Linux menu and taskbar);
- `frontend/src/assets/logo.png` (256 px, in-app logo).

On Linux, `task install:linux` builds the app and runs `scripts/install-linux.sh`, which registers it in the desktop menu. The script needs no root: it copies a `.desktop` entry to `~/.local/share/applications` and the icons to `~/.local/share/icons/hicolor`, pointing the `.desktop` entry at the built executable. On Wayland this is required for the taskbar to show the app icon. The script can also be run on its own: `scripts/install-linux.sh [path/to/BelMemories]`.

`task package:linux` ships `install-linux.sh` and the icons (`linux/`) in `dist/BelMemories-linux/`, so in a packaged build you just run `./install-linux.sh ./BelMemories`.

## CLIP model

The app uses the `openai/clip-vit-base-patch32` image encoder in ONNX format (int8, ~89 MB). The text prompts for each category are turned into vectors ahead of time, so the app does not need a tokenizer.

```bash
cd tools/clip-export
python -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt        # torch, transformers, onnx, onnxruntime
python export.py                        # → models/clip-image.onnx, models/clip-labels.json
```

Categories and prompts are defined in the `LABELS` list in `export.py` (group `photo` → photos, `picture` → pictures). Labels in `HUMAN` mean people are in the shot: such images stay in photos unless the network is at least 90% sure it is a picture. After editing prompts, only the labels need to be recomputed; the image model stays the same:

```bash
python export.py --labels-only          # → models/clip-labels.json
```

If the export fails with `broken text embedding`, the weight cache in `~/.cache/huggingface` is corrupt: delete it and run again.

Where the app looks for files:

- model: folder from the settings → `<app folder>/models` → `./models` → `<config>/BelMemories/models`;
- `onnxruntime`: next to the executable → `models/lib/<os>/` → `models/lib/` → system paths (`/usr/lib`).

The `onnxruntime` version must support C API ≥ 29 (1.29+). To download a different version: `ORT_VERSION=1.30.0 scripts/fetch-onnxruntime.sh`.

## Tests

```bash
go test -race ./internal/...
BELMEMORIES_MODEL_DIR=$PWD/models go test -run TestClipModel -v ./internal/classify/   # neural network check
```

The integration tests (`internal/archiver`) build a messy source in temporary folders and check sorting by year, duplicate skipping, renaming, dry runs, cancellation, pausing and cross-archive deduplication.

## See also

- [Configuration and files](configuration.md): where the app looks for the model and `onnxruntime`
- [Architecture](architecture.md): why cgo is needed and how the classifier works
