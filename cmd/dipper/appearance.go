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
	title, light, dark, system, detail, saveError string
}{
	title: "界面主题", light: "浅色", dark: "夜间", system: "跟随系统",
	detail:    "切换后立即生效，下次打开时保留。跟随系统会自动切换浅色与夜间模式。",
	saveError: "外观设置保存失败：",
}

type colorPalette struct {
	accent, accentHover, accentPressed                 ui.Color
	blue, blueHover, bluePressed, blueWash, accentText ui.Color
	muted, subtle, ink, line, outline, hoverLine       ui.Color
	chrome, paper, paperHover, canvas, brandWash       ui.Color
	segThumb, topHighlight                             ui.Color
	green, greenWash, amber, amberWash                 ui.Color
	errorInk, errorWash                                ui.Color
	glassBg, glassBorder, glassInk                     ui.Color
	scrim, popoverShadow, surfaceShadow                ui.Color
	cardShadow, cardShadowHover                        ui.Color
	sunken                                             ui.Color
}

var lightPalette = colorPalette{
	// Monochrome Chinese ink palette: warm Xuan paper neutrals with Wu-Mo (ink black) accent.
	accent: ui.Hex("#23201c"), accentHover: ui.Hex("#38332d"), accentPressed: ui.Hex("#141210"),
	blue: ui.Hex("#23201c"), blueHover: ui.Hex("#38332d"), bluePressed: ui.Hex("#141210"), blueWash: ui.Hex("#e5e1d8"), accentText: ui.Hex("#fcfbf8"),
	muted: ui.Hex("#615b52"), subtle: ui.Hex("#7a7368"), ink: ui.Hex("#23201c"),
	line: ui.RGBA(35, 32, 28, .08), outline: ui.RGBA(35, 32, 28, .16), hoverLine: ui.RGBA(35, 32, 28, .28),
	chrome: ui.Hex("#efece5"), paper: ui.Hex("#fcfbf8"), paperHover: ui.Hex("#f3f0e9"), canvas: ui.Hex("#f7f5f0"), brandWash: ui.Hex("#efece5"),
	segThumb: ui.Hex("#fcfbf8"), topHighlight: ui.RGBA(255, 252, 245, .85),
	green: ui.Hex("#23201c"), greenWash: ui.Hex("#e5e1d8"), amber: ui.Hex("#6e5a20"), amberWash: ui.Hex("#f4efe0"),
	errorInk: ui.Hex("#8e2936"), errorWash: ui.Hex("#f5e8ea"),
	glassBg: ui.RGBA(24, 22, 19, .64), glassBorder: ui.RGBA(255, 250, 240, .24), glassInk: ui.Hex("#fcfbf8"),
	scrim: ui.RGBA(24, 22, 19, .34), popoverShadow: ui.RGBA(35, 32, 28, .14), surfaceShadow: ui.RGBA(35, 32, 28, .05),
	cardShadow: ui.RGBA(35, 32, 28, .10), cardShadowHover: ui.RGBA(35, 32, 28, .20),
	sunken: ui.Hex("#e7e3da"),
}

