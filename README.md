# BelMemories

**English** · [Русский](README.ru.md)

> One tidy photo and video archive, sorted by year — gathered from all your drives, without duplicates and without putting your data at risk.

A desktop app for Linux and Windows. It scans drives and folders (including phone backups), picks out only photos and videos, and files them onto an archive drive by year. Logos, screenshots and memes go to a separate "pictures" folder.

## Quick start

- **Windows 10/11**: unpack the release archive and run `BelMemories.exe`.
- **Arch Linux**:

```bash
sudo pacman -S --needed webkit2gtk-4.1
./BelMemories
./install-linux.sh ./BelMemories   # optional: menu entry and taskbar icon
```

To build from source, see [docs/build.md](docs/build.md).

## Features

- **Photos and videos only**: JPEG, PNG, HEIC, WebP, RAW, MP4, MOV, AVI, MKV and more; everything else is skipped.
- **Year from the capture date**: EXIF → video metadata → file name (`IMG_20190512_…`, WhatsApp, Telegram) → modification time.
- **No duplicates**: files are compared by content (BLAKE3 hash), also against every archive the app has written to before, as long as its drive is connected. Same name but different content is saved as `name_1.jpg`. A file you deleted from the archive by hand is copied again on the next run.
- **Photo or picture**: rules plus the CLIP neural network, fully offline. Shots with people stay in photos even if they look like a screenshot or a repost.
- **Stays out of your way**: low priority, three load levels, pause.
- **Safe for the archive**: existing files are never overwritten, every copy is verified, and a crash cannot corrupt the archive.
- **English and Russian UI**: the language follows the operating system.
- **Light and dark theme**: taken from the OS on first start, then switched with a button. Includes a built-in help page and an About dialog.

## Archive layout

```
<Archive drive>/
├── Фото/2019/…        Photos: people, animals, nature, everyday life
├── Видео/2019/…       Videos
├── Картинки/2021/…    Pictures: logos, icons, screenshots, memes, documents
├── Без даты/{Фото,Видео,Картинки}/…   No date
└── .memoryarchive/    index, journal, reports (internal folder)
```

For now the archive folder names are always in Russian, whatever the UI language.

## How it works

1. **Source**: pick one or more drives or folders and click Scan. The last three sources are offered for quick selection.
2. **Scan**: the app shows how many photos and videos it found and how much space they take.
3. **Destination**: choose the archive drive and the load level. The app shows free space, how many files and bytes will be copied, and how many files are already in the folder. Optionally enable a dry run: nothing is copied, and the plan screen offers an "Archive now" button to carry out that plan right away.
4. **Archiving**: progress, speed and time remaining. You can pause or stop.
5. **Report**: files copied, duplicates skipped, files renamed, files without a date. The full list is saved as CSV. "New archive" starts over from scratch.

## Data safety

- Sources are **read-only**: nothing in them is changed or deleted.
- Existing archive files are **never overwritten**.
- Each file is written to a temporary `.part` file, flushed to disk and checked against its hash before it appears in the archive.
- At least 1 GiB is always kept free on the archive drive. If space runs out or the disk reports an error, the run stops immediately.
- After a failure, just run it again: files already copied will not be duplicated.

---

## Documentation

| Document | Contents |
|---|---|
| [Configuration and files](docs/configuration.md) | Settings, theme and language, load levels, where logs, the index and reports live |
| [Building](docs/build.md) | Building for Linux and Windows, icons and desktop integration, the CLIP model, tests |
| [Architecture](docs/architecture.md) | Packages, pipeline, safe writes, index, cross-archive duplicates, classification |

## Author

Belkania Z. · [belteosystems@gmail.com](mailto:belteosystems@gmail.com)
