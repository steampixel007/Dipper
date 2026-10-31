package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
)

func TestUIFollowingCardShowsCompletedCount(t *testing.T) {
	s := uiFixture(t, "following")
	s.project(core.State{
		Subscriptions: []core.Subscription{{ID: "sub-x", Anime: core.Anime{ID: "render-1"}, Group: core.Group{ID: "g1", Name: "回归测试组甲"}}},
		Tasks: []core.TaskRecord{
			{Task: core.DownloadTask{Hash: "hash-1", AnimeID: "render-1", Resource: core.Resource{Title: "ep1", Size: 200 << 20}, Directory: "C:\\d\\a"}},
			{Task: core.DownloadTask{Hash: "hash-2", AnimeID: "render-1", Resource: core.Resource{Title: "ep2", Size: 200 << 20}, Directory: "C:\\d\\b"}},
			{Task: core.DownloadTask{Hash: "hash-3", AnimeID: "render-1", Resource: core.Resource{Title: "ep3", Size: 200 << 20}, Directory: "C:\\d\\c"}},
		},
	}, []downloader.Snapshot{
		{ID: "hash-1", Progress: 1},
		{ID: "hash-2", Progress: 0.5},
		{ID: "hash-3", Progress: 1, Error: "disk full"},
	})
	if s.Subscriptions[0].Completed != 1 {
		t.Fatalf("only the finished download counts as completed, got %d", s.Subscriptions[0].Completed)
	}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "已下载 1 个资源")
}
