package downloader

import (
	"context"
	"path/filepath"
	"testing"

	"dipper.local/dipper/internal/core"
)

type coreAdapter struct{ engine *Engine }

func (a coreAdapter) Ensure(task core.DownloadTask) error {
	return a.engine.Ensure(Task{ID: task.Hash, Magnet: task.Resource.Magnet, TorrentURL: task.Resource.TorrentURL, Folder: task.Directory, Context: task.Context})
}

func TestCoreRealEnginePipelineAndRestart(t *testing.T) {
	seed := newFixture(t, 256<<10)
	root := t.TempDir()
	config := Config{DataDir: filepath.Join(root, "bt-state")}
	e := testEngine(t, config)
	selected := core.Resource{ID: "selected", Title: "[Local] Episode 01", Magnet: seed.magnet}
	skipped := core.Resource{ID: "skipped", Title: "[Local] Episode 02", Magnet: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"}
	feed := func(context.Context, string, string) ([]core.Resource, error) {
		alias := selected
		alias.ID = "rss-duplicate"
		return []core.Resource{selected, alias, skipped}, nil
	}
	openCore := func(engine *Engine) *core.Core {
		t.Helper()
		c, err := core.Open(filepath.Join(root, "core.sqlite"), core.Options{Downloader: coreAdapter{engine}, Feed: feed, DownloadRoot: filepath.Join(root, "downloads"), DisableScheduler: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := c.Close(); err != nil {
				t.Error(err)
			}
		})
		return c
	}
	c := openCore(e)
	sub, err := c.CreateSubscription(context.Background(), core.Anime{ID: "anime", Title: "Local Anime"}, core.Group{ID: "group", Name: "Local"}, core.Rule{}, []core.Resource{selected, skipped}, []string{selected.ID})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "core-selected resource downloaded by real engine", func() bool { return one(t, e).Complete })
	snapshot := one(t, e)
	if snapshot.InfoHash != seed.mi.HashInfoBytes().HexString() || snapshot.ID != snapshot.InfoHash {
		t.Fatalf("core identity differs from real torrent: %+v", snapshot)
	}
	checkFiles(t, seed, snapshot.Folder)
	assertState := func(c *core.Core, engine *Engine) {
		t.Helper()
		state, err := c.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Tasks) != 1 || !state.Tasks[0].Accepted || len(engine.Snapshot()) != 1 {
			t.Fatalf("pipeline duplicated/lost accepted task: %+v", state.Tasks)
		}
		if len(state.Subscriptions) != 1 {
			t.Fatal("subscription lost")
		}
		item := state.Subscriptions[0].Items[skipped.ID]
		if !item.Read || item.Status != core.ItemSkipped {
			t.Fatalf("unchecked resource was not durably skipped: %+v", item)
		}
	}
	for range 2 {
		if err := c.Check(context.Background(), sub.ID); err != nil {
			t.Fatal(err)
		}
		assertState(c, e)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	e = testEngine(t, config)
	c = openCore(e)
	assertState(c, e)
	if err := c.Check(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	assertState(c, e)
	waitFor(t, "real engine restored after core restart", func() bool { return one(t, e).Complete })
	checkFiles(t, seed, one(t, e).Folder)
}
