package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// A crash between the tombstone commit and the finalize leaves the same state
// as a downloader that refused to remove: Removing stays set with the task
// record intact. Startup recovery must complete both halves.
func TestRecoveryCompletesInterruptedRemoval(t *testing.T) {
	a := resource("a", 1, "1080p")
	hash, _ := ResourceHash(a)
	path := filepath.Join(t.TempDir(), "state.db")
	d := &fakeDownloader{}
	c := openTest(t, path, Options{Downloader: d})
	sub := create(t, c, anime, group, Rule{}, []Resource{a}, []string{a.ID})
	if err := c.RemoveTask(context.Background(), hash, false, func() (bool, error) {
		return false, errors.New("process died before the downloader forgot the task")
	}); err == nil {
		t.Fatal("the failed removal must report an error")
	}
	tasks, err := c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !tasks[0].Removing {
		t.Fatalf("failed removal lost its tombstone: %+v", tasks)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	d2 := &fakeDownloader{}
	c = openTest(t, path, Options{Downloader: d2})
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return d2.removeCount() == 1 })
	if d2.count() != 0 {
		t.Fatalf("recovery resubmitted a deleted task: calls=%d", d2.count())
	}
	tasks, err = c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("recovery kept the tombstoned record: %+v", tasks)
	}
	if got := current(t, c, sub.ID).Items[a.ID]; got.Status != ItemSkipped {
		t.Fatalf("recovery left the item %q", got.Status)
	}
}

func TestRecoveryCompletesInterruptedRedownload(t *testing.T) {
	a := resource("a", 1, "1080p")
	hash, _ := ResourceHash(a)
	path := filepath.Join(t.TempDir(), "state.db")
	d := &fakeDownloader{}
	c := openTest(t, path, Options{Downloader: d})
	sub := create(t, c, anime, group, Rule{}, []Resource{a}, []string{a.ID})
	if err := c.RemoveTask(context.Background(), hash, true, func() (bool, error) {
		return false, errors.New("process died before the downloader forgot the task")
	}); err == nil {
		t.Fatal("the failed removal must report an error")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	d2 := &fakeDownloader{}
	c = openTest(t, path, Options{Downloader: d2})
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return d2.count() == 1 })
	if d2.removeCount() != 1 {
		t.Fatalf("recovery skipped the engine half: attempts=%d", d2.removeCount())
	}
	tasks, err := c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !tasks[0].Accepted || tasks[0].Removing || tasks[0].Task.Directory != filepath.Join(sub.Directory, hash) {
		t.Fatalf("recovered redownload: %+v", tasks)
	}
}

// A downloader that keeps failing must not hot-loop: the retry backs off
// through the record and stays a tombstone until it succeeds.
func TestRecoveryBacksOffFailingRemoval(t *testing.T) {
	a := resource("a", 1, "1080p")
	hash, _ := ResourceHash(a)
	path := filepath.Join(t.TempDir(), "state.db")
	d := &fakeDownloader{}
	c := openTest(t, path, Options{Downloader: d})
	create(t, c, anime, group, Rule{}, []Resource{a}, []string{a.ID})
	if err := c.RemoveTask(context.Background(), hash, false, func() (bool, error) {
		return false, errors.New("crashed before the downloader ran")
	}); err == nil {
		t.Fatal("the failed removal must report an error")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	d2 := &fakeDownloader{}
	d2.setRemoveHook(func(string) error { return errors.New("content folder is locked") })
	c = openTest(t, path, Options{Downloader: d2})
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		tasks, err := c.ListTasks()
		if err != nil {
			t.Fatal(err)
		}
		return len(tasks) == 1 && tasks[0].LastError != ""
	})
	before := d2.removeCount()
	c.recoverRemovals(context.Background(), false)
	if d2.removeCount() != before {
		t.Fatalf("recovery ignored the backoff: attempts=%d", d2.removeCount())
	}
	tasks, err := c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !tasks[0].Removing {
		t.Fatalf("failed recovery lost the tombstone: %+v", tasks)
	}
}
