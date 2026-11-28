package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSafeTriangleGeometry(t *testing.T) {
	subRight := ui.Rect{X: 300, Y: 100, W: 200, H: 120}
	anchor := ui.Point{X: 200, Y: 140}

	// Diagonal path from anchor toward top/bottom of submenu stays inside.
	if !pointInSafeTriangle(ui.Point{X: 250, Y: 160}, anchor, subRight, 8) {
		t.Fatal("diagonal point toward submenu must be inside safe triangle")
	}
	// Extended 8 DIP margin above top edge of submenu is included.
	if !pointInSafeTriangle(ui.Point{X: 298, Y: 94}, anchor, subRight, 8) {
		t.Fatal("point inside 8 DIP extended top margin must be inside safe triangle")
	}
	// Straight down away from submenu slope is outside.
	if pointInSafeTriangle(ui.Point{X: 200, Y: 200}, anchor, subRight, 8) {
		t.Fatal("point straight below anchor must be outside safe triangle")
	}
	// Moving left away from right-side submenu is outside.
	if pointInSafeTriangle(ui.Point{X: 160, Y: 140}, anchor, subRight, 8) {
		t.Fatal("point moving away from submenu must be outside safe triangle")
	}
	// Flipped submenu on the left of parent menu.
	subLeft := ui.Rect{X: 40, Y: 100, W: 140, H: 120}
	anchorRight := ui.Point{X: 260, Y: 140}
	if !pointInSafeTriangle(ui.Point{X: 220, Y: 170}, anchorRight, subLeft, 8) {
		t.Fatal("diagonal point toward left-flipped submenu must be inside safe triangle")
	}
	if pointInSafeTriangle(ui.Point{X: 280, Y: 140}, anchorRight, subLeft, 8) {
		t.Fatal("point moving away from left-flipped submenu must be outside safe triangle")
	}
}

