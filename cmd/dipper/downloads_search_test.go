package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIDownloadsSearchFiltersGroups(t *testing.T) {
	s := uiFixture(t, "downloads")
	first, second := s.Downloads[0], s.Downloads[2]
	second.AnimeID = "render-2"
	second.Title = "[回归测试组乙] 雨后的星期天 - 01 [1080p][简日内嵌][HEVC].mkv"
	second.Folder = filepath.Join(s.Root, "雨后的星期天")
	second.ID = "render-download-3b"
	s.Downloads = []Download{first, second}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "星海旅记", "雨后的星期天")

	uiClick(t, tt, "搜索下载")
	tt.Type("星期天")
	tt.Frame()
	if tt.HasText("星海旅记") {
		t.Fatalf("search must hide the non-matching anime group; texts=%q", tt.Texts())
	}
	uiRequireText(t, tt, "雨后的星期天")

	uiClick(t, tt, "搜索下载")
	tt.Type("不存在的番剧名")
	tt.Frame()
	uiRequireText(t, tt, "没有匹配的下载")

	// The release title also matches the search.
	uiClick(t, tt, "搜索下载")
	tt.Type("回归测试组乙")
	tt.Frame()
	if tt.HasText("星海旅记") {
		t.Fatalf("release-title search must hide the non-matching anime group; texts=%q", tt.Texts())
	}
	uiRequireText(t, tt, "雨后的星期天")

	uiClick(t, tt, "清除搜索")
	tt.Frame()
	uiRequireText(t, tt, "星海旅记", "雨后的星期天")
}
