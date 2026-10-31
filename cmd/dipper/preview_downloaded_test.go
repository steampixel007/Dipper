package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIPreviewFiltersToDownloaded(t *testing.T) {
	s := uiFixture(t, "detail")
	s.ResourceList = append(s.ResourceList, Resource{
		ID:      "render-resource-4",
		Title:   "[回归测试组甲] 星海旅记 - 03 [1080p][简日内嵌][HEVC].mkv",
		Episode: "03",
	})
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "星海旅记 - 03")

	uiClick(t, tt, "只看已在下载库")
	if tt.HasText("星海旅记 - 03") {
		t.Fatal("the download filter left an undownloaded row visible")
	}
	uiRequireText(t, tt, "星海旅记 - 01", "星海旅记 - 02")

	uiClick(t, tt, "只看已在下载库")
	uiRequireText(t, tt, "星海旅记 - 03")
}
