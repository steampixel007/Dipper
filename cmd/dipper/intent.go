package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	intentRevealDuration   = 130 * time.Millisecond
	intentScrollRevealHold = 800 * time.Millisecond
	intentEdgeBand         = float32(120)
)

type intentEdgeKey struct{}

type intentEdgeState struct {
	lastY         float32
	hasY          bool
	scrolledUntil time.Time
}

// intentReveal returns the opacity (0..1) for secondary controls belonging to
// region. Secondary controls remain hidden at rest and appear over 130 ms when
// the pointer enters the region, keyboard focus is inside it, scrolling moves
// it near the viewport edge, or the user is actively working in it.
func intentReveal(c *ui.Context, region ui.Element, working bool) float32 {
	return intentRevealIn(c, region, ui.Rect{}, working)
}

func intentRevealIn(c *ui.Context, region ui.Element, viewport ui.Rect, working bool) float32 {
	state := ui.Local(region, intentEdgeKey{}, func() intentEdgeState { return intentEdgeState{} })
	now := c.Now()
	_, winH := c.Size()
	topEdge, bottomEdge := topbarHeight, winH
	if viewport.H > 0 {
		topEdge, bottomEdge = viewport.Y, viewport.Y+viewport.H
	}
	if bounds := region.Bounds(); bounds.H > 0 {
		if state.hasY && math.Abs(float64(bounds.Y-state.lastY)) > 1 {
			nearTop := bounds.Y+bounds.H >= topEdge && bounds.Y <= topEdge+intentEdgeBand
			nearBottom := bounds.Y <= bottomEdge && bounds.Y+bounds.H >= bottomEdge-intentEdgeBand
			if nearTop || nearBottom {
				state.scrolledUntil = now.Add(intentScrollRevealHold)
				c.After(intentScrollRevealHold)
			}
		}
		state.lastY = bounds.Y
		state.hasY = true
	}
	scrollEdge := !state.scrolledUntil.IsZero() && now.Before(state.scrolledUntil)
	if scrollEdge {
		c.After(state.scrolledUntil.Sub(now))
	}
	revealed := region.Hovered() || region.FocusWithin() || working || scrollEdge
	target := float32(0)
	if revealed {
		target = 1
	}
	return region.AnimateWith("intent-reveal", target, intentRevealDuration, spectrumEase)
}

// intentGroup wraps secondary controls so their layout space stays reserved at
// rest while opacity transitions smoothly on user intent. Controls remain
// reachable by Tab so keyboard focus reveals the group.
func intentGroup(c *ui.Context, region ui.Element, working bool, build func()) ui.Element {
	return intentGroupIn(c, region, ui.Rect{}, working, build)
}

func intentGroupIn(c *ui.Context, region ui.Element, viewport ui.Rect, working bool, build func()) ui.Element {
	return ui.Row(c).AlignItems(ui.Center).Gap(controlGap).Shrink(0).Opacity(intentRevealIn(c, region, viewport, working)).Children(build)
}
