package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"belmemories/internal/logging"
)

//go:embed all:frontend/dist
var assets embed.FS

// appIcon is the window icon on Linux/X11 (Windows takes it from the exe resources).
//
//go:embed build/linux/icons/hicolor/256x256/apps/belmemories.png
var appIcon []byte

// linuxAppID is the GTK program name. On Wayland it is the app_id the desktop
// matches against belmemories.desktop to show the taskbar icon.
const linuxAppID = "belmemories"

// version is injected at build time: -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	moved, migrateErr := logging.MigrateLegacyDir()
	_, logPath := logging.Setup(logging.Options{Stderr: version == "dev"})
	if moved {
		slog.Info("settings moved from the legacy MemoryArchive directory", "dir", logging.AppConfigDir())
	} else if migrateErr != nil {
		slog.Warn("cannot move legacy MemoryArchive settings", "err", migrateErr)
	}

	app := NewApp(logPath)

	err := wails.Run(&options.App{
		Title:     "BelMemories",
		Width:     1100,
		Height:    750,
		MinWidth:  820,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 26, B: 31, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			Icon:        appIcon,
			ProgramName: linuxAppID,
		},
	})
	if err != nil {
		slog.Error("wails run failed", "err", err)
		os.Exit(1)
	}
}
