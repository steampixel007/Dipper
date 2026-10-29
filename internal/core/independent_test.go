package core

import (
	"context"
	"path/filepath"
	"testing"
)

func TestResetAfterUnsubscribeRedownloadsIndependently(t *testing.T) {
	a := resource("a", 1, "1080p")
	hash, _ := ResourceHash(a)
	d := &fakeDownloader{}
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d})
	sub := create(t, c, anime, group, Rule{}, []Resource{a}, []string{a.ID})
	if err := c.RemoveSubscription(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.ResetTask(context.Background(), hash); err != nil {
		t.Fatal(err)
	}
	tasks, err := c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Accepted || !tasks[0].Independent {
		t.Fatalf("reset did not keep an independent intent: %+v", tasks)
	}
	if tasks[0].Task.Directory != filepath.Join(sub.Directory, hash) {
		t.Fatalf("independent intent lost its folder: %+v", tasks[0].Task)
	}
	if err := c.RetryPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.count() != 2 {
		t.Fatalf("independent intent was not resubmitted: calls=%d", d.count())
	}
	tasks, err = c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !tasks[0].Accepted {
		t.Fatalf("resubmitted task is not accepted: %+v", tasks)
	}
}

func TestIndependentIntentSurvivesRestart(t *testing.T) {
	a := resource("a", 1, "1080p")
	hash, _ := ResourceHash(a)
	path := filepath.Join(t.TempDir(), "state.db")
	d := &fakeDownloader{}
	c := openTest(t, path, Options{Downloader: d})
	sub := create(t, c, anime, group, Rule{}, []Resource{a}, []string{a.ID})
	if err := c.RemoveSubscription(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.ResetTask(context.Background(), hash); err != nil {
		t.Fatal(err)
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
	tasks, err := c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !tasks[0].Accepted || tasks[0].Task.Directory != filepath.Join(sub.Directory, hash) {
		t.Fatalf("restart replay of the independent intent: %+v", tasks)
	}
}
