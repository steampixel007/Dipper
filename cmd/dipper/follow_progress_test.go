package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
)

func TestUIFollowingCardShowsDownloadProgress(t *testing.T) {
	s := uiFixture(t, "following")
	s.project(core.State{
		Subscriptions: []core.Subscription{{ID: "sub-x", Anime: core.Anime{ID: "render-1"}, Group: core.Group{ID: "g1", Name: "回归测试组甲"}}},
		Tasks: []core.TaskRecord{
			{Task: core.DownloadTask{Hash: "hash-1", AnimeID: "render-1", Resource: core.Resource{Title: "ep1", Size: 200 << 20}, Directory: "C:\\d\\a"}},
			{Task: core.DownloadTask{Hash: "hash-2", AnimeID: "render-1", Resource: core.Resource{Title: "ep2", Size: 200 << 20}, Directory: "C:\\d\\b"}},
		},
	}, []downloader.Snapshot{{ID: "hash-1", Progress: 0.5}, {ID: "hash-2", Progress: 1}})
	if len(s.Subscriptions) != 1 || !s.Subscriptions[0].Active {
		t.Fatalf("a follow with a running download must be active: %+v", s.Subscriptions)
	}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "下载中 75%")
}
