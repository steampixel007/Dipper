package main

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func countNonBackgroundPixels(img *image.RGBA, rect ui.Rect, bg ui.Color) int {
	bounds := image.Rect(int(rect.X), int(rect.Y), int(rect.X+rect.W), int(rect.Y+rect.H)).Intersect(img.Bounds())
	want := color.NRGBA{R: bg.R, G: bg.G, B: bg.B, A: 255}
	diff := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			dr := int(got.R) - int(want.R)
			dg := int(got.G) - int(want.G)
			db := int(got.B) - int(want.B)
			if dr < 0 {
				dr = -dr
			}
			if dg < 0 {
				dg = -dg
			}
			if db < 0 {
				db = -db
			}
			if dr+dg+db > 18 {
				diff++
			}
		}
	}
	return diff
}

func TestIntentRevealHidesSecondaryAndShowsOnHoverFocusAndScroll(t *testing.T) {
	t.Run("episode_row_rest_hover_tab", func(t *testing.T) {
		s := uiFixture(t, "episodes")
		s.LibraryID = s.Subscriptions[0].ID
		s.ReduceEffects = false
		tt := ui.NewTester(s.view, 1320, 860)
		tt.Advance(200 * time.Millisecond)

		completed := s.Downloads[2]
		playRect := requireLayoutBound(t, tt, "播放 "+completed.ID, 1320, 860)
		folderRect := requireLayoutBound(t, tt, "打开文件夹 "+completed.ID, 1320, 860)
		titleRect := requireLayoutBound(t, tt, completed.Title, 1320, 860)

		// At rest: primary play action is visible, secondary folder button is visually transparent.
		if pixels := countNonBackgroundPixels(tt.Image(), playRect, lightPalette.canvas); pixels < 20 {
			t.Fatalf("primary play button is hidden at rest: %d non-bg pixels", pixels)
		}
		if pixels := countNonBackgroundPixels(tt.Image(), folderRect, lightPalette.canvas); pixels != 0 {
			t.Fatalf("secondary folder action is visible at rest: %d non-bg pixels", pixels)
		}

		// Hovering the row reveals secondary action over 130 ms without moving layout bounds.
		tt.Move(titleRect.X+20, titleRect.Y+titleRect.H/2)
		tt.Advance(150 * time.Millisecond)
		folderAfterHover := requireLayoutBound(t, tt, "打开文件夹 "+completed.ID, 1320, 860)
		if folderAfterHover != folderRect {
			t.Fatalf("intent reveal moved layout: before=%+v after=%+v", folderRect, folderAfterHover)
		}
		if pixels := countNonBackgroundPixels(tt.Image(), folderAfterHover, lightPalette.chrome); pixels < 10 {
			t.Fatalf("secondary folder action did not reveal on hover: %d non-bg pixels", pixels)
		}

		// Moving pointer away hides it again.
		tt.Move(10, 10)
		tt.Advance(180 * time.Millisecond)
		if pixels := countNonBackgroundPixels(tt.Image(), folderRect, lightPalette.canvas); pixels != 0 {
			t.Fatalf("secondary folder action stayed visible after hover left: %d non-bg pixels", pixels)
		}

		// Tabbing keyboard focus into the row reveals secondary controls too.
		foundFocus := false
		for step := 0; step < 60; step++ {
			tt.Key(0, ui.KeyTab)
			tt.Advance(150 * time.Millisecond)
			if tt.Focused("打开文件夹 "+completed.ID) || tt.Focused("播放 "+completed.ID) {
				foundFocus = true
				break
			}
		}
		if !foundFocus {
			t.Fatal("could not reach episode row controls via Tab")
		}
		if pixels := countNonBackgroundPixels(tt.Image(), folderRect, lightPalette.canvas); pixels < 10 {
			t.Fatalf("secondary folder action did not reveal on keyboard focus within row: %d non-bg pixels", pixels)
		}
	})

	t.Run("downloads_retry_and_scroll", func(t *testing.T) {
		s := uiFixture(t, "downloads")
		s.ReduceEffects = false
		s.Downloads[0].Failed = true
		s.Downloads[0].Error = "连接超时"
		for i := 0; i < 6; i++ {
			extra := s.Downloads[1]
			extra.ID = "extra-" + string(rune('a'+i))
			extra.Title = "额外任务 " + extra.ID
			s.Downloads = append(s.Downloads, extra)
		}
		tt := ui.NewTester(s.view, 1320, 860)
		tt.Advance(200 * time.Millisecond)

		retryRect := requireLayoutBound(t, tt, "重试", 1320, 860)
		if pixels := countNonBackgroundPixels(tt.Image(), retryRect, lightPalette.canvas); pixels < 10 {
			t.Fatalf("failure retry action must never be hidden at rest: %d non-bg pixels", pixels)
		}
		delRect := requireLayoutBound(t, tt, "删除", 1320, 860)
		if pixels := countNonBackgroundPixels(tt.Image(), delRect, lightPalette.canvas); pixels != 0 {
			t.Fatalf("secondary delete action should be hidden at rest: %d non-bg pixels", pixels)
		}

		// Scrolling the list brings rows toward the viewport edge and reveals secondary controls.
		tt.Scroll(700, 500, 0, 120)
		tt.Advance(160 * time.Millisecond)
		afterScroll := requireLayoutBound(t, tt, "删除", 1320, 860)
		if pixels := countNonBackgroundPixels(tt.Image(), afterScroll, lightPalette.canvas); pixels < 10 {
			t.Fatalf("scrolling row near viewport edge did not reveal secondary controls: %d non-bg pixels (rect=%+v)", pixels, afterScroll)
		}
	})
}
