package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIBrowseFollowedCardHidesDownloadCount(t *testing.T) {
	state := uiFixture(t, "browse")
	tester := ui.NewTester(state.view, 1320, 860)
	uiRequireText(t, tester, "已追番")
	if tester.HasText("已完成 1 个下载") {
		t.Fatalf("browse cards show a completed download count; texts=%q", tester.Texts())
	}
}
