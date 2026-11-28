package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	appearanceLight  = "light"
	appearanceDark   = "dark"
	appearanceSystem = "system"
)

var appearanceText = struct {
	title, light, dark, system, saveError string
}{
	title: "界面主题", light: "浅色", dark: "夜间", system: "跟随系统",
	saveError: "外观设置保存失败：",
}

type colorPalette struct {
	canvas, chrome, dialogSurface, surface, surfaceHover, sunken ui.Color
	ink, muted, subtle                                           ui.Color
	accent, accentHover                                          ui.Color
	accentPressed, accentWash                                    ui.Color
	accentInk, accentText                                        ui.Color
	warningInk, warningWash                                      ui.Color
	errorInk, errorWash                                          ui.Color
	line, outline, hoverLine                                     ui.Color
	segThumb, topHighlight                                       ui.Color
	glassBg, glassBorder, glassInk                               ui.Color
	scrim, brandWash                                             ui.Color
	popoverShadow, surfaceShadow                                 ui.Color
	cardShadow, cardShadowHover                                  ui.Color
}

var lightPalette = colorPalette{
	// Monochrome Chinese ink palette: warm Xuan paper neutrals with Wu-Mo (ink black) accent.
	canvas:          ui.Hex("#f7f5f0"),
	chrome:          ui.Hex("#efece5"),
	dialogSurface:   ui.Hex("#f7f5f0"),
	surface:         ui.Hex("#faf8f4"),
	surfaceHover:    ui.Hex("#f3f0e9"),
	sunken:          ui.Hex("#e7e3da"),
	ink:             ui.Hex("#23201c"),
	muted:           ui.Hex("#615b52"),
	subtle:          ui.Hex("#7a7368"),
	accent:          ui.Hex("#23201c"),
	accentHover:     ui.Hex("#38332d"),
	accentPressed:   ui.Hex("#141210"),
	accentWash:      ui.Hex("#e5e1d8"),
	accentInk:       ui.Hex("#23201c"),
	accentText:      ui.Hex("#fcfbf8"),
	warningInk:      ui.Hex("#6e5a20"),
	warningWash:     ui.Hex("#f4efe0"),
	errorInk:        ui.Hex("#8e2936"),
	errorWash:       ui.Hex("#f5e8ea"),
	line:            ui.RGBA(35, 32, 28, .08),
	outline:         ui.RGBA(35, 32, 28, .16),
	hoverLine:       ui.RGBA(35, 32, 28, .28),
	segThumb:        ui.Hex("#fcfbf8"),
	topHighlight:    ui.RGBA(255, 252, 245, .85),
	glassBg:         ui.RGBA(24, 22, 19, .64),
	glassBorder:     ui.RGBA(255, 250, 240, .24),
	glassInk:        ui.Hex("#fcfbf8"),
	scrim:           ui.RGBA(24, 22, 19, .34),
	brandWash:       ui.Hex("#efece5"),
	popoverShadow:   ui.RGBA(35, 32, 28, .14),
	surfaceShadow:   ui.RGBA(35, 32, 28, .05),
	cardShadow:      ui.RGBA(35, 32, 28, .10),
	cardShadowHover: ui.RGBA(35, 32, 28, .20),
}

