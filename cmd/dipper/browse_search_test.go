package main

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIBrowseSearchClearsAndResetsScroll(t *testing.T) {
	s := uiFixture(t, "browse")
	for i := 2; i <= 14; i++ {
		s.Catalog = append(s.Catalog, Anime{ID: fmt.Sprintf("browse-extra-%d", i), Title: fmt.Sprintf("星海旅记 第%d部", i), Day: i % 7})
	}
	tt := ui.NewTester(s.view, 1320, 860)
	s.CatalogScroll.Y = 400
	tt.Frame()
	if s.CatalogScroll.Y <= 0 {
		t.Fatalf("precondition: the catalog should stay scrolled, got %v", s.CatalogScroll.Y)
	}

	uiClick(t, tt, "搜索番剧")
	tt.Type("星海")
	if s.CatalogScroll.Y != 0 {
		t.Fatalf("typing a search kept the catalog scroll at %v", s.CatalogScroll.Y)
	}
	uiRequireText(t, tt, "星海旅记")

	uiClick(t, tt, "清除搜索")
	if s.Search != "" {
		t.Fatalf("the clear button left the search text %q", s.Search)
	}
	uiRequireText(t, tt, "雨后的星期天")
}
