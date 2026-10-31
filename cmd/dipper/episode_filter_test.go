package main

import (
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
