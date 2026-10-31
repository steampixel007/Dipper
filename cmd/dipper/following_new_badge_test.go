package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIFollowingNewBadgeOpensNewOnlyView(t *testing.T) {
	s := uiFixture(t, "following")
	sub := &s.Subscriptions[0]
	sub.NewCount = 1
	sub.NewTitles = map[string]bool{normalize(s.ResourceList[0].Title): true}
	tt := ui.NewTester(s.view, 1320, 860)

	uiClick(t, tt, "新资源 1 条 回归测试组甲")
	// The fixture has no source client; supply the history the detail loader
	// would have fetched.
	s.Groups = []string{"回归测试组甲"}
	s.GroupIDs = []string{"render-group-1"}
	s.Group = 0
	s.ResourceList = []Resource{
		{ID: "render-resource-1", Title: "[回归测试组甲] 星海旅记 - 01 [1080p][简日内嵌][HEVC].mkv"},
		{ID: "render-resource-3", Title: "[回归测试组甲] 星海旅记 - 02 [720p][CHS][AVC].mp4"},
	}
	tt.Frame()
	if s.Page != "detail" || !s.ViewResources || s.EditID != sub.ID || !s.OnlyNew {
		t.Fatalf("badge click state: page=%q view=%v edit=%q onlyNew=%v", s.Page, s.ViewResources, s.EditID, s.OnlyNew)
	}
	if tt.HasText("[720p][CHS][AVC]") {
		t.Fatalf("the opened view must filter to the pending release; texts=%q", tt.Texts())
	}
	if !tt.HasText("星海旅记 - 01") {
		t.Fatal("the pending release must stay visible")
	}
}
