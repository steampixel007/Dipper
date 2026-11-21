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
	s.Downloads[1].Failed = true
	s.Downloads[1].Error = "连接超时，请重试"
	played := make(chan string, 1)
	engine := &playTestEngine{files: []string{filepath.Join(s.Root, "星海旅记", "episode.mkv")}}
	s.Engine = engine
	s.PlayFile = func(path string) error { played <- path; return nil }
	updates := make(chan func(), 4)
	s.Update = func(fn func()) { updates <- fn }
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "第 2 集", "第 5 集", "已完成", "失败")
	if tt.HasText("播放 " + s.Downloads[1].ID) {
		t.Fatal("a failed download offers play")
	}
	episode02, ok := tt.Find("第 2 集")
	if !ok {
		t.Fatal("missing the 第 2 集 row")
	}
	episode05, ok := tt.Find("第 5 集")
	if !ok {
		t.Fatal("missing the 第 5 集 row")
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
	uiRequireText(t, tt, "还没有下载完成的剧集")
	uiClick(t, tt, "查看资源")
	if s.Page != "detail" || !s.ViewResources || !s.Backfill {
		t.Fatalf("empty library state: page=%q view=%v backfill=%v", s.Page, s.ViewResources, s.Backfill)
	}
}

func TestUIEpisodeLibraryKeepsPlaybackBesideEpisodeDetails(t *testing.T) {
	for _, viewport := range []struct {
		name          string
		width, height int
	}{
		{"normal", 1320, 860},
		{"minimum-width", 1040, 860},
		{"minimum-height", 1320, 720},
		{"minimum-window", 1040, 720},
	} {
		for _, appearance := range []string{appearanceLight, appearanceDark} {
			t.Run(viewport.name+"/"+appearance, func(t *testing.T) {
				s := uiFixture(t, "episodes")
				s.LibraryID = s.Subscriptions[0].ID
				s.Appearance = appearance
				episode := s.Downloads[2]
				episode.Episode = "05"
				s.Downloads = []Download{episode}
				tester := ui.NewTester(s.view, viewport.width, viewport.height)
				for _, scale := range []float32{1, 2} {
					tester.SetPreferences(ui.Preferences{TextScale: scale, ReduceMotion: true})
					label, labelFound := tester.Find("第 5 集")
					title, titleFound := tester.Find(episode.Title)
					play, playFound := tester.Find("播放 " + episode.ID)
					if !labelFound || !titleFound || !playFound {
						t.Fatalf("text scale %.1f: missing episode details or playback", scale)
					}
					if play.X <= label.X || play.Y >= title.Y+title.H || play.Y+play.H <= label.Y {
						t.Errorf("text scale %.1f: playback is detached from the episode details: episode=%+v title=%+v play=%+v", scale, label, title, play)
					}
					if play.H < 40 || play.X+play.W > float32(viewport.width) || play.Y+play.H > float32(viewport.height) {
						t.Errorf("text scale %.1f: playback target is too small or outside the window: %+v", scale, play)
					}
				}
			})
		}
	}
}
