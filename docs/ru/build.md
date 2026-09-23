# Сборка

[English](../build.md) · **Русский**

[← Настройки и файлы](configuration.md) · [Назад к README](../../README.ru.md) · [Архитектура →](architecture.md)

## Зависимости (Arch Linux)

```bash
sudo pacman -S --needed go nodejs npm webkit2gtk-4.1 unzip zip
go install github.com/wailsapp/wails/v2/cmd/wails@latest   # CLI Wails v2
go install github.com/go-task/task/v3/cmd/task@latest      # необязательно: Taskfile
# для сборки Windows .exe на Linux:
sudo pacman -S --needed mingw-w64-gcc
# необязательно: даты для AVI/MKV/MTS
sudo pacman -S --needed ffmpeg
# необязательно: пересоздание иконок (task icons)
sudo pacman -S --needed python-pillow
```

- Go ≥ 1.25 (в `Taskfile.yml` включён `GOTOOLCHAIN=local`, новые версии Go не скачиваются).
- Node.js ≥ 18 (фронтенд собирается Vite 6).
- WebKitGTK: по умолчанию сборка использует `webkit2gtk-4.1` (тег `webkit2_41`). Для старого `webkit2gtk` (API 4.0) задайте `WEBKIT_TAGS=""`.

> **Ошибка линковки `undefined reference to uloc_…_78`** означает, что `webkit2gtk` собран под другую версию ICU, чем установлена (система обновлена частично). Выполните `sudo pacman -Syu` или установите `webkit2gtk-4.1`.

## Команды

| Команда | Что делает |
|---|---|
| `task dev` | запуск в режиме разработки с горячей перезагрузкой |
| `task test` | все Go-тесты с детектором гонок |
| `task lint` | `go vet` + `svelte-check` |
| `task models:fetch` | скачать `onnxruntime` 1.29.1 для Linux и Windows в `models/lib/` |
| `task models:export` | экспортировать модель CLIP в `models/` |
| `task build:linux` | собрать `build/bin/BelMemories` |
| `task build:windows` | собрать `build/bin/BelMemories.exe` (нужен `mingw-w64-gcc`) |
| `task package:linux` / `package:windows` | собрать готовую папку и zip в `dist/` |
| `task install:linux` | собрать и добавить программу в меню Linux с иконкой (`~/.local/share`, без root) |
| `task icons -- icon.png` | пересоздать все иконки из одного PNG |

Без `task`: `wails build -platform linux/amd64 -tags webkit2_41`.

## Сборка для Windows

