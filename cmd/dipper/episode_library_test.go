package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestUIEpisodeLibraryListsAndPlaysEpisodes(t *testing.T) {
	s := uiFixture(t, "episodes")
	s.LibraryID = s.Subscriptions[0].ID
	s.Downloads[0].Episode = "02"
	s.Downloads[2].Episode = "05"
	played := make(chan string, 1)
	engine := &playTestEngine{files: []string{filepath.Join(s.Root, "星海旅记", "episode.mkv")}}
	s.Engine = engine
	s.PlayFile = func(path string) error { played <- path; return nil }
	updates := make(chan func(), 4)
	s.Update = func(fn func()) { updates <- fn }
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "第 02 集", "第 05 集", "可播放", "下载失败")
	if tt.HasText("播放 " + s.Downloads[1].ID) {
		t.Fatal("a failed download offers play")
	}
	episode02, ok := tt.Find("第 02 集")
	if !ok {
		t.Fatal("missing the 第 02 集 row")
	}
	episode05, ok := tt.Find("第 05 集")
	if !ok {
		t.Fatal("missing the 第 05 集 row")
	}
	if episode05.Y < episode02.Y {
		t.Fatalf("episodes are not ordered by number: 02 at %.0f, 05 at %.0f", episode02.Y, episode05.Y)
	}

	uiClick(t, tt, "播放 "+s.Downloads[2].ID)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	var path string
	opened := false
	for !opened {
		select {
		case path = <-played:
			opened = true
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatal("the episode library play button did not open a file")
		}
	}
	if path != filepath.Join(s.Root, "星海旅记", "episode.mkv") {
		t.Fatalf("played %q, want the downloaded episode", path)
	}
}

func TestUIEpisodeLibraryEmptyOffersTheResourceView(t *testing.T) {
	s := uiFixture(t, "episodes")
	s.LibraryID = s.Subscriptions[0].ID
	s.Downloads = nil
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "还没有已下载的剧集")
	uiClick(t, tt, "查看资源")
	if s.Page != "detail" || !s.ViewResources || !s.Backfill {
		t.Fatalf("empty library state: page=%q view=%v backfill=%v", s.Page, s.ViewResources, s.Backfill)
	}
}
