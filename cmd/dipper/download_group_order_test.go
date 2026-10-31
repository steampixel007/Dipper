package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIDownloadGroupSortsRowsByEpisode(t *testing.T) {
	s := uiFixture(t, "downloads")
	ep01, failedEp02, ep02 := s.Downloads[0], s.Downloads[1], s.Downloads[2]
	ep01.Episode, failedEp02.Episode, ep02.Episode = "01", "02", "02"
	s.Downloads = []Download{ep02, ep01, failedEp02}
	tt := ui.NewTester(s.view, 1320, 860)

	texts := tt.Texts()
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "星海旅记 - 01") || !strings.Contains(joined, "星海旅记 - 02") {
		t.Fatalf("expected both episodes rendered; texts=%q", texts)
	}
	first02 := strings.Index(joined, "星海旅记 - 02")
	first01 := strings.Index(joined, "星海旅记 - 01")
	if first01 > first02 {
		t.Fatalf("group rows must read in episode order: 01 at %d, 02 at %d", first01, first02)
	}
}