var darkPalette = colorPalette{
	// Monochrome Chinese ink palette: warm pine-soot neutrals with Ya-Bai (warm ivory) accent.
	canvas:          ui.Hex("#141311"),
	chrome:          ui.Hex("#1a1816"),
	dialogSurface:   ui.Hex("#1c1a17"),
	surface:         ui.Hex("#22201d"),
	surfaceHover:    ui.Hex("#2b2824"),
	sunken:          ui.Hex("#0f0e0c"),
	ink:             ui.Hex("#f2efe9"),
	muted:           ui.Hex("#beb7ac"),
	subtle:          ui.Hex("#8d867b"),
	accent:          ui.Hex("#efece6"),
	accentHover:     ui.Hex("#faf8f4"),
	accentPressed:   ui.Hex("#dcd6cc"),
	accentWash:      ui.Hex("#2c2925"),
	accentInk:       ui.Hex("#f2efe9"),
	accentText:      ui.Hex("#161513"),
	warningInk:      ui.Hex("#d8c382"),
	warningWash:     ui.Hex("#2e291c"),
	errorInk:        ui.Hex("#eba3ad"),
	errorWash:       ui.Hex("#321f23"),
	line:            ui.RGBA(248, 242, 232, .07),
	outline:         ui.RGBA(248, 242, 232, .13),
	hoverLine:       ui.RGBA(248, 242, 232, .26),
	segThumb:        ui.Hex("#2e2b27"),
	topHighlight:    ui.RGBA(255, 248, 236, .07),
	glassBg:         ui.RGBA(20, 19, 17, .62),
	glassBorder:     ui.RGBA(255, 248, 236, .16),
	glassInk:        ui.Hex("#f5f2ec"),
	scrim:           ui.RGBA(10, 9, 8, .64),
	brandWash:       ui.Hex("#efece5"),
	popoverShadow:   ui.RGBA(0, 0, 0, .54),
	surfaceShadow:   ui.RGBA(0, 0, 0, .24),
	cardShadow:      ui.RGBA(0, 0, 0, .44),
	cardShadowHover: ui.RGBA(0, 0, 0, .64),
}

func colors(c *ui.Context) *colorPalette {
	if c.Theme().Dark {
		return &darkPalette
	}
	return &lightPalette
}

func windowBackground() string {
	light, dark := lightPalette.canvas, darkPalette.canvas
	return fmt.Sprintf("light-dark(#%02x%02x%02x, #%02x%02x%02x)", light.R, light.G, light.B, dark.R, dark.G, dark.B)
}

func normalizeAppearance(value string) string {
	switch value {
	case appearanceDark, appearanceSystem:
		return value
	default:
		return appearanceLight
	}
}

func (s *State) darkAppearance(systemDark bool) bool {
	switch normalizeAppearance(s.Appearance) {
	case appearanceDark:
		return true
	case appearanceSystem:
		return systemDark
	default:
		return false
	}
}

func spectrumTheme(dark bool) *ui.Theme {
	p, t := lightPalette, ui.LightTheme()
	if dark {
		p, t = darkPalette, ui.DarkTheme()
	}
	t.Background = p.canvas
	t.Surface = p.surface
	t.SurfaceHover = p.surfaceHover
	t.SurfacePressed = p.sunken
	t.Text = p.ink
	t.TextMuted = p.muted
	t.Border = p.outline
	t.Accent = p.accent
	t.AccentHover = p.accentHover
	t.AccentPressed = p.accentPressed
	t.Focus = p.accent
	t.AccentText = p.accentText
	t.Success = p.ink
	t.Warning = p.warningInk
	t.Danger = p.errorInk
	t.Selection = p.accent.Alpha(.28)
	if dark {
		t.Inverse = p.surfaceHover
		t.InverseText = p.ink
	} else {
		t.Inverse = ui.Hex("#23201c")
		t.InverseText = ui.Hex("#f7f5f0")
	}
	t.Font = uiFontFamily
	t.FontSize = float32(textControl)
	t.Radius = radiusSmall
	t.Spacing = 4
	return t
}

func (s *State) setAppearance(value string) {
	if s.Busy {
		return
	}
	previous := s.Appearance
	s.Appearance = normalizeAppearance(value)
	if err := s.savePreferences(); err != nil {
		s.Appearance = previous
		s.SaveError = appearanceText.saveError + err.Error()
		return
	}
	s.SaveError = ""
	if s.ApplyAppearance != nil {
		s.ApplyAppearance(s.Appearance)
	}
}

