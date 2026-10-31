package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIDownloadGroupOpensFolderWhileCollapsed(t *testing.T) {
	s := uiFixture(t, "downloads")
	for i := range s.Downloads {
		s.Downloads[i].Folder = filepath.Join(s.Dir, "星海旅记", s.Downloads[i].ID)
	}
	opened := ""
	s.OpenFolder = func(path string) { opened = path }
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "收起")
	uiClick(t, tt, "打开番剧文件夹")
	want := filepath.Join(s.Dir, "星海旅记")
	if opened != want {
		t.Fatalf("group header opened %q, want the anime directory %q", opened, want)
	}
}
