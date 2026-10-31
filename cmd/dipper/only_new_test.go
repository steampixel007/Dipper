package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIResourceViewFiltersToNewReleases(t *testing.T) {
	s := uiFixture(t, "detail")
	s.EditID = "render-sub-1"
	s.ViewResources = true
	s.Backfill = true
	s.Subscriptions[0].NewTitles = map[string]bool{normalize(s.ResourceList[0].Title): true}
	tt := ui.NewTester(s.view, 1320, 860)

	if _, ok := tt.Find("只看新资源"); !ok {
		t.Fatalf("the followed resource view must offer 只看新资源; texts=%q", tt.Texts())
	}
	uiClick(t, tt, "只看新资源")
	if tt.HasText("[720p][CHS][AVC]") {
		t.Fatalf("only-new must hide releases outside the pending set; texts=%q", tt.Texts())
	}
	if !tt.HasText("星海旅记 - 01") {
		t.Fatal("only-new must keep the pending release visible")
	}

	uiClick(t, tt, "只看新资源")
	if !tt.HasText("[720p][CHS][AVC]") {
		t.Fatal("toggling off must restore every row")
	}
}
