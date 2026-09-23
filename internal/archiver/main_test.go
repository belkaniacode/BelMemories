package archiver

import (
	"os"
	"testing"
)

// TestMain isolates the tests from the user's installed files: the classifier
// must run in rules-only mode even if a CLIP model is installed in the
// per-user data or config folder.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "archiver-test-home")
	if err != nil {
		panic(err)
	}
	for _, k := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "LOCALAPPDATA", "APPDATA"} {
		os.Setenv(k, dir)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
