package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	cascadingOpenDelay  = 120 * time.Millisecond
	cascadingCloseDelay = 300 * time.Millisecond
	safeTrianglePadding = float32(8)
)

type cascadingMenuItem struct {
	Key         string
	Label       string
	AccessLabel string
	Disabled    bool
	Danger      bool
	Separator   bool
	Action      func()
	Submenu     []cascadingMenuItem
}

type cascadingMenuKey struct {
	name string
}

type cascadingMenuState struct {
	wasOpen        bool
	openedAt       time.Time
	activeIdx      int
	openSubIdx     int
	subActiveIdx   int
	focusLevel     int
	pendingOpenIdx int
	pendingOpenAt  time.Time
	pendingCloseAt time.Time
	lastPtr        ui.Point
	hasLastPtr     bool
	ptrMovedAt     time.Time
	triAnchor      ui.Point
	hasTriAnchor   bool
	parentRect     ui.Rect
	subRect        ui.Rect
	itemRects      []ui.Rect
}

func (st *cascadingMenuState) reset() {
	st.wasOpen = false
	st.openedAt = time.Time{}
	st.activeIdx = -1
	st.openSubIdx = -1
	st.subActiveIdx = -1
	st.focusLevel = 0
	st.pendingOpenIdx = -1
	st.pendingOpenAt = time.Time{}
	st.pendingCloseAt = time.Time{}
	st.hasLastPtr = false
	st.ptrMovedAt = time.Time{}
	st.hasTriAnchor = false
	st.parentRect = ui.Rect{}
	st.subRect = ui.Rect{}
	st.itemRects = st.itemRects[:0]
}

