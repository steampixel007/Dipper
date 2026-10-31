package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIDownloadRowsShowEpisodeBadge(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads[0].Episode = "01"
	s.Downloads[2].Episode = "02"
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "第 01 集", "第 02 集")

	s.Downloads[0].Episode = ""
	tt.Frame()
	if tt.HasText("第 01 集") {
		t.Fatalf("a task without a known episode must not show the badge; texts=%q", tt.Texts())
	}
	uiRequireText(t, tt, "第 02 集")
}
