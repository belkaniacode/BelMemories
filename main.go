package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"memoryarchive/internal/logging"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is injected at build time: -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	_, logPath := logging.Setup(logging.Options{Stderr: version == "dev"})

	app := NewApp(logPath)

	err := wails.Run(&options.App{
		Title:     "MemoryArchive",
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
	})
	if err != nil {
		slog.Error("wails run failed", "err", err)
		os.Exit(1)
	}
}
