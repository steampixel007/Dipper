package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// A re-download that lands while a removal is between its tombstone and its
// finalize must survive it. The removal holds the delivery slot across both
// halves, so an intent recorded under the same slot waits for the finalize
// instead of being swept up by it.
func TestRedownloadDuringRemovalKeepsIntent(t *testing.T) {
	a := resource("a", 1, "1080p")
	hash, _ := ResourceHash(a)
	d := &fakeDownloader{}
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d})
	sub := create(t, c, anime, group, Rule{}, []Resource{a}, []string{a.ID})

	entered := make(chan struct{})
	release := make(chan struct{})
	removal := make(chan error, 1)
	go func() {
		removal <- c.RemoveTask(context.Background(), hash, false, func() (bool, error) {
			close(entered)
			<-release
			return true, nil
		})
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the removal never reached the downloader")
	}

	started := make(chan struct{})
	download := make(chan error, 1)
	go func() {
		close(started)
		_, err := c.DownloadExisting(context.Background(), sub.ID, []Resource{a}, []string{a.ID})
		download <- err
	}()
	<-started
	// An intent recorded while the removal is held observes itself in state at
	// once; one serialized behind the removal waits for the slot and never
	// appears, so this poll drains the window in which the removal is held.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if current(t, c, sub.ID).Items[a.ID].Reason == "主动补下载，等待提交" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	if err := <-removal; err != nil {
		t.Fatalf("the removal failed: %v", err)
	}
	if err := <-download; err != nil {
		t.Fatalf("the re-download failed: %v", err)
	}
	item := current(t, c, sub.ID).Items[a.ID]
	if item.Status != ItemDownloaded {
		t.Fatalf("the removal swept up the re-download: status = %q reason = %q", item.Status, item.Reason)
	}
	if d.count() != 2 {
		t.Fatalf("the re-download was not submitted: ensures = %d", d.count())
	}
	tasks, err := c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !tasks[0].Accepted {
		t.Fatalf("the re-download left no accepted record: %+v", tasks)
	}
}

// A failed removal leaves its tombstone for recovery to finish. An explicit
// re-download recorded after that failure replaces the tombstone, so neither
// submission nor recovery can sweep the fresh intent up.
func TestRedownloadAfterFailedRemovalSupersedesTombstone(t *testing.T) {
	a := resource("a", 1, "1080p")
	hash, _ := ResourceHash(a)
	d := &fakeDownloader{}
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d})
	sub := create(t, c, anime, group, Rule{}, []Resource{a}, []string{a.ID})
	if err := c.RemoveTask(context.Background(), hash, false, func() (bool, error) {
		return false, errors.New("the downloader did not answer")
	}); err == nil {
		t.Fatal("the failed removal must report an error")
	}
	if ok, err := c.DownloadExisting(context.Background(), sub.ID, []Resource{a}, []string{a.ID}); !ok || err != nil {
		t.Fatalf("the re-download was not saved: saved=%v err=%v", ok, err)
	}
	if d.count() != 2 {
		t.Fatalf("the re-download was not submitted: ensures = %d", d.count())
	}
	tasks, err := c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !tasks[0].Accepted || tasks[0].Removing {
		t.Fatalf("the tombstone survived the re-download: %+v", tasks)
	}
	c.recoverRemovals(context.Background(), true)
	if d.removeCount() != 0 {
		t.Fatalf("recovery removed the superseded intent: removes = %d", d.removeCount())
	}
	if item := current(t, c, sub.ID).Items[a.ID]; item.Status != ItemDownloaded {
		t.Fatalf("the re-download was swept: status = %q reason = %q", item.Status, item.Reason)
	}
}