Нейросеть подключается через [`onnxruntime_go`](https://github.com/yalue/onnxruntime_go), который использует cgo (библиотека загружается во время работы, при сборке нужны только заголовки). Поэтому кросс-сборка требует C-компилятора для Windows.

**mingw-w64** (так работает `task build:windows`):

```bash
sudo pacman -S --needed mingw-w64-gcc
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc wails build -platform windows/amd64
```

**Альтернатива без root — Zig** в роли C-компилятора:

```bash
CC="zig cc -target x86_64-windows-gnu" CXX="zig c++ -target x86_64-windows-gnu" CGO_ENABLED=1 \
  wails build -platform windows/amd64 -ldflags "-extldflags=-Wl,--subsystem,windows"
```

Часть `-extldflags` обязательна: `zig cc` игнорирует флаг `-mwindows`, который Go передаёт для оконных программ, и без неё `.exe` открывает лишнее окно консоли. Проверка: `file build/bin/BelMemories.exe` должна показать `(GUI)`. Zig можно взять и без установки в систему — из PyPI (пакет `ziglang`).

Ещё один вариант — собирать прямо на Windows (Go + Node.js + [MSYS2 mingw-w64](https://www.msys2.org/)).

## Версии

У версии один источник — `info.productVersion` в `wails.json` (`1.1.0`). Программа встраивает этот файл, поэтому «О программе», журнал и свойства `.exe` в Windows всегда показывают одну и ту же версию — и в `wails dev` тоже, без `-ldflags`.

Выпуски нумеруются по семантическим версиям: `1.1.0` → `1.1.1` для исправлений, `1.2.0` для новых возможностей, `2.0.0` для несовместимых изменений:

```bash
task version              # показать текущую версию
task release -- 1.2.0     # записать в wails.json, закоммитить и поставить тег v1.2.0
git push --follow-tags    # отправить коммит и тег
```

## Иконки и интеграция с Linux

Все иконки создаются из одного квадратного PNG с прозрачным фоном:

```bash
task icons -- icon.png        # или: python tools/make-icons.py icon.png
```

Скрипт `tools/make-icons.py` (нужны Python и Pillow) записывает:

- `build/appicon.png` (1024 px, для `wails build`);
- `build/windows/icon.ico` (16–256 px, exe и панель задач Windows);
- `build/linux/icons/hicolor/NxN/apps/belmemories.png` (меню и панель задач Linux);
- `frontend/src/assets/logo.png` (256 px, логотип в интерфейсе).

На Linux программа регистрируется в меню командой `task install:linux` (собирает программу и запускает `scripts/install-linux.sh`). Скрипт работает без root: копирует `.desktop`-файл в `~/.local/share/applications` и иконки в `~/.local/share/icons/hicolor`, указывая в `.desktop` путь к собранному файлу. На Wayland это нужно, чтобы на панели задач показывалась иконка программы. Скрипт можно запустить и отдельно: `scripts/install-linux.sh [путь/к/BelMemories]`.

`task package:linux` кладёт в `dist/BelMemories-linux/` скрипт `install-linux.sh` и иконки (`linux/`), так что в готовом пакете достаточно выполнить `./install-linux.sh ./BelMemories`.

## Модель CLIP

Программа использует image-encoder `openai/clip-vit-base-patch32` в формате ONNX (int8, ~89 МБ). Текстовые описания категорий переведены в векторы заранее, поэтому токенизатор в программе не нужен.

```bash
cd tools/clip-export
python -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt        # torch, transformers, onnx, onnxruntime
python export.py                        # → models/clip-image.onnx, models/clip-labels.json
```

Категории и подсказки задаются в списке `LABELS` в `export.py` (группа `photo` → «Фото», `picture` → «Картинки»). Метки из `HUMAN` означают людей на снимке: такие изображения остаются в «Фото», пока нейросеть не уверена в «картинке» на 90% и больше. После правки подсказок достаточно пересчитать только метки, модель изображений не меняется:

```bash
python export.py --labels-only          # → models/clip-labels.json
```

Если экспорт падает с `broken text embedding`, повреждён кэш весов в `~/.cache/huggingface`: удалите его и запустите снова.

Где программа ищет файлы:

- модель: папка из настроек → `<папка программы>/models` → `./models` → `<конфиг>/BelMemories/models`;
- `onnxruntime`: рядом с программой → `models/lib/<os>/` → `models/lib/` → системные пути (`/usr/lib`).

Версия `onnxruntime` должна поддерживать C API ≥ 29 (1.29+). Другую версию можно скачать так: `ORT_VERSION=1.30.0 scripts/fetch-onnxruntime.sh`.

## Тесты

```bash
go test -race ./internal/...
BELMEMORIES_MODEL_DIR=$PWD/models go test -run TestClipModel -v ./internal/classify/   # проверка нейросети
```

Интеграционные тесты (`internal/archiver`) создают во временных папках «грязный» источник и проверяют раскладку по годам, пропуск дублей, переименование, пробный запуск, отмену, паузу и дедупликацию между архивами.

## См. также

- [Настройки и файлы](configuration.md): где программа ищет модель и `onnxruntime`
- [Архитектура](architecture.md): зачем нужен cgo и как устроен классификатор