var darkPalette = colorPalette{
	// Monochrome Chinese ink palette: warm pine-soot neutrals with Ya-Bai (warm ivory) accent.
	accent: ui.Hex("#efece6"), accentHover: ui.Hex("#faf8f4"), accentPressed: ui.Hex("#dcd6cc"),
	blue: ui.Hex("#f2efe9"), blueHover: ui.Hex("#ffffff"), bluePressed: ui.Hex("#ddd7cd"), blueWash: ui.Hex("#2c2925"), accentText: ui.Hex("#161513"),
	muted: ui.Hex("#beb7ac"), subtle: ui.Hex("#8d867b"), ink: ui.Hex("#f2efe9"),
	line: ui.RGBA(248, 242, 232, .07), outline: ui.RGBA(248, 242, 232, .13), hoverLine: ui.RGBA(248, 242, 232, .26),
	chrome: ui.Hex("#1a1816"), paper: ui.Hex("#22201d"), paperHover: ui.Hex("#2b2824"), canvas: ui.Hex("#141311"), brandWash: ui.Hex("#efece5"),
	segThumb: ui.Hex("#2e2b27"), topHighlight: ui.RGBA(255, 248, 236, .07),
	green: ui.Hex("#f2efe9"), greenWash: ui.Hex("#2c2925"), amber: ui.Hex("#d8c382"), amberWash: ui.Hex("#2e291c"),
	errorInk: ui.Hex("#eba3ad"), errorWash: ui.Hex("#321f23"),
	glassBg: ui.RGBA(20, 19, 17, .62), glassBorder: ui.RGBA(255, 248, 236, .16), glassInk: ui.Hex("#f5f2ec"),
	scrim: ui.RGBA(10, 9, 8, .64), popoverShadow: ui.RGBA(0, 0, 0, .54), surfaceShadow: ui.RGBA(0, 0, 0, .24),
	cardShadow: ui.RGBA(0, 0, 0, .44), cardShadowHover: ui.RGBA(0, 0, 0, .64),
	sunken: ui.Hex("#0f0e0c"),
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
	t.Surface = p.paper
	t.SurfaceHover = p.paperHover
	t.SurfacePressed = p.line
	t.Text = p.ink
	t.TextMuted = p.muted
	t.Border = p.outline
	t.Accent = p.accent
	t.AccentHover = p.accentHover
	t.AccentPressed = p.accentPressed
	t.Focus = p.blue
	t.AccentText = p.accentText
	t.Success = p.green
	t.Warning = p.amber
	t.Danger = p.errorInk
	t.Selection = p.accent.Alpha(.28)
	if dark {
		t.Inverse = p.paperHover
		t.InverseText = p.ink
	} else {
		t.Inverse = ui.Hex("#23201c")
		t.InverseText = ui.Hex("#f7f5f0")
	}
	t.Font = "Segoe UI, Microsoft YaHei UI"
	t.FontSize = float32(textBody)
	t.Radius = controlRadius
	t.Spacing = 4
	return t
}

func surfacePanel(c *ui.Context, e ui.Element) ui.Element {
	pal := colors(c)
	e.Background(pal.paper).Border(1, pal.line)
	if c.Theme().Dark {
		e.InsetShadow(0, 1, 0, 0, pal.topHighlight)
	} else {
		e.Shadow(0, 1, 3, 0, pal.surfaceShadow)
	}
	return e
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
	ui.Text(c, appearanceText.title).FontSize(textSize(c, textBody)).FontWeight(600)
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
				Column().Gap(8).Padding(10).Radius(10).
				MinWidth(max(132, textSize(c, textBody)*8)).
				Background(pal.canvas).Border(1, pal.outline).
				Cursor(ui.CursorPointer).Disabled(s.Busy)
			if active {
				card.Background(pal.blueWash).Border(2, pal.accent)
			} else if card.Hovered() {
				card.Background(pal.paperHover).BorderColor(pal.hoverLine)
			}
			tactile(card).Children(func() {
				appearanceSwatch(c, ch.value)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					label := ui.Text(c, ch.label).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink)
					if active {
						label.FontWeight(600).TextColor(pal.blue)
					}
				})
			}).OnClick(func() { s.setAppearance(ch.value) })
		}
	})
	caption(c, appearanceText.detail)
}

func appearanceSwatch(c *ui.Context, mode string) {
	pal := colors(c)
	previewHalf := func(p *colorPalette) {
		ui.Row(c).Grow(1).FillHeight().Background(p.canvas).Children(func() {
			ui.Column(c).Width(24).FillHeight().Background(p.chrome).Padding(6, 4).Gap(4).Children(func() {
				ui.Box(c).Height(5).Radius(2).Background(p.accent)
				ui.Box(c).Height(4).Radius(2).Background(p.outline)
			})
			ui.Column(c).Grow(1).FillHeight().Padding(7).Gap(5).Children(func() {
				ui.Box(c).Width(36).Height(5).Radius(2).Background(p.outline)
				ui.Row(c).Grow(1).Gap(5).Children(func() {
					ui.Box(c).Grow(1).FillHeight().Radius(4).Background(p.paper).Border(1, p.line)
					ui.Box(c).Grow(1).FillHeight().Radius(4).Background(p.accent)
				})
			})
		})
	}
	ui.Row(c).FillWidth().Height(54).Radius(7).Clip().Border(1, pal.outline).Children(func() {
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
	navIndicatorMatch   struct{}
	seasonThumbMatch    struct{}
	dayUnderlineMatch   struct{}
	downloadFilterMatch struct{}
	posterHeroMatch     struct{ animeID string }
)

func tactile(e ui.Element) ui.Element {
	e.Transition(hoverTransition)
	pose := ui.Pose{}
	if e.Pressed() {
		pose.Scale = .975
	}
	return e.AnimatePose(pose, tactileMotion)
}
