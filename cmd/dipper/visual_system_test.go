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
				if red < green || green < blue {
					t.Errorf("chrome at %v has a cold tint: rgb=(%d,%d,%d), want warm neutrals", point, red/257, green/257, blue/257)
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

func TestUIDarkPrimaryButtonUsesWarmIvoryMonochromeAccent(t *testing.T) {
	state := uiFixture(t, "episodes")
	state.LibraryID = state.Subscriptions[0].ID
	state.Appearance = appearanceDark
	tester := ui.NewTester(state.view, 1320, 860)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})

	playBounds := requireLayoutBound(t, tester, "播放 "+state.Downloads[2].ID, 1320, 860)
	frame := tester.Image()
	sample := color.NRGBAModel.Convert(frame.At(int(playBounds.X+6), int(playBounds.Y+6))).(color.NRGBA)
	want := color.NRGBA{R: darkPalette.accent.R, G: darkPalette.accent.G, B: darkPalette.accent.B, A: 255}
	if sample != want {
		t.Fatalf("dark primary play button fill=%v, want warm ivory accent %v", sample, want)
	}
	if spread := max(int(want.R), max(int(want.G), int(want.B))) - min(int(want.R), min(int(want.G), int(want.B))); spread > 14 {
		t.Fatalf("dark primary button fill %v carries a colored cast, want warm monochrome ivory", want)
	}
	if want.R < want.G || want.G < want.B {
		t.Fatalf("dark primary button fill %v has a cold tint, want warm ivory", want)
	}
	if darkPalette.accentText.R > 40 || darkPalette.accentText.G > 40 || darkPalette.accentText.B > 40 {
		t.Fatalf("dark primary button label color=%v, want dark ink on ivory", darkPalette.accentText)
	}
}

func TestUISidebarUtilitiesLeftAlignWithPrimaryNav(t *testing.T) {
	state := uiFixture(t, "browse")
	tester := ui.NewTester(state.view, 1040, 720)
	tester.SetPreferences(ui.Preferences{TextScale: 1, ReduceMotion: true})

	frame := tester.Image()
	following := requireLayoutBound(t, tester, "我的追番", 1040, 720)
	settings := requireLayoutBound(t, tester, "设置", 1040, 720)
	help := requireLayoutBound(t, tester, "追番小指南", 1040, 720)
	bg := color.NRGBAModel.Convert(color.NRGBA{R: lightPalette.chrome.R, G: lightPalette.chrome.G, B: lightPalette.chrome.B, A: 255}).(color.NRGBA)
	firstInkX := func(bounds ui.Rect) int {
		for x := int(bounds.X) + 4; x < int(bounds.X+bounds.W)-4; x++ {
			for y := int(bounds.Y) + 4; y < int(bounds.Y+bounds.H)-4; y++ {
				px := color.NRGBAModel.Convert(frame.At(x, y)).(color.NRGBA)
				if px != bg {
					return x
				}
			}
		}
		return -1
	}
	navX := firstInkX(following)
	for _, item := range []struct {
		name   string
		bounds ui.Rect
	}{{"设置", settings}, {"追番小指南", help}} {
		gotX := firstInkX(item.bounds)
		if gotX < 0 || gotX-navX < -4 || gotX-navX > 4 {
			t.Errorf("sidebar utility %q content starts at x=%d, want left-aligned with primary nav x=%d", item.name, gotX, navX)
		}
	}
}

func TestUISettingsAppearanceUsesVisualPreviewCards(t *testing.T) {
	state := uiFixture(t, "settings")
	tester := ui.NewTester(state.view, 1320, 860)
	tester.SetPreferences(ui.Preferences{TextScale: 1, ReduceMotion: true})

	for _, label := range []string{appearanceText.light, appearanceText.dark, appearanceText.system} {
		bounds := requireLayoutBound(t, tester, label, 1320, 860)
		if bounds.H < 60 {
			t.Errorf("appearance choice %q height=%.1f, want preview card at least 60 DIP tall", label, bounds.H)
		}
	}
}
