package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIDownloadGroupsCollapseAndExpand(t *testing.T) {
	s := uiFixture(t, "downloads")
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "已分享 0.36 倍")
	uiClick(t, tt, "收起")
	if tt.HasText("已分享 0.36 倍") {
		t.Fatal("collapsed group still shows its download rows")
	}
	if !tt.HasText("展开") {
		t.Fatal("collapsed group lost its expand control")
	}
	uiClick(t, tt, "展开")
	uiRequireText(t, tt, "已分享 0.36 倍")
	if s.CollapsedDownloads["render-1"] {
		t.Fatal("expanding did not clear the collapsed state")
	}
}
