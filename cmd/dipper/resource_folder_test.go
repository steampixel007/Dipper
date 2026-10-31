package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIResourceRowOpensDownloadFolder(t *testing.T) {
	s := uiFixture(t, "detail")
	var opened []string
	s.OpenFolder = func(folder string) { opened = append(opened, folder) }
	tt := ui.NewTester(s.view, 1320, 860)
	if _, ok := tt.Find("打开文件夹 render-resource-3"); !ok {
		t.Fatalf("a completed resource row must offer 打开文件夹; texts=%q", tt.Texts())
	}
	// Hide the first episode so the target row renders inside the viewport.
	s.EpisodeFilter = "02"
	tt.Frame()
	uiClick(t, tt, "打开文件夹 render-resource-3")
	if len(opened) != 1 || opened[0] != s.Downloads[2].Folder {
		t.Fatalf("opened %q, want the task folder %q", opened, s.Downloads[2].Folder)
	}

	if _, ok := tt.Find("打开文件夹 render-resource-1"); ok {
		t.Fatal("an in-flight resource row must not offer 打开文件夹")
	}
}
