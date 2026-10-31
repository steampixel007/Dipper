package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestUIResourceRowShowsDownloadState(t *testing.T) {
	s := uiFixture(t, "detail")
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "下载中 43%", "已在下载库", "下载失败")
	s.Downloads[0].Progress = 1
	s.Downloads[1].Failed = false
	tt.Frame()
	if tt.HasText("下载中 43%") {
		t.Error("a completed download still shows its progress on the resource row")
	}
	if tt.HasText("下载失败") {
		t.Error("a recovered download still shows failure on the resource row")
	}
	if !tt.HasText("已在下载库") {
		t.Fatal("recovered downloads do not read as in the library")
	}
}

func TestUIResourceRowPlayOpensDownloadedRelease(t *testing.T) {
	s := uiFixture(t, "detail")
	played := ""
	engine := &playTestEngine{files: []string{filepath.Join(s.Root, "星海旅记", "episode.mkv")}}
	s.Engine = engine
	s.PlayFile = func(path string) { played = path }
	updates := make(chan func(), 2)
	s.Update = func(fn func()) { updates <- fn }
	s.Downloads[0].Progress = 1
	tt := ui.NewTester(s.view, 1320, 860)
	if _, ok := tt.Find("播放"); !ok {
		t.Fatalf("a downloaded resource must offer 播放; texts=%q", tt.Texts())
	}
	uiClick(t, tt, "播放")
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for played == "" && s.SaveError == "" {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatal("play did not open a file")
		}
	}
	if played != filepath.Join(s.Root, "星海旅记", "episode.mkv") {
		t.Fatalf("played %q, want the downloaded episode", played)
	}
}
