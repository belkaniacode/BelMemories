// Package priority lowers process CPU/IO priority and defines load profiles
// so archiving does not slow down the rest of the computer.
package priority

import (
	"runtime"

	"memoryarchive/internal/config"
	"memoryarchive/internal/logging"
)

// Profile holds concurrency and bandwidth limits for a load level.
type Profile struct {
	Level       config.LoadLevel `json:"level"`
	CPUWorkers  int              `json:"cpuWorkers"`
	Readers     int              `json:"readers"`
	BytesPerSec int64            `json:"bytesPerSec"` // 0 = unlimited
	IdleIO      bool             `json:"idleIo"`
}

// ProfileFor returns the profile for a level. override > 0 replaces the
// bandwidth limit.
func ProfileFor(level config.LoadLevel, override int64) Profile {
	n := runtime.NumCPU()
	var p Profile
	switch level {
	case config.LoadLow:
		p = Profile{Level: level, CPUWorkers: 1, Readers: 1, BytesPerSec: 40 << 20, IdleIO: true}
	case config.LoadHigh:
		p = Profile{Level: level, CPUWorkers: max(1, n-1), Readers: 2}
	default:
		p = Profile{Level: config.LoadMedium, CPUWorkers: max(1, min(4, n/2)), Readers: 1}
	}
	if override > 0 {
		p.BytesPerSec = override
	}
	return p
}

// Apply lowers the process priority according to the profile. Failures are
// logged and ignored: the app keeps working at normal priority.
func Apply(p Profile) {
	log := logging.Component("priority")
	if err := setBackground(p.IdleIO); err != nil {
		log.Warn("cannot lower process priority", "err", err)
	}
	log.Info("load profile applied", "level", p.Level, "cpuWorkers", p.CPUWorkers,
		"readers", p.Readers, "bytesPerSec", p.BytesPerSec, "idleIO", p.IdleIO)
}

// Restore returns the process to normal priority where the OS allows it.
func Restore() {
	if err := restoreNormal(); err != nil {
		logging.Component("priority").Debug("cannot restore priority", "err", err)
	}
}
