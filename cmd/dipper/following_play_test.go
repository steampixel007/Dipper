package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestUIFollowingCardPlaysLatestCompleted(t *testing.T) {
	s := uiFixture(t, "following")
	played := ""
	engine := &playTestEngine{files: []string{filepath.Join(s.Root, "星海旅记", "episode.mkv")}}
	s.Engine = engine
	s.PlayFile = func(path string) error { played = path; return nil }
	updates := make(chan func(), 2)
	s.Update = func(fn func()) { updates <- fn }
	s.Downloads[2].Episode = "02"
	s.Downloads = append(s.Downloads, Download{ID: "render-download-4", AnimeID: s.AnimeID, Episode: "05", Progress: 1})
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "播放第 05 集")
	if tt.HasText("播放第 02 集") {
		t.Fatal("the play shortcut should offer the newest completed episode")
	}

	uiClick(t, tt, "播放第 05 集")
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for played == "" && s.SaveError == "" {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatal("the follow card play shortcut did not open a file")
		}
	}
	if played != filepath.Join(s.Root, "星海旅记", "episode.mkv") {
		t.Fatalf("played %q, want the downloaded episode", played)
	}
}
