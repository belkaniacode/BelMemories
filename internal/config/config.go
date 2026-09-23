// Package config loads and saves persistent user settings (settings.json).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"memoryarchive/internal/logging"
)

// LoadLevel is a user-selectable resource usage level.
type LoadLevel string

const (
	LoadLow    LoadLevel = "low"
	LoadMedium LoadLevel = "medium"
	LoadHigh   LoadLevel = "high"
)

// Settings is the persisted user configuration.
type Settings struct {
	RecentSources   []string  `json:"recentSources"`
	LastDestination string    `json:"lastDestination"`
	Load            LoadLevel `json:"load"`
	VerifyAfterCopy bool      `json:"verifyAfterCopy"`
	// BytesPerSecLimit overrides the load profile's bandwidth limit when > 0.
	BytesPerSecLimit int64 `json:"bytesPerSecLimit"`
	// ModelDir overrides where CLIP model files are searched.
	ModelDir string `json:"modelDir"`
	// ClipThreshold is the minimal group probability to trust CLIP.
	ClipThreshold float64 `json:"clipThreshold"`
	// OtherArchives lists other archive roots used for cross-disk dedup.
	OtherArchives []string `json:"otherArchives"`
}

// Defaults returns default settings.
func Defaults() Settings {
	return Settings{
		Load:            LoadMedium,
		VerifyAfterCopy: true,
		ClipThreshold:   0.5,
	}
}

// Store reads/writes settings to a JSON file, safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore creates a store at <AppConfigDir>/settings.json unless path is given.
func NewStore(path string) *Store {
	if path == "" {
		path = filepath.Join(logging.AppConfigDir(), "settings.json")
	}
	return &Store{path: path}
}

// Path returns the settings file path.
func (s *Store) Path() string { return s.path }

// Load reads settings; missing or corrupt files yield defaults. A corrupt file
// is renamed to settings.json.bak so the user's data is not lost silently.
func (s *Store) Load() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	log := logging.Component("config")

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		log.Info("settings file not found, using defaults", "path", s.path)
		return Defaults()
	}
	if err != nil {
		log.Warn("cannot read settings, using defaults", "path", s.path, "err", err)
		return Defaults()
	}
	st := Defaults()
	if err := json.Unmarshal(data, &st); err != nil {
		log.Warn("corrupt settings file, using defaults", "path", s.path, "err", err)
		_ = os.Rename(s.path, s.path+".bak")
		return Defaults()
	}
	st.normalize()
	log.Info("settings loaded", "path", s.path)
	return st
}

// Save writes settings atomically (tmp file + rename).
func (s *Store) Save(st Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	log := logging.Component("config")

	st.normalize()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace settings: %w", err)
	}
	log.Info("settings saved", "path", s.path)
	return nil
}

func (st *Settings) normalize() {
	switch st.Load {
	case LoadLow, LoadMedium, LoadHigh:
	default:
		st.Load = LoadMedium
	}
	if st.ClipThreshold <= 0 || st.ClipThreshold >= 1 {
		st.ClipThreshold = 0.5
	}
	if len(st.RecentSources) > 20 {
		st.RecentSources = st.RecentSources[:20]
	}
}
