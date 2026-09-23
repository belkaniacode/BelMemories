[← Настройки и файлы](configuration.md) · [Назад к README](../README.md) · [Архитектура →](architecture.md)

# Сборка

## Зависимости (Arch Linux)

```bash
sudo pacman -S --needed go nodejs npm webkit2gtk-4.1 unzip zip
go install github.com/wailsapp/wails/v2/cmd/wails@latest   # CLI Wails v2
go install github.com/go-task/task/v3/cmd/task@latest      # необязательно: Taskfile
# для сборки Windows .exe на Linux:
sudo pacman -S --needed mingw-w64-gcc
# необязательно: даты для AVI/MKV/MTS
sudo pacman -S --needed ffmpeg
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
| `task build:linux` | собрать `build/bin/MemoryArchive` |
| `task build:windows` | собрать `build/bin/MemoryArchive.exe` (нужен `mingw-w64-gcc`) |
| `task package:linux` / `package:windows` | собрать готовую папку и zip в `dist/` |

Без `task`: `wails build -platform linux/amd64 -tags webkit2_41`.

## Почему для Windows нужен mingw

Нейросеть подключается через [`onnxruntime_go`](https://github.com/yalue/onnxruntime_go), который использует cgo (библиотека загружается во время работы, при сборке нужны только заголовки). Поэтому кросс-сборка требует C-компилятора для Windows:

```bash
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc wails build -platform windows/amd64
```

Альтернатива — собирать прямо на Windows (Go + Node.js + [MSYS2 mingw-w64](https://www.msys2.org/)).

## Модель CLIP

Программа использует image-encoder `openai/clip-vit-base-patch32` в формате ONNX (int8, ~89 МБ). Текстовые описания категорий переведены в векторы заранее, поэтому токенизатор в программе не нужен.

```bash
cd tools/clip-export
python -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt        # torch, transformers, onnx, onnxruntime
python export.py                        # → models/clip-image.onnx, models/clip-labels.json
```

Категории и подсказки задаются в списке `LABELS` в `export.py` (группа `photo` → «Фото», `picture` → «Картинки»). После правки запустите экспорт снова.

Где программа ищет файлы:

- модель: папка из настроек → `<папка программы>/models` → `./models` → `<конфиг>/MemoryArchive/models`;
- `onnxruntime`: рядом с программой → `models/lib/<os>/` → `models/lib/` → системные пути (`/usr/lib`).

Версия `onnxruntime` должна поддерживать C API ≥ 29 (1.29+). Другую версию можно скачать так: `ORT_VERSION=1.30.0 scripts/fetch-onnxruntime.sh`.

## Тесты

```bash
go test -race ./internal/...
MEMORYARCHIVE_MODEL_DIR=$PWD/models go test -run TestClipModel -v ./internal/classify/   # проверка нейросети
```

Интеграционные тесты (`internal/archiver`) создают во временных папках «грязный» источник и проверяют раскладку по годам, пропуск дублей, переименование, пробный запуск, отмену, паузу и дедупликацию между архивами.

## См. также

- [Настройки и файлы](configuration.md): где программа ищет модель и `onnxruntime`
- [Архитектура](architecture.md): зачем нужен cgo и как устроен классификатор
