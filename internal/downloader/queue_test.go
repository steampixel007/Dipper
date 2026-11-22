package downloader

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ensureQueueMagnet(t *testing.T, engine *Engine, digit string) string {
	t.Helper()
	id := strings.Repeat(digit, 40)
	if err := engine.Ensure(Task{ID: id, Magnet: "magnet:?xt=urn:btih:" + id, Folder: filepath.Join(filepath.Dir(engine.dir), id)}); err != nil {
		t.Fatal(err)
	}
	return id
}

func queueSnapshot(t *testing.T, engine *Engine, id string) Snapshot {
	t.Helper()
	for _, task := range engine.Snapshot() {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("task %s is missing", id)
	return Snapshot{}
}

func requireQueueStatus(t *testing.T, engine *Engine, id, want string) {
	t.Helper()
	if got := queueSnapshot(t, engine, id); got.Status != want {
		t.Fatalf("task %s status = %s, want %s", id, got.Status, want)
	}
}

func TestDownloadQueueLimitAndRestartOrder(t *testing.T) {
	config := Config{DataDir: filepath.Join(t.TempDir(), "state")}
	engine := testEngine(t, config)
	first := ensureQueueMagnet(t, engine, "f")
	second := ensureQueueMagnet(t, engine, "e")
	third := ensureQueueMagnet(t, engine, "d")
	fourth := ensureQueueMagnet(t, engine, "c")
	last := ensureQueueMagnet(t, engine, "a")
	for _, id := range []string{first, second, third} {
		requireQueueStatus(t, engine, id, StatusMetadata)
	}
	for _, id := range []string{fourth, last} {
		requireQueueStatus(t, engine, id, StatusQueued)
	}

	engine.SetDownloadLimit(2)
	requireQueueStatus(t, engine, third, StatusQueued)
	if err := engine.Pause(first); err != nil {
		t.Fatal(err)
	}
	requireQueueStatus(t, engine, first, StatusPaused)
	requireQueueStatus(t, engine, third, StatusMetadata)
	if err := engine.Resume(first); err != nil {
		t.Fatal(err)
	}
	requireQueueStatus(t, engine, first, StatusQueued)
	engine.SetDownloadLimit(4)
	requireQueueStatus(t, engine, fourth, StatusMetadata)
	requireQueueStatus(t, engine, last, StatusMetadata)
	requireQueueStatus(t, engine, first, StatusQueued)

	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	config.DownloadLimit = 1
	reopened := testEngine(t, config)
	requireQueueStatus(t, reopened, second, StatusMetadata)
	if err := reopened.Remove(second); err != nil {
		t.Fatal(err)
	}
	requireQueueStatus(t, reopened, third, StatusMetadata)
	requireQueueStatus(t, reopened, last, StatusQueued)
	requireQueueStatus(t, reopened, first, StatusQueued)
}

func TestDownloadQueueWaitsForPayloadAndFillsSlotOnCompletion(t *testing.T) {
	seed := newFixture(t, 128<<10)
	root := t.TempDir()
	engine := testEngine(t, Config{DataDir: filepath.Join(root, "state"), DownloadLimit: 1})
	first := ensureQueueMagnet(t, engine, "f")
	task := Task{ID: seed.mi.HashInfoBytes().HexString(), Magnet: seed.magnet, Folder: filepath.Join(root, "content")}
	if err := engine.Ensure(task); err != nil {
		t.Fatal(err)
	}
	last := ensureQueueMagnet(t, engine, "c")
	requireQueueStatus(t, engine, task.ID, StatusQueued)
	waitFor(t, "queued torrent metadata", 20*time.Second, func() bool {
		return queueSnapshot(t, engine, task.ID).TotalBytes > 0
	})
	waiting := queueSnapshot(t, engine, task.ID)
	if waiting.Status != StatusQueued || waiting.DownloadedBytes != 0 || waiting.CompletedBytes != 0 {
		t.Fatalf("waiting torrent transferred payload: %+v", waiting)
	}
	if err := engine.Pause(first); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "queued download completes and starts the next task", 30*time.Second, func() bool {
		return queueSnapshot(t, engine, task.ID).Status == StatusSeeding &&
			queueSnapshot(t, engine, last).Status == StatusMetadata
	})
	checkFiles(t, seed, task.Folder)
}

func TestDownloadQueueRotatesOnlyAfterObservedOnlineIdle(t *testing.T) {
	engine := testEngine(t, Config{DataDir: filepath.Join(t.TempDir(), "state"), DownloadLimit: 1})
	first := ensureQueueMagnet(t, engine, "f")
	next := ensureQueueMagnet(t, engine, "e")
	engine.cancel()
	<-engine.done
	now, awake := time.Now(), time.Second
	tick := func(elapsed time.Duration) {
		engine.mu.Lock()
		defer engine.mu.Unlock()
		now, awake = now.Add(elapsed), awake+elapsed
		online := engine.isOnlineLocked()
		for _, task := range engine.tasks {
			engine.sampleLocked(task, now, awake, online)
		}
		engine.scheduleLocked(online)
	}
	tick(time.Second)
	tick(idleDownloadLimit + time.Second)
	engine.SetNetworkAvailable(false)
	for range int(idleDownloadLimit/time.Second) + 1 {
		tick(time.Second)
	}
	requireQueueStatus(t, engine, first, StatusMetadata)
	engine.SetNetworkAvailable(true)
	engine.SetSuspended(true)
	tick(idleDownloadLimit + time.Second)
	requireQueueStatus(t, engine, first, StatusMetadata)
	engine.SetSuspended(false)
	tick(time.Second)
	for range int(idleDownloadLimit/time.Second) - 1 {
		tick(time.Second)
	}
	requireQueueStatus(t, engine, first, StatusMetadata)
	tick(time.Second)
	requireQueueStatus(t, engine, next, StatusMetadata)
	requireQueueStatus(t, engine, first, StatusQueued)
}
