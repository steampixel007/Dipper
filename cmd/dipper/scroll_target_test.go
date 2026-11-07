package main

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIScrollToReachesClippedDownloadAction(t *testing.T) {
	for _, height := range []int{720, 760, 800, 840, 860, 900, 940, 980} {
		t.Run(fmt.Sprint(height), func(t *testing.T) {
			state := uiFixture(t, "downloads")
			tester := ui.NewTester(state.view, 1320, height)
			uiScrollTo(t, tester, "停止分享")
			uiClick(t, tester, "停止分享")
		})
	}
}
