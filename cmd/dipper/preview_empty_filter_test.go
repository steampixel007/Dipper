package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIPreviewEmptyFilterOffersClear(t *testing.T) {
	s := uiFixture(t, "detail")
	tt := ui.NewTester(s.view, 1320, 860)

	if _, ok := tt.Find("没有符合筛选的资源"); ok {
		t.Fatal("an unfiltered preview must not show the empty state")
	}

	uiClick(t, tt, "只看已在下载库")
	s.EpisodeFilter = "03"
	tt.Frame()
	uiRequireText(t, tt, "没有符合筛选的资源")

	uiClick(t, tt, "清除筛选")
	if s.OnlyDownloaded || s.OnlyMatches || s.OnlyNew || s.EpisodeFilter != "" {
		t.Fatalf("clearing must reset the view filters: onlyDownloaded=%v onlyMatches=%v onlyNew=%v episode=%q", s.OnlyDownloaded, s.OnlyMatches, s.OnlyNew, s.EpisodeFilter)
	}
	uiRequireText(t, tt, "[回归测试组甲] 星海旅记 - 01")
}
