package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRememberArchive(t *testing.T) {
	st := Defaults()
	st.RememberArchive("/mnt/a")
	st.RememberArchive("/mnt/b")
	st.RememberArchive("/mnt/a") // already known: moves to the front, no duplicate
	if want := []string{"/mnt/a", "/mnt/b"}; !reflect.DeepEqual(st.KnownArchives, want) {
		t.Fatalf("known = %v, want %v", st.KnownArchives, want)
	}
}

func TestLegacyOtherArchivesMerged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	data := `{"knownArchives":["/mnt/a"],"otherArchives":["/mnt/b","/mnt/a"],"lastDestination":"/mnt/c"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	st := NewStore(path).Load()
	if want := []string{"/mnt/a", "/mnt/b", "/mnt/c"}; !reflect.DeepEqual(st.KnownArchives, want) {
		t.Fatalf("known = %v, want %v", st.KnownArchives, want)
	}
	if len(st.OtherArchives) != 0 {
		t.Fatalf("legacy list kept: %v", st.OtherArchives)
	}
}
