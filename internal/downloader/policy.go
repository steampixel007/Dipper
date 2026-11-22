package downloader

import (
	"math"
	"time"
)

const (
	defaultSeedTarget = time.Hour
	// seedForever asks the policy to stop only on the ratio target.
	seedForever = -1
	// seedRatioUnlimited asks the policy to stop only on seeding time.
	seedRatioUnlimited = -1
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
	// RatioTarget is the share-ratio stop target: a positive multiple of the
	// downloaded size, -1 to stop only on seeding time, zero for the default
	// 1:1.
	RatioTarget float64 `json:"ratio_target,omitempty"`
}

type observation struct {
	at               time.Time
	awake            time.Duration
	up, down         int64
	eligible         bool
	downloadEligible bool
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
	if elapsed := observedTime(prev, cur); prev.eligible && cur.eligible && elapsed > 0 {
		p.SeedingTime = min(p.target(), p.SeedingTime+elapsed)
	}
	if complete && !p.Paused && !p.ManuallyStopped && p.reached(size) {
		p.AutoStopped = true
	}
}

func observedTime(prev, cur observation) time.Duration {
	wall := cur.at.UnixNano() - prev.at.UnixNano()
	delta := cur.awake - prev.awake
	if wall <= 0 || wall > int64(3*time.Second) || delta <= 0 || delta > 3*time.Second {
		return 0
	}
	return delta
}

// ratio is the effective share-ratio multiple; zero disables the ratio stop.
func (p policyState) ratio() float64 {
	switch {
	case p.RatioTarget == seedRatioUnlimited:
		return 0
	case p.RatioTarget > 0:
		return p.RatioTarget
	}
	return 1
}

func (p policyState) reached(size int64) bool {
	denominator := p.DownloadedBytes
	// Imported files must contribute their content size rather than a zero-byte target.
	if denominator == 0 {
		denominator = size
	}
	if ratio := p.ratio(); denominator > 0 && ratio > 0 && float64(p.UploadedBytes) >= float64(denominator)*ratio {
		return true
	}
	return p.SeedingTime >= p.target()
}

func counterDelta(previous, current int64) int64 {
	if current < previous {
		return max(0, current)
	}
	return current - previous
}
