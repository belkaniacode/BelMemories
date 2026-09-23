# Configuration and files

**English** · [Русский](ru/configuration.md)

[Back to README](../README.md) · [Building →](build.md)

## File locations

| What | Linux | Windows |
|---|---|---|
| Settings | `~/.config/BelMemories/settings.json` | `%AppData%\BelMemories\settings.json` |
| Logs (rotated, 10 MB × 5) | `~/.config/BelMemories/logs/app.log` | `%AppData%\BelMemories\logs\app.log` |
| Dry-run reports | `~/.config/BelMemories/reports/` | `%AppData%\BelMemories\reports\` |
| Archive index | `<archive>/.memoryarchive/index.db` | same |
| Run reports (JSON + CSV) | `<archive>/.memoryarchive/reports/` | same |
| Temporary copy files | `<archive>/.memoryarchive/tmp/*.part` | same |

Do not delete the `.memoryarchive` folder: without the index the app cannot tell which files are already archived. If the index does get lost, the Destination screen offers a "Build index" button.

## settings.json

The file is written by the app itself; you normally do not need to edit it.

| Key | Default | Meaning |
|---|---|---|
| `load` | `medium` | Load level: `low`, `medium`, `high` |
| `verifyAfterCopy` | `true` | Re-read every written file and compare its hash |
| `bytesPerSecLimit` | `0` | Custom read speed limit in bytes/s; `0` uses the load level's limit |
| `clipThreshold` | `0.5` | Neural network confidence (0–1) needed to send an image to pictures. Higher is stricter |
| `modelDir` | `""` | Custom CLIP model folder; empty means auto-detect |
| `theme` | `auto` | UI theme: `light` or `dark`. `auto` means no choice has been made yet (see below) |
| `knownArchives` | `[]` | Filled automatically: archives the app has written to (newest first, at most 50). Used for duplicate detection, see below |
| `recentSources` | `[]` | The last three sources |
| `lastDestination` | `""` | The last chosen destination folder |

On load, the legacy `otherArchives` key (the old manual list of other archive drives) and `lastDestination` are merged into `knownArchives`; `otherArchives` is then dropped from the file.

If the file is corrupt, the app renames it to `settings.json.bak` and starts with default settings.

## Theme

There are two themes: light and dark. On first start (`theme` = `auto`) the app detects the OS color scheme once:

- **Linux**: the `color-scheme` key in `gsettings` (GNOME, KDE and most modern desktops), then `ColorScheme` in `~/.config/kdeglobals`, then the `GTK_THEME` environment variable and the GTK theme from `gsettings`. If nothing matches, light is used.
- **Windows**: the `AppsUseLightTheme` registry value (`HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`).

After that, the sun/moon button in the top-right corner of the window switches the theme, and the choice is saved to `theme`. Later changes to the OS theme are not followed.

## Interface language

The UI is available in English and Russian. The language is picked at startup from the OS locale: Russian if the system language is Russian, English otherwise. There is no setting for it.

The archive folder names (`Фото`, `Видео`, `Картинки`, `Без даты` — Photos, Videos, Pictures, No date) do not depend on the language for now and stay in Russian.

## Recent sources

Only the last three sources are remembered. The Source screen shows them on one line as short chips, and the list can be cleared. After a real (non-dry) run the selected sources are cleared, so the next run cannot silently include them again. The "New archive" button on the report screen resets the whole wizard: sources, scan results and report.

## Duplicates across archives

There is no setting for other archive drives. Before each run the app checks for duplicates in:

- every archive listed in `knownArchives` (each folder a real run has written to is added there);
- any archive found at the root of a connected drive (one that contains `.memoryarchive/index.db`).

Other archives are opened read-only. Archives on disconnected drives are skipped silently.

## Destination screen

For the selected folder the app shows:

- free space on the drive;
- how many files and bytes will be copied. Files already physically present in the archive are excluded. The estimate is based on file sizes, so it is an upper bound;
- how many files are physically in the folder (files on disk are counted, not index records).

The 1 GiB safety reserve is not included in the "will be copied" total, but it is required for the "enough space" check: free space must be at least the copy size plus 1 GiB. During a run, archiving stops if writing the next file would leave less than 1 GiB free.

## Load levels

| Level | Analysis workers | Parallel reads | Speed limit | Disk priority |
|---|---|---|---|---|
| Low | 1 | 1 | 40 MB/s | background (idle) |
| Medium | min(4, cores/2) | 1 | none | lowered |
| High | cores − 1 | 2 | none | lowered |

CPU priority is lowered on every level (nice 10 on Linux, background mode on Windows). For external HDDs, Low or Medium is recommended: the disk is read sequentially and does not thrash.

## Logs

Verbosity is set with the `LOG_LEVEL` environment variable:

```bash
LOG_LEVEL=debug ./BelMemories   # decision for every file: date, category, duplicate
```

| Value | What is logged |
|---|---|
| `error` | errors only |
| `warn` | + skipped files, access problems |
| `info` (default) | + run start/finish, progress every 1000 files |
| `debug` | + per-file details |

Records are JSON with the fields `component`, `path`, `err`.

## CLIP neural network

The model is searched in this order: `modelDir` from the settings, the `models/` folder next to the executable, `./models`, `<data>/BelMemories/models` (`~/.local/share/BelMemories/models` on Linux, `%LOCALAPPDATA%\BelMemories\models` on Windows; filled by `task install:linux`), `<config>/BelMemories/models`. The `onnxruntime` library is searched next to the executable, in `models/lib/`, then in system paths.

If something is missing, the app falls back to rules only. The Destination screen shows this with a "rules only" label and the reason.

## See also

- [Building](build.md): how to get the model and `onnxruntime`
- [Architecture](architecture.md): how classification and the index work