func (s *State) appearanceSettings(c *ui.Context) {
	pal := colors(c)
	ui.Text(c, appearanceText.title).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5)
	selected := normalizeAppearance(s.Appearance)
	ui.Row(c).Wrap().Gap(12).Children(func() {
		for _, choice := range []struct{ value, label string }{
			{appearanceLight, appearanceText.light},
			{appearanceDark, appearanceText.dark},
			{appearanceSystem, appearanceText.system},
		} {
			ch := choice
			active := selected == ch.value
			card := ui.ButtonBase(c.Key("appearance-"+ch.value)).
				Label(ch.label).
				Column().Gap(8).Padding(12).Radius(radiusControl).
				MinWidth(max(132, textSize(c, textBody)*8)).
				Background(pal.canvas).Border(1, pal.outline).
				Cursor(ui.CursorPointer).Disabled(s.Busy)
			if active {
				card.Background(pal.accentWash).Border(2, pal.accent)
			} else if card.Hovered() {
				card.Background(pal.surfaceHover).BorderColor(pal.hoverLine)
			}
			tactile(card).Children(func() {
				appearanceSwatch(c, ch.value)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					label := ui.Text(c, ch.label).FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.ink)
					if active {
						label.FontWeight(600).TextColor(pal.accentInk)
					}
				})
			}).OnClick(func() { s.setAppearance(ch.value) })
		}
	})
}

func appearanceSwatch(c *ui.Context, mode string) {
	pal := colors(c)
	previewHalf := func(p *colorPalette) {
		ui.Row(c).Grow(1).FillHeight().Background(p.canvas).Children(func() {
			ui.Column(c).Width(24).FillHeight().Background(p.chrome).Padding(6, 4).Gap(4).Children(func() {
				ui.Box(c).Height(5).Radius(2).Background(p.accentWash).Border(1, p.outline)
				ui.Box(c).Height(4).Radius(2).Background(p.line)
				ui.Box(c).Height(4).Radius(2).Background(p.line)
			})
			ui.Column(c).Grow(1).FillHeight().Padding(6).Gap(5).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
					ui.Box(c).Width(28).Height(4).Radius(2).Background(p.ink).Opacity(0.55)
					ui.Spacer(c)
					ui.Box(c).Width(14).Height(5).Radius(2).Background(p.accent)
				})
				ui.Row(c).Grow(1).Gap(4).Children(func() {
					ui.Box(c).Grow(1).FillHeight().Radius(4).Background(p.surface).Border(1, p.outline)
					ui.Box(c).Grow(1).FillHeight().Radius(4).Background(p.sunken).Border(1, p.line)
					ui.Box(c).Grow(1).FillHeight().Radius(4).Background(p.surface).Border(1, p.outline)
				})
			})
		})
	}
	ui.Row(c).FillWidth().Height(54).Radius(radiusSmall).Clip().Border(1, pal.outline).Children(func() {
		switch mode {
		case appearanceLight:
			previewHalf(&lightPalette)
		case appearanceDark:
			previewHalf(&darkPalette)
		default:
			previewHalf(&lightPalette)
			previewHalf(&darkPalette)
		}
	})
}

