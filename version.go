package main

import (
	_ "embed"
	"encoding/json"
	"log/slog"
)

// wails.json is the single source of the product version (info.productVersion):
// it is shown in "About", written to the log and stamped into the Windows .exe.
// Bump it with `task release -- 1.2.0`.
//
//go:embed wails.json
var wailsJSON []byte

// version may still be forced at build time: -ldflags "-X main.version=1.2.3".
var version = ""

// appVersion returns the product version, e.g. "1.1.0".
func appVersion() string {
	if version != "" {
		return version
	}
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsJSON, &cfg); err != nil || cfg.Info.ProductVersion == "" {
		slog.Warn("[FIX] cannot read productVersion from wails.json", "err", err)
		return "0.0.0"
	}
	return cfg.Info.ProductVersion
}