// pointInSafeTriangle reports whether pointer p lies within the triangle formed
// by anchor and the top/bottom edges of subRect nearest anchor (extended by
// pad DIPs) while on the approaching side of subRect.
func pointInSafeTriangle(p, anchor ui.Point, subRect ui.Rect, pad float32) bool {
	if subRect.W <= 0 || subRect.H <= 0 {
		return false
	}
	var edgeX float32
	switch {
	case anchor.X <= subRect.X:
		edgeX = subRect.X
	case anchor.X >= subRect.X+subRect.W:
		edgeX = subRect.X + subRect.W
	default:
		return false
	}
	// Reject movement away from the submenu's nearest edge.
	if (edgeX-anchor.X)*(p.X-anchor.X) < 0 {
		return false
	}
	v0 := anchor
	v1 := ui.Point{X: edgeX, Y: subRect.Y - pad}
	v2 := ui.Point{X: edgeX, Y: subRect.Y + subRect.H + pad}
	cross := func(u, v, w ui.Point) float32 {
		return (u.X-w.X)*(v.Y-w.Y) - (v.X-w.X)*(u.Y-w.Y)
	}
	d1 := cross(p, v0, v1)
	d2 := cross(p, v1, v2)
	d3 := cross(p, v2, v0)
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

func towardSubmenuEdge(prev, cur ui.Point, subRect ui.Rect) bool {
	if subRect.W <= 0 || subRect.H <= 0 {
		return false
	}
	edgeX := subRect.X
	if prev.X >= subRect.X+subRect.W {
		edgeX = subRect.X + subRect.W
	}
	prevDist := math.Abs(float64(edgeX - prev.X))
	curDist := math.Abs(float64(edgeX - cur.X))
	return curDist < prevDist-0.25
}

func rectContainsPoint(r ui.Rect, p ui.Point) bool {
	return r.W > 0 && r.H > 0 && p.X >= r.X && p.X <= r.X+r.W && p.Y >= r.Y && p.Y <= r.Y+r.H
}

func firstEnabledMenuItem(items []cascadingMenuItem) int {
	for i, it := range items {
		if !it.Separator && !it.Disabled {
			return i
		}
	}
	return -1
}

func stepMenuItem(items []cascadingMenuItem, cur, delta int) int {
	n := len(items)
	if n == 0 {
		return -1
	}
	start := cur
	if start < 0 || start >= n {
		if delta >= 0 {
			return firstEnabledMenuItem(items)
		}
		for i := n - 1; i >= 0; i-- {
			if !items[i].Separator && !items[i].Disabled {
				return i
			}
		}
		return -1
	}
	for step := 1; step <= n; step++ {
		idx := ((start+delta*step)%n + n) % n
		if !items[idx].Separator && !items[idx].Disabled {
			return idx
		}
	}
	return cur
}

// cascadingMenu draws a desktop menu with 120 ms submenu open delay, 300 ms
// submenu close delay, safe-triangle pointer protection, and keyboard arrows.
func cascadingMenu(c *ui.Context, key string, anchor ui.Element, open *bool, items []cascadingMenuItem) {
	st := ui.Local(anchor, cascadingMenuKey{name: key}, func() cascadingMenuState {
		var init cascadingMenuState
		init.reset()
		return init
	})
	if !*open {
		if st.wasOpen {
			st.reset()
		}
		return
	}
	now := c.Now()
	if !st.wasOpen {
		st.reset()
		st.wasOpen = true
		st.openedAt = now
	}
	pal := colors(c)
	winW, winH := c.Size()
	menuWidth := max(float32(224), textSize(c, textControl)*13)

	// Keyboard navigation (works across parent menu A and submenu B).
	c.OnShortcut(0, ui.KeyDown, func() {
		if st.focusLevel == 1 && st.openSubIdx >= 0 && st.openSubIdx < len(items) {
			st.subActiveIdx = stepMenuItem(items[st.openSubIdx].Submenu, st.subActiveIdx, 1)
			return
		}
		st.activeIdx = stepMenuItem(items, st.activeIdx, 1)
		if st.openSubIdx >= 0 && st.activeIdx != st.openSubIdx {
			st.openSubIdx = -1
			st.subActiveIdx = -1
			st.pendingCloseAt = time.Time{}
		}
	})
	c.OnShortcut(0, ui.KeyUp, func() {
		if st.focusLevel == 1 && st.openSubIdx >= 0 && st.openSubIdx < len(items) {
			st.subActiveIdx = stepMenuItem(items[st.openSubIdx].Submenu, st.subActiveIdx, -1)
			return
		}
		st.activeIdx = stepMenuItem(items, st.activeIdx, -1)
		if st.openSubIdx >= 0 && st.activeIdx != st.openSubIdx {
			st.openSubIdx = -1
			st.subActiveIdx = -1
			st.pendingCloseAt = time.Time{}
		}
	})
	c.OnShortcut(0, ui.KeyRight, func() {
		if st.focusLevel == 0 && st.activeIdx >= 0 && st.activeIdx < len(items) {
			it := items[st.activeIdx]
			if !it.Disabled && len(it.Submenu) > 0 {
				st.openSubIdx = st.activeIdx
				st.focusLevel = 1
				st.subActiveIdx = firstEnabledMenuItem(it.Submenu)
				st.pendingOpenIdx = -1
				st.pendingOpenAt = time.Time{}
				st.pendingCloseAt = time.Time{}
			}
		}
	})
	c.OnShortcut(0, ui.KeyLeft, func() {
		if st.openSubIdx >= 0 {
			st.openSubIdx = -1
			st.subActiveIdx = -1
			st.focusLevel = 0
			st.pendingCloseAt = time.Time{}
			st.hasTriAnchor = false
		}
	})
	c.OnShortcut(0, ui.KeyEscape, func() {
		if st.openSubIdx >= 0 {
			st.openSubIdx = -1
			st.subActiveIdx = -1
			st.focusLevel = 0
			st.pendingCloseAt = time.Time{}
			st.hasTriAnchor = false
			return
		}
		*open = false
	})
	activateCurrent := func() {
		if st.focusLevel == 1 && st.openSubIdx >= 0 && st.openSubIdx < len(items) {
			sub := items[st.openSubIdx].Submenu
			if st.subActiveIdx >= 0 && st.subActiveIdx < len(sub) && !sub[st.subActiveIdx].Disabled {
				action := sub[st.subActiveIdx].Action
				*open = false
				if action != nil {
					action()
				}
			}
			return
		}
		if st.activeIdx >= 0 && st.activeIdx < len(items) {
			it := items[st.activeIdx]
			if it.Disabled || it.Separator {
				return
			}
			if len(it.Submenu) > 0 {
				st.openSubIdx = st.activeIdx
				st.focusLevel = 1
				st.subActiveIdx = firstEnabledMenuItem(it.Submenu)
				st.pendingOpenIdx = -1
				st.pendingOpenAt = time.Time{}
				st.pendingCloseAt = time.Time{}
				return
			}
			action := it.Action
			*open = false
			if action != nil {
				action()
			}
		}
	}
	c.OnShortcut(0, ui.KeyEnter, activateCurrent)
	c.OnShortcut(0, ui.KeySpace, activateCurrent)

	ui.Overlay(c, func() {
		tracker := ui.Box(c.Key(key+"-tracker")).Absolute().Top(0).Left(0).Size(winW, winH)
		px, py, overWin := tracker.PointerPosition()
		ptr := ui.Point{X: px, Y: py}

		prevPtr := st.lastPtr
		hadPrev := st.hasLastPtr
		moved := false
		if overWin {
			moved = !st.hasLastPtr || math.Abs(float64(ptr.X-st.lastPtr.X)) > 0.5 || math.Abs(float64(ptr.Y-st.lastPtr.Y)) > 0.5
			if moved {
				st.lastPtr = ptr
				st.hasLastPtr = true
				st.ptrMovedAt = now
			}
		}

		// Determine which parent item currently holds the pointer using previous frame bounds.
		hoveredIdx := -1
		if overWin && len(st.itemRects) == len(items) {
			for i, r := range st.itemRects {
				if rectContainsPoint(r, ptr) {
					hoveredIdx = i
					break
				}
			}
		}

		// Update open/close timers and safe-triangle logic.
		if st.openSubIdx >= 0 && st.openSubIdx < len(items) {
			var openItemRect ui.Rect
			if st.openSubIdx < len(st.itemRects) {
				openItemRect = st.itemRects[st.openSubIdx]
			}
			overOpenItem := overWin && rectContainsPoint(openItemRect, ptr)
			overSub := overWin && rectContainsPoint(st.subRect, ptr)
			overParent := overWin && rectContainsPoint(st.parentRect, ptr)

			switch {
			case overOpenItem:
				st.triAnchor = ptr
				st.hasTriAnchor = true
				st.activeIdx = st.openSubIdx
				st.pendingCloseAt = time.Time{}
				st.pendingOpenIdx = -1
				st.pendingOpenAt = time.Time{}
			case overSub:
				st.pendingCloseAt = time.Time{}
				st.pendingOpenIdx = -1
				st.pendingOpenAt = time.Time{}
			case overParent && hoveredIdx >= 0 && hoveredIdx != st.openSubIdx && !items[hoveredIdx].Disabled && !items[hoveredIdx].Separator:
				inTri := st.hasTriAnchor && pointInSafeTriangle(ptr, st.triAnchor, st.subRect, safeTrianglePadding)
				movingToward := !moved || !hadPrev || towardSubmenuEdge(prevPtr, ptr, st.subRect)
				dwell := now.Sub(st.ptrMovedAt)
				stoppedTooLong := !st.ptrMovedAt.IsZero() && dwell >= cascadingOpenDelay
				if inTri && movingToward && !stoppedTooLong {
					st.pendingCloseAt = time.Time{}
					remaining := cascadingOpenDelay - dwell
					if remaining > 0 {
						c.After(remaining)
					}
				} else {
					st.activeIdx = hoveredIdx
					st.openSubIdx = -1
					st.subActiveIdx = -1
					st.focusLevel = 0
					st.pendingCloseAt = time.Time{}
					st.hasTriAnchor = false
					if len(items[hoveredIdx].Submenu) > 0 {
						st.pendingOpenIdx = hoveredIdx
						st.pendingOpenAt = now.Add(cascadingOpenDelay)
						c.After(cascadingOpenDelay)
					} else {
						st.pendingOpenIdx = -1
						st.pendingOpenAt = time.Time{}
					}
				}
			default:
				inGapTri := overWin && st.hasTriAnchor && pointInSafeTriangle(ptr, st.triAnchor, st.subRect, safeTrianglePadding) && (!moved || !hadPrev || towardSubmenuEdge(prevPtr, ptr, st.subRect))
				if inGapTri && (st.ptrMovedAt.IsZero() || now.Sub(st.ptrMovedAt) < cascadingCloseDelay) {
					st.pendingCloseAt = time.Time{}
					c.After(cascadingCloseDelay)
				} else {
					if st.pendingCloseAt.IsZero() {
						st.pendingCloseAt = now.Add(cascadingCloseDelay)
					}
					if !now.Before(st.pendingCloseAt) {
						st.openSubIdx = -1
						st.subActiveIdx = -1
						st.focusLevel = 0
						st.pendingCloseAt = time.Time{}
						st.hasTriAnchor = false
					} else {
						c.After(st.pendingCloseAt.Sub(now))
					}
				}
			}
		} else if overWin && hoveredIdx >= 0 && !items[hoveredIdx].Disabled && !items[hoveredIdx].Separator {
			st.activeIdx = hoveredIdx
			if len(items[hoveredIdx].Submenu) > 0 {
				if st.pendingOpenIdx != hoveredIdx {
					st.pendingOpenIdx = hoveredIdx
					st.pendingOpenAt = now.Add(cascadingOpenDelay)
					c.After(cascadingOpenDelay)
				} else if !now.Before(st.pendingOpenAt) {
					st.openSubIdx = hoveredIdx
					st.pendingOpenIdx = -1
					st.pendingOpenAt = time.Time{}
					st.pendingCloseAt = time.Time{}
					st.triAnchor = ptr
					st.hasTriAnchor = true
				} else {
					c.After(st.pendingOpenAt.Sub(now))
				}
			} else {
				st.pendingOpenIdx = -1
				st.pendingOpenAt = time.Time{}
			}
		} else {
			st.pendingOpenIdx = -1
			st.pendingOpenAt = time.Time{}
		}

		// Build parent menu panel A.
		panelA := ui.Column(c.Key(key+"-panel")).
			AttachTo(anchor, ui.AnchorBottomEnd, ui.AnchorTopEnd).
			Margin(4, 0, 0, 0).
			Width(menuWidth).
			Padding(12).
			Gap(4).
			Radius(radiusPanel).
			Background(pal.surface).
			Border(1, pal.outline).
			Transition(popupTransition)

		if len(st.itemRects) != len(items) {
			st.itemRects = make([]ui.Rect, len(items))
		}
		var openSubItemEl ui.Element
		panelA.Children(func() {
			for i := range items {
				idx := i
				it := items[idx]
				if it.Separator {
					ui.Divider(c.Key(key+"-sep-"+it.Key)).Background(pal.line).Margin(4, 0)
					st.itemRects[idx] = ui.Rect{}
					continue
				}
				label := it.AccessLabel
				if label == "" {
					label = it.Label
				}
				highlighted := st.activeIdx == idx || st.openSubIdx == idx
				row := ui.ButtonBase(c.Key(key+"-item-"+it.Key)).
					Label(label).
					FillWidth().
					Padding(8, 12).
					Gap(8).
					AlignItems(ui.Center).
					Radius(radiusSmall).
					Cursor(ui.CursorPointer).
					Disabled(it.Disabled)
				tactile(row)
				if highlighted || row.Hovered() {
					row.Background(pal.chrome)
				}
				textColor := pal.ink
				if it.Danger {
					textColor = pal.errorInk
				}
				if it.Disabled {
					textColor = pal.muted
				}
				row.Children(func() {
					ui.Text(c, it.Label).FontSize(textSize(c, textControl)).FontWeight(500).TextColor(textColor).LineHeight(1.5).Grow(1).MinWidth(0)
					if len(it.Submenu) > 0 {
						ui.Box(c).TextColor(pal.muted).Children(func() { icon(c, "next") })
					}
				})
				if row.Clicked() {
					if len(it.Submenu) > 0 {
						st.activeIdx = idx
						st.openSubIdx = idx
						st.pendingOpenIdx = -1
						st.pendingOpenAt = time.Time{}
						st.pendingCloseAt = time.Time{}
						if overWin {
							st.triAnchor = ptr
							st.hasTriAnchor = true
						}
					} else {
						action := it.Action
						*open = false
						if action != nil {
							action()
						}
					}
				}
				st.itemRects[idx] = row.Bounds()
				if st.openSubIdx == idx {
					openSubItemEl = row
				}
			}
		})
		st.parentRect = panelA.Bounds()

		// Build submenu panel B when open.
		var panelB ui.Element
		if st.openSubIdx >= 0 && st.openSubIdx < len(items) && openSubItemEl.Valid() {
			subItems := items[st.openSubIdx].Submenu
			panelB = ui.Column(c.Key(key+"-submenu-"+items[st.openSubIdx].Key)).
				AttachTo(openSubItemEl, ui.AnchorTopEnd, ui.AnchorTopStart).
				Margin(0, 0, 0, 4).
				Width(menuWidth).
				Padding(12).
				Gap(4).
				Radius(radiusPanel).
				Background(pal.surface).
				Border(1, pal.outline).
				Transition(popupTransition)
			panelB.Children(func() {
				for j := range subItems {
					subIdx := j
					sub := subItems[subIdx]
					if sub.Separator {
						ui.Divider(c.Key(key+"-subsep-"+sub.Key)).Background(pal.line).Margin(4, 0)
						continue
					}
					label := sub.AccessLabel
					if label == "" {
						label = sub.Label
					}
					subRow := ui.ButtonBase(c.Key(key+"-subitem-"+sub.Key)).
						Label(label).
						FillWidth().
						Padding(8, 12).
						Gap(8).
						AlignItems(ui.Center).
						Radius(radiusSmall).
						Cursor(ui.CursorPointer).
						Disabled(sub.Disabled)
					tactile(subRow)
					if subRow.Hovered() {
						st.subActiveIdx = subIdx
						st.focusLevel = 1
					}
					if (st.focusLevel == 1 && st.subActiveIdx == subIdx) || subRow.Hovered() {
						subRow.Background(pal.chrome)
					}
					textColor := pal.ink
					if sub.Danger {
						textColor = pal.errorInk
					}
					if sub.Disabled {
						textColor = pal.muted
					}
					subRow.Children(func() {
						ui.Text(c, sub.Label).FontSize(textSize(c, textControl)).FontWeight(500).TextColor(textColor).LineHeight(1.5).Grow(1).MinWidth(0)
					})
					if subRow.Clicked() {
						action := sub.Action
						*open = false
						if action != nil {
							action()
						}
					}
				}
			})
			st.subRect = panelB.Bounds()
		} else {
			st.subRect = ui.Rect{}
		}

		// Close immediately on a press outside both parent menu A and submenu B.
		if panelA.PressedOutside() && (!panelB.Valid() || panelB.PressedOutside()) {
			*open = false
		}
	})
}