// Motion tokens follow MyGo v0.9.0's velocity-preserving springs and Oklab color
// interpolation while honoring ReduceMotion / ReduceEffects.
var (
	spectrumEase  = ui.CubicBezier(0, 0, .4, 1)
	tactileMotion = ui.Animation{
		Spring: ui.SpringOptions{Duration: 220 * time.Millisecond, Bounce: 0},
		Oklab:  true,
	}
	indicatorMotion = ui.Animation{
		Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.08},
		Oklab:  true,
	}
	panelMotion = ui.Animation{
		Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.04},
		Oklab:  true,
	}
	contentMotion = ui.Animation{
		Duration:  180 * time.Millisecond,
		Ease:      ui.EaseOut,
		Oklab:     true,
		NonMotion: true,
	}
	hoverTransition = ui.ElementTransition{
		Colors:    true,
		Shadows:   true,
		Animation: &tactileMotion,
	}
	revealTransition = ui.ElementTransition{
		Duration: 130 * time.Millisecond,
		Ease:     spectrumEase,
		Enter:    &ui.Motion{},
		Exit:     &ui.Motion{},
	}
	pageTransition = ui.ElementTransition{
		Duration:     200 * time.Millisecond,
		ExitDuration: 130 * time.Millisecond,
		Ease:         ui.EaseOut,
		ExitEase:     ui.EaseIn,
		Enter:        &ui.Motion{Y: 6},
		Exit:         &ui.Motion{},
	}
	popupTransition = ui.ElementTransition{
		Animation:    &panelMotion,
		ExitDuration: 120 * time.Millisecond,
		ExitEase:     ui.EaseIn,
		Enter:        &ui.Motion{Y: 4, Scale: 0.97},
		Exit:         &ui.Motion{Y: 2, Scale: 0.98},
	}
	dialogTransition = ui.ElementTransition{
		Animation:    &panelMotion,
		ExitDuration: 130 * time.Millisecond,
		ExitEase:     ui.EaseIn,
		Enter:        &ui.Motion{Y: 8, Scale: 0.96},
		Exit:         &ui.Motion{Y: 4, Scale: 0.98},
	}
	fadeTransition = ui.ElementTransition{
		Duration: 180 * time.Millisecond,
		Ease:     ui.EaseOut,
		Colors:   true,
		Enter:    &ui.Motion{},
		Exit:     &ui.Motion{},
	}
	rowTransition = ui.ElementTransition{
		Animation: &panelMotion,
		Position:  true,
		Colors:    true,
		Enter:     &ui.Motion{Y: 4},
		Exit:      &ui.Motion{},
	}
	posterOverlayTransition = ui.ElementTransition{
		Animation:    &tactileMotion,
		ExitDuration: 120 * time.Millisecond,
		ExitEase:     ui.EaseIn,
		Enter:        &ui.Motion{Y: 6},
		Exit:         &ui.Motion{Y: 4},
	}
)

type (
	sidebarNavMatch      struct{}
	seasonThumbMatch     struct{}
	dayUnderlineMatch    struct{}
	downloadFilterMatch  struct{}
	versionLangMatch     struct{}
	versionPriorityMatch struct{}
	proxyModeMatch       struct{}
	posterHeroMatch      struct{ animeID string }
)

func tactile(e ui.Element) ui.Element {
	e.Transition(hoverTransition)
	pose := ui.Pose{}
	if e.Pressed() {
		pose.Scale = .975
	}
	return e.AnimatePose(pose, tactileMotion)
}

func glidingSegmented(c *ui.Context, key, label string, selected int, labels []string, match any, disabled bool, choose func(int)) {
	pal := colors(c)
	activeIndex := selected
	parts := ui.SegmentedBase(c.Key(key), &selected, len(labels))
	parts.Track.Label(label).Wrap().Gap(4).Padding(4).Radius(radiusControl).Background(pal.sunken).Border(1, pal.line).Disabled(disabled).Children(func() {
		for index, text := range labels {
			idx := index
			active := activeIndex == idx
			seg := parts.Segment(idx).Label(text).Padding(compactPaddingY, 12).Radius(radiusSmall).Center().Shrink(0).
				FontSize(textSize(c, textControl)).FontWeight(500).Cursor(ui.CursorPointer).TextColor(pal.muted).Disabled(disabled)
			if active {
				seg.TextColor(pal.ink).FontWeight(600)
			} else if seg.Hovered() {
				seg.TextColor(pal.ink)
			}
			tactile(seg).Children(func() {
				if active {
					thumb := ui.Box(c.Key(fmt.Sprintf("%s-thumb-%d", key, idx))).Absolute().Top(0).Bottom(0).Left(0).Right(0).
						ZIndex(-1).Radius(radiusSmall).Background(pal.segThumb).
						Shadow(0, 1, 4, 0, pal.cardShadow).IgnoreInput(true).AccessHidden(true).
						Transition(ui.ElementTransition{Animation: &indicatorMotion, Match: match})
					if c.Theme().Dark {
						thumb.InsetShadow(0, 1, 0, 0, pal.topHighlight)
					}
				}
				ui.Text(c, text).SingleLine()
			})
			if active {
				seg.OnClick(func() {
					if !disabled {
						choose(idx)
					}
				})
			}
		}
	}).OnChange(func() {
		if !disabled {
			choose(selected)
		}
	})
}
