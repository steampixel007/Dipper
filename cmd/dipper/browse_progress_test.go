package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIBrowseFollowedCardShowsProgress(t *testing.T) {
	s := uiFixture(t, "browse")
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "已追番", "已完成 1 个下载")

	s.Downloads = s.Downloads[:1]
	tt.Frame()
	if tt.HasText("已完成 1 个下载") {
		t.Fatalf("a card without completed downloads must not show progress; texts=%q", tt.Texts())
	}
}
