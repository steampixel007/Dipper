package main

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIChromeStaysNeutralAcrossThemes(t *testing.T) {
	for _, appearance := range []string{appearanceLight, appearanceDark} {
		t.Run(appearance, func(t *testing.T) {
			state := uiFixture(t, "browse")
			state.Appearance = appearance
			tester := ui.NewTester(state.view, 1040, 720)
			tester.SetPreferences(ui.Preferences{ReduceMotion: true})
			frame := tester.Image()
			for _, point := range []image.Point{
				image.Pt(8, 80), image.Pt(8, 400), image.Pt(8, 680),
				image.Pt(1032, 8), image.Pt(210, 680),
			} {
				red, green, blue, _ := frame.At(point.X, point.Y).RGBA()
				if spread := max(red, green, blue) - min(red, green, blue); spread > 10*257 {
					t.Errorf("chrome at %v carries a color cast: rgb=(%d,%d,%d), want neutral surroundings for artwork", point, red/257, green/257, blue/257)
				}
			}
		})
	}
}

func TestUISidebarCountSharesRowAndLargeTextRemainsReachable(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		state := uiFixture(t, "browse")
		state.Subscriptions[1].CheckFailed = false
		state.Downloads[1].Failed = true
		state.Downloads[1].Error = "连接超时，请重试"
		tester := ui.NewTester(func(c *ui.Context) {
			installTheme(c, false)
			state.sidebar(c)
		}, 1040, 720)
		tester.SetPreferences(ui.Preferences{TextScale: scale, ReduceMotion: true})
		download := requireLayoutBound(t, tester, "下载", 204, 720)
		failure := requireLayoutBound(t, tester, "1 失败", 204, 720)
		if scale == 1 {
			if delta := (download.Y + download.H/2) - (failure.Y + failure.H/2); delta < -2 || delta > 2 {
				t.Errorf("navigation count consumes a second row at normal text size: download=%+v failure=%+v", download, failure)
			}
		}
		tester.ClickAt(download.X+download.W/2, download.Y+download.H/2)
		if state.Page != "downloads" {
			t.Errorf("navigation at text scale %v did not open downloads: page=%s", scale, state.Page)
		}
	}
}
