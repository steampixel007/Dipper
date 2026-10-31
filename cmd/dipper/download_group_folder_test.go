package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIDownloadGroupOpensFolderWhileCollapsed(t *testing.T) {
	s := uiFixture(t, "downloads")
	opened := ""
	s.OpenFolder = func(path string) { opened = path }
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "收起")
	uiClick(t, tt, "打开文件夹")
	if opened != s.Downloads[0].Folder {
		t.Fatalf("group header opened %q, want the anime folder %q", opened, s.Downloads[0].Folder)
	}
}
