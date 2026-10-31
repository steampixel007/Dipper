package main

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIResourceRowsFilterByEpisode(t *testing.T) {
	s := uiFixture(t, "detail")
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "星海旅记 - 01", "星海旅记 - 02")

	uiClick(t, tt, "本集匹配 2 条")
	if tt.HasText("星海旅记 - 01") {
		t.Fatal("episode filter left other episodes visible")
	}
	uiRequireText(t, tt, "星海旅记 - 02", "只看第 02 集")

	uiClick(t, tt, "清除")
	uiRequireText(t, tt, "星海旅记 - 01")
}

func TestUIEpisodeFilterPreservesScroll(t *testing.T) {
	s := uiFixture(t, "detail")
	for i := 3; i <= 13; i++ {
		ep := fmt.Sprintf("%02d", i)
		releases := 1
		if i == 11 {
			releases = 3
		}
		for j := 0; j < releases; j++ {
			s.ResourceList = append(s.ResourceList, Resource{
				ID:      "scroll-resource-" + ep + "-" + strconv.Itoa(j),
				Title:   "[回归测试组甲] 星海旅记 - " + ep + " [1080p][简日内嵌][HEVC].mkv",
				Episode: ep,
			})
		}
	}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "本集匹配 3 条")

	const targetRow = "[回归测试组甲] 星海旅记 - 11 [1080p][简日内嵌][HEVC].mkv"
	s.ResourceScroll.Y = 400
	tt.Frame()
	for i := 0; i < 8; i++ {
		r, ok := tt.Find(targetRow)
		if !ok {
			t.Fatalf("scroll target %q vanished", targetRow)
		}
		if r.H > 0 && r.Y >= 520 && r.Y <= 780 {
			break
		}
		s.ResourceScroll.Y += r.Y - 620
		tt.Frame()
	}
	before := s.ResourceScroll.Y
	if before <= 0 {
		t.Fatalf("scroll offset should be nonzero after scrolling down, got %v", before)
	}

	uiClick(t, tt, "本集匹配 3 条")
	uiRequireText(t, tt, "只看第 11 集")
	s.ResourceScroll.Y = 0
	tt.Frame()
	uiClick(t, tt, "本集匹配 3 条")
	if tt.HasText("只看第 11 集") {
		t.Fatal("episode filter survived its own clear click")
	}
	uiRequireText(t, tt, "星海旅记 - 01")
	if restored := s.ResourceScroll.Y; restored < before-1 || restored > before+1 {
		t.Fatalf("clearing the filter moved the scroll position from %v to %v", before, restored)
	}
}
