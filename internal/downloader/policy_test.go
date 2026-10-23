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
