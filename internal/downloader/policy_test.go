package downloader

import (
	"testing"
	"time"
)

func TestPolicyCountsOnlyEligibleAwakeIntervals(t *testing.T) {
	p := policyState{}
	prev := observation{at: time.Unix(100, 0), awake: 100 * time.Second, eligible: true}
	step := func(wall, awake time.Duration, eligible bool) {
		t.Helper()
		cur := observation{at: prev.at.Add(wall), awake: prev.awake + awake, eligible: eligible, up: prev.up + 10, down: prev.down + 20}
		p.observe(prev, cur, true, 10000)
		prev = cur
	}
	step(time.Second, time.Second, true)
	if p.SeedingTime != time.Second {
		t.Fatal(p)
	}
	step(time.Second, time.Second, false)
	step(time.Second, time.Second, false)
	step(time.Second, time.Second, true)
	if p.SeedingTime != time.Second {
		t.Fatal("network transition/offline time counted:", p)
	}
	step(time.Hour, 100*time.Millisecond, true)
	if p.SeedingTime != time.Second {
		t.Fatal("suspend gap counted:", p)
	}
	step(500*time.Millisecond, 200*time.Millisecond, true)
	if p.SeedingTime != 1200*time.Millisecond {
		t.Fatal("sleep-biased clock was used:", p)
	}
	if p.UploadedBytes != 60 || p.DownloadedBytes != 120 {
		t.Fatal("payload accounting depends on connectivity:", p)
	}
}

func TestPolicyHourRatioImportedFilesAndRestartEpoch(t *testing.T) {
	p := policyState{SeedingTime: 3599 * time.Second, DownloadedBytes: 100, UploadedBytes: 50}
	prev := observation{at: time.Unix(100, 0), awake: 100 * time.Second, eligible: true}
	cur := observation{at: prev.at.Add(2 * time.Second), awake: prev.awake + 2*time.Second, eligible: true}
	p.observe(prev, cur, true, 100)
	if !p.AutoStopped || p.SeedingTime != time.Hour {
		t.Fatal("hour target not enforced exactly:", p)
	}
	q := policyState{DownloadedBytes: 100, UploadedBytes: 100}
	q.observe(observation{}, observation{}, false, 100)
	if q.AutoStopped {
		t.Fatal("ratio stopped an incomplete download")
	}
	q.observe(observation{}, observation{}, true, 100)
	if !q.AutoStopped {
		t.Fatal("ratio target ignored")
	}
	imported := policyState{}
	imported.observe(observation{}, observation{}, true, 100)
	if imported.AutoStopped {
		t.Fatal("imported file stopped at zero/zero ratio")
	}
	imported.observe(observation{}, observation{up: 100}, true, 100)
	if !imported.AutoStopped {
		t.Fatal("imported file content-size target ignored")
	}
	restarted := policyState{DownloadedBytes: 1000, UploadedBytes: 500}
	restarted.observe(observation{}, observation{up: 20, down: 30}, false, 5000)
	if restarted.UploadedBytes != 520 || restarted.DownloadedBytes != 1030 {
		t.Fatal("new epoch counters lost/doubled:", restarted)
	}
}

func TestPolicyHonorsConfiguredSeedTarget(t *testing.T) {
	drive := func(p *policyState, seconds int64) {
		t.Helper()
		prev := observation{at: time.Unix(100, 0), awake: 100 * time.Second, eligible: true}
		for i := int64(0); i < seconds; i++ {
			next := observation{at: prev.at.Add(time.Second), awake: prev.awake + time.Second, eligible: true}
			p.observe(prev, next, true, 100)
			prev = next
		}
	}
	short := policyState{SeedTarget: 30 * time.Minute}
	drive(&short, 30*60)
	if !short.AutoStopped || short.SeedingTime != 30*time.Minute {
		t.Fatalf("30-minute target not enforced: %+v", short)
	}
	forever := policyState{SeedTarget: seedForever, SeedingTime: time.Hour, DownloadedBytes: 100, UploadedBytes: 50}
	drive(&forever, 2)
	if forever.AutoStopped {
		t.Fatalf("unlimited seed target stopped on time: %+v", forever)
	}
	ratio := policyState{SeedTarget: seedForever, DownloadedBytes: 100, UploadedBytes: 50}
	drive(&ratio, 1)
	if ratio.AutoStopped {
		t.Fatalf("unlimited seed target stopped below ratio: %+v", ratio)
	}
	ratio.UploadedBytes = 99
	ratio.observe(observation{}, observation{up: 1}, true, 100)
	if !ratio.AutoStopped {
		t.Fatalf("unlimited seed target ignored the ratio stop: %+v", ratio)
	}
	unset := policyState{SeedTarget: 0}
	drive(&unset, 60*60)
	if !unset.AutoStopped || unset.SeedingTime != time.Hour {
		t.Fatalf("default hour target not kept for zero target: %+v", unset)
	}
}

func TestPolicyStopsAtConfiguredShareRatio(t *testing.T) {
	p := policyState{DownloadedBytes: 100, UploadedBytes: 199, RatioTarget: 2}
	if p.reached(0) {
		t.Fatal("a 2:1 target stopped before twice the downloaded size")
	}
	p.UploadedBytes = 200
	if !p.reached(0) {
		t.Fatal("a 2:1 target did not stop at twice the downloaded size")
	}
}

func TestPolicyUnlimitedRatioStopsOnlyOnSeedingTime(t *testing.T) {
	p := policyState{DownloadedBytes: 100, UploadedBytes: 5000, RatioTarget: seedRatioUnlimited}
	if p.reached(0) {
		t.Fatal("an unlimited ratio target stopped on uploaded bytes")
	}
}
