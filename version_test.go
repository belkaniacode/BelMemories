package main

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// The version shown in "About" must be the product version from wails.json
// (1.0, 1.1, 1.1.1 …), never a git commit hash or "dev".
func TestAppVersionIsProductVersion(t *testing.T) {
	data, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	got := appVersion()
	if got != cfg.Info.ProductVersion {
		t.Fatalf("appVersion() = %q, want productVersion %q", got, cfg.Info.ProductVersion)
	}
	if !regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`).MatchString(got) {
		t.Fatalf("version %q is not like 1.1 or 1.1.0", got)
	}
}