func TestCascadingMenuOpenCloseDelaysSafeTriangleAndKeyboard(t *testing.T) {
	openHeroMenu := func(t *testing.T, reduceMotion bool) (*State, *ui.Tester, ui.Rect, ui.Rect) {
		t.Helper()
		s := uiFixture(t, "episodes")
		s.LibraryID = s.Subscriptions[0].ID
		s.ReduceEffects = reduceMotion
		tt := ui.NewTester(s.view, 1320, 860)
		if reduceMotion {
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		}
		uiClick(t, tt, "更多操作")
		if !s.LibraryMenuOpen {
			t.Fatal("clicking library menu trigger did not open cascading menu")
		}
		subTrigger := requireLayoutBound(t, tt, "文件与记录", 1320, 860)
		cancelItem := requireLayoutBound(t, tt, "取消追番", 1320, 860)
		return s, tt, subTrigger, cancelItem
	}

	t.Run("open_close_delays_with_reduced_motion", func(t *testing.T) {
		for _, reduce := range []bool{false, true} {
			_, tt, subTrigger, _ := openHeroMenu(t, reduce)
			subLabel := "打开番剧文件夹"

			// Hover submenu trigger: must NOT open before 120 ms.
			tt.Move(subTrigger.X+subTrigger.W/2, subTrigger.Y+subTrigger.H/2)
			tt.Advance(80 * time.Millisecond)
			if _, ok := tt.Find(subLabel); ok {
				t.Fatalf("reduce=%v: submenu opened before 120 ms open delay", reduce)
			}

			// Advancing past 120 ms opens submenu.
			tt.Advance(60 * time.Millisecond)
			subItem := requireLayoutBound(t, tt, subLabel, 1320, 860)

			// Leave both parent item and submenu: must stay open at 200 ms.
			tt.Move(40, 40)
			tt.Advance(200 * time.Millisecond)
			if _, ok := tt.Find(subLabel); !ok {
				t.Fatalf("reduce=%v: submenu closed before 300 ms close delay", reduce)
			}

			// Re-entering submenu cancels the close timer.
			tt.Move(subItem.X+subItem.W/2, subItem.Y+subItem.H/2)
			tt.Advance(200 * time.Millisecond)
			if _, ok := tt.Find(subLabel); !ok {
				t.Fatalf("reduce=%v: re-entering submenu did not cancel close timer", reduce)
			}

			// Leaving again past 300 ms closes submenu.
			tt.Move(40, 40)
			tt.Advance(340 * time.Millisecond)
			if _, ok := tt.Find(subLabel); ok {
				t.Fatalf("reduce=%v: submenu stayed open past 300 ms close delay", reduce)
			}
		}
	})

	t.Run("safe_triangle_protection", func(t *testing.T) {
		_, tt, subTrigger, cancelItem := openHeroMenu(t, true)
		subLabel := "打开番剧文件夹"

		// Click opens submenu immediately (no 120 ms wait).
		tt.ClickAt(subTrigger.X+subTrigger.W/2, subTrigger.Y+subTrigger.H/2)
		subItem := requireLayoutBound(t, tt, subLabel, 1320, 860)

		// Determine direction toward submenu (left or right of parent menu).
		dir := float32(1)
		if subItem.X < subTrigger.X {
			dir = -1
		}
		// Start on the far side of subTrigger so diagonal step across cancelItem moves toward submenu.
		startX := subTrigger.X + subTrigger.W/2 - dir*40
		tt.Move(startX, subTrigger.Y+subTrigger.H/2)
		tt.Frame()

		// Step diagonally onto cancelItem (below subTrigger) moving toward submenu edge:
		// inside the safe triangle -> submenu MUST stay open!
		diagX := startX + dir*90
		diagY := cancelItem.Y + cancelItem.H/2
		tt.Move(diagX, diagY)
		tt.Advance(40 * time.Millisecond)
		if _, ok := tt.Find(subLabel); !ok {
			t.Fatal("moving diagonally inside safe triangle closed submenu prematurely")
		}

		// Stopping inside parent menu on cancelItem for > 120 ms lets cancelItem take over.
		tt.Advance(140 * time.Millisecond)
		if _, ok := tt.Find(subLabel); ok {
			t.Fatal("stopping inside parent menu past open delay did not hand over to hovered item")
		}

		// Re-open submenu and move AWAY from submenu (outside safe triangle) onto cancelItem:
		// must yield immediately!
		tt.ClickAt(subTrigger.X+subTrigger.W/2, subTrigger.Y+subTrigger.H/2)
		requireLayoutBound(t, tt, subLabel, 1320, 860)
		tt.Move(subTrigger.X+subTrigger.W/2, subTrigger.Y+subTrigger.H/2)
		tt.Frame()
		tt.Move(subTrigger.X+subTrigger.W/2-dir*30, cancelItem.Y+cancelItem.H/2)
		tt.Frame()
		if _, ok := tt.Find(subLabel); ok {
			t.Fatal("moving outside safe triangle onto sibling did not close submenu immediately")
		}
	})

	t.Run("keyboard_navigation", func(t *testing.T) {
		s, tt, _, _ := openHeroMenu(t, true)
		openedFolder := ""
		s.OpenFolder = func(path string) { openedFolder = path }
		subLabel := "打开番剧文件夹"

		// Move down 5 times to reach "文件与记录" (index 4: view, edit, pause, check, files-group).
		for i := 0; i < 5; i++ {
			tt.Key(0, ui.KeyDown)
		}
		// Right arrow opens submenu immediately.
		tt.Key(0, ui.KeyRight)
		requireLayoutBound(t, tt, subLabel, 1320, 860)

		// Left arrow closes submenu immediately while keeping parent menu open.
		tt.Key(0, ui.KeyLeft)
		if _, ok := tt.Find(subLabel); ok {
			t.Fatal("Left arrow did not close submenu immediately")
		}
		if !s.LibraryMenuOpen {
			t.Fatal("Left arrow closed the entire menu instead of only the submenu")
		}

		// Right arrow again, then Enter activates first submenu item (打开番剧文件夹).
		tt.Key(0, ui.KeyRight)
		requireLayoutBound(t, tt, subLabel, 1320, 860)
		tt.Key(0, ui.KeyEnter)
		if openedFolder != s.Subscriptions[0].Folder {
			t.Fatalf("Enter in submenu opened %q, want %q", openedFolder, s.Subscriptions[0].Folder)
		}
		if s.LibraryMenuOpen {
			t.Fatal("activating a submenu item left the menu open")
		}

		// Open menu again, open submenu, Escape closes submenu first then second Escape closes menu.
		uiClick(t, tt, "更多操作")
		for i := 0; i < 5; i++ {
			tt.Key(0, ui.KeyDown)
		}
		tt.Key(0, ui.KeyRight)
		requireLayoutBound(t, tt, subLabel, 1320, 860)
		tt.Key(0, ui.KeyEscape)
		if _, ok := tt.Find(subLabel); ok || !s.LibraryMenuOpen {
			t.Fatalf("first Escape should close only submenu: menuOpen=%v", s.LibraryMenuOpen)
		}
		tt.Key(0, ui.KeyEscape)
		if s.LibraryMenuOpen {
			t.Fatal("second Escape did not close parent menu")
		}
	})
}
