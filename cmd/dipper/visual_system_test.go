package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUISidebarUsesSharedSelectionWash(t *testing.T) {
	for _, appearance := range []string{appearanceLight, appearanceDark} {
		t.Run(appearance, func(t *testing.T) {
			state := uiFixture(t, "browse")
			state.Appearance = appearance
			tester := ui.NewTester(state.view, 1040, 720)
			tester.SetPreferences(ui.Preferences{ReduceMotion: true})
			palette := lightPalette
			if appearance == appearanceDark {
				palette = darkPalette
			}
			frame := tester.Image()
			for _, sample := range []struct {
				name  string
				point image.Point
				fill  ui.Color
			}{
				{"selected navigation", image.Pt(180, 114), palette.blueWash},
				{"inactive navigation", image.Pt(180, 166), palette.chrome},
			} {
				pixel := color.NRGBAModel.Convert(frame.At(sample.point.X, sample.point.Y)).(color.NRGBA)
				want := color.NRGBA{R: sample.fill.R, G: sample.fill.G, B: sample.fill.B, A: 255}
				if pixel != want {
					t.Errorf("%s fill=%v, want %v", sample.name, pixel, want)
				}
			}
		})
	}
}

func TestUISidebarUsesAppBadge(t *testing.T) {
	for _, appearance := range []string{appearanceLight, appearanceDark} {
		t.Run(appearance, func(t *testing.T) {
			state := uiFixture(t, "browse")
			state.Appearance = appearance
			tester := ui.NewTester(state.view, 1040, 720)
			tester.SetPreferences(ui.Preferences{ReduceMotion: true})
			pixel := color.NRGBAModel.Convert(tester.Image().At(28, 30)).(color.NRGBA)
			want := color.NRGBA{R: 207, G: 226, B: 255, A: 255}
			if pixel != want {
				t.Errorf("sidebar badge fill=%v, want %v from the app icon", pixel, want)
			}
		})
	}
}

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
				if spread := max(red, green, blue) - min(red, green, blue); spread > 14*257 {
					t.Errorf("chrome at %v carries a color cast: rgb=(%d,%d,%d), want neutral surroundings for artwork", point, red/257, green/257, blue/257)
				}
				if blue < green || green < red {
					t.Errorf("chrome at %v has a competing tint: rgb=(%d,%d,%d), want cool neutrals", point, red/257, green/257, blue/257)
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
