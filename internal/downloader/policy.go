package downloader

import "time"

const seedTarget = time.Hour

type policyState struct {
	Paused          bool          `json:"paused"`
	ManuallyStopped bool          `json:"manually_stopped"`
	AutoStopped     bool          `json:"auto_stopped"`
	DownloadedBytes int64         `json:"downloaded_bytes"`
	UploadedBytes   int64         `json:"uploaded_bytes"`
	SeedingTime     time.Duration `json:"seeding_nanoseconds"`
}

type observation struct {
	at       time.Time
	awake    time.Duration
	up, down int64
	eligible bool
}

func (p *policyState) observe(prev, cur observation, complete bool, size int64) {
	p.UploadedBytes += counterDelta(prev.up, cur.up)
	p.DownloadedBytes += counterDelta(prev.down, cur.down)
	wall := cur.at.UnixNano() - prev.at.UnixNano()
	dt := cur.awake - prev.awake
	// A missed sampling window cannot establish continuous network availability.
	if prev.eligible && cur.eligible && wall > 0 && wall <= int64(3*time.Second) && dt > 0 && dt <= 3*time.Second {
		p.SeedingTime = min(seedTarget, p.SeedingTime+dt)
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
	return p.SeedingTime >= seedTarget || denominator > 0 && p.UploadedBytes >= denominator
}

func counterDelta(previous, current int64) int64 {
	if current < previous {
		return max(0, current)
	}
	return current - previous
}
