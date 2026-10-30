package downloader

import (
	"math"
	"time"
)

const (
	defaultSeedTarget = time.Hour
	// seedForever asks the policy to stop only on the ratio target.
	seedForever = -1
)

type policyState struct {
	Paused          bool          `json:"paused"`
	ManuallyStopped bool          `json:"manually_stopped"`
	AutoStopped     bool          `json:"auto_stopped"`
	DownloadedBytes int64         `json:"downloaded_bytes"`
	UploadedBytes   int64         `json:"uploaded_bytes"`
	SeedingTime     time.Duration `json:"seeding_nanoseconds"`
	// SeedTarget is the seeding-time stop target: positive minutes, or -1 to
	// seed without a time limit (the ratio target still applies). Zero and
	// any other negative value select the default one hour.
	SeedTarget time.Duration `json:"seed_target_ns,omitempty"`
}

type observation struct {
	at       time.Time
	awake    time.Duration
	up, down int64
	eligible bool
}

func (p *policyState) target() time.Duration {
	switch {
	case p.SeedTarget == seedForever:
		return time.Duration(math.MaxInt64)
	case p.SeedTarget > 0:
		return p.SeedTarget
	}
	return defaultSeedTarget
}

func (p *policyState) observe(prev, cur observation, complete bool, size int64) {
	p.UploadedBytes += counterDelta(prev.up, cur.up)
	p.DownloadedBytes += counterDelta(prev.down, cur.down)
	wall := cur.at.UnixNano() - prev.at.UnixNano()
	dt := cur.awake - prev.awake
	// A missed sampling window cannot establish continuous network availability.
	if prev.eligible && cur.eligible && wall > 0 && wall <= int64(3*time.Second) && dt > 0 && dt <= 3*time.Second {
		p.SeedingTime = min(p.target(), p.SeedingTime+dt)
	}
	if complete && !p.Paused && !p.ManuallyStopped && p.reached(size) {
		p.AutoStopped = true
	}
}

func (p policyState) reached(size int64) bool {
	denominator := p.DownloadedBytes
	// Imported files must contribute their content size rather than a zero-byte target.
	if denominator == 0 {
		denominator = size
	}
	return p.SeedingTime >= p.target() || denominator > 0 && p.UploadedBytes >= denominator
}

func counterDelta(previous, current int64) int64 {
	if current < previous {
		return max(0, current)
	}
	return current - previous
}
