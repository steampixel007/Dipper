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
	blue, blueHover, bluePressed, blueWash, accentText ui.Color
	muted, ink, line, outline, hoverLine               ui.Color
	chrome, paper, canvas, brandWash                   ui.Color
	green, greenWash, amber, amberWash                 ui.Color
	avoidInk, avoidWash, errorInk, errorWash           ui.Color
	scrim, popoverShadow                               ui.Color
}

var lightPalette = colorPalette{
	blue: ui.Hex("#3b63fb"), blueHover: ui.Hex("#274dea"), bluePressed: ui.Hex("#1d3ecf"), blueWash: ui.Hex("#f5f9ff"), accentText: ui.Hex("#ffffff"),
	muted: ui.Hex("#505050"), ink: ui.Hex("#292929"), line: ui.Hex("#e9e9e9"), outline: ui.Hex("#c6c6c6"), hoverLine: ui.Hex("#b1b1b1"),
	chrome: ui.Hex("#f3f3f3"), paper: ui.Hex("#ffffff"), canvas: ui.Hex("#f8f8f8"), brandWash: ui.Hex("#f5f9ff"),
	green: ui.Hex("#12805c"), greenWash: ui.Hex("#e9f7ef"), amber: ui.Hex("#9c5700"), amberWash: ui.Hex("#fff4db"),
	avoidInk: ui.Hex("#b1262b"), avoidWash: ui.Hex("#fff5f5"), errorInk: ui.Hex("#c9252d"), errorWash: ui.Hex("#fff0f0"),
	scrim: ui.RGBA(0, 0, 0, .24), popoverShadow: ui.RGBA(0, 0, 0, .10),
}

var darkPalette = colorPalette{
	blue: ui.Hex("#9bb3ff"), blueHover: ui.Hex("#b2c4ff"), bluePressed: ui.Hex("#87a3f5"), blueWash: ui.Hex("#283952"), accentText: ui.Hex("#14213a"),
	muted: ui.Hex("#b3c0d1"), ink: ui.Hex("#e7edf5"), line: ui.Hex("#334353"), outline: ui.Hex("#718298"), hoverLine: ui.Hex("#91a2b7"),
	chrome: ui.Hex("#18212b"), paper: ui.Hex("#202b37"), canvas: ui.Hex("#1b2530"), brandWash: ui.Hex("#f5f9ff"),
	green: ui.Hex("#83d9b5"), greenWash: ui.Hex("#233e37"), amber: ui.Hex("#f0c27b"), amberWash: ui.Hex("#403625"),
	avoidInk: ui.Hex("#f5a3a8"), avoidWash: ui.Hex("#422d38"), errorInk: ui.Hex("#ffafb4"), errorWash: ui.Hex("#452d38"),
	scrim: ui.RGBA(0, 0, 0, .56), popoverShadow: ui.RGBA(0, 0, 0, .32),
}

func colors(c *ui.Context) *colorPalette {
	if c.Theme().Dark {
		return &darkPalette
	}
	return &lightPalette
}

func windowBackground() string {
	light, dark := lightPalette.chrome, darkPalette.chrome
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
	t.SurfaceHover = p.chrome
	t.SurfacePressed = p.line
	t.Text = p.ink
	t.TextMuted = p.muted
	t.Border = p.outline
	t.Accent = p.blue
	t.AccentHover = p.blueHover
	t.AccentPressed = p.bluePressed
	t.Focus = p.blue
	t.AccentText = p.accentText
	t.Success = p.green
	t.Warning = p.amber
	t.Danger = p.errorInk
	t.Selection = p.blue.Alpha(.3)
	t.Font = "Segoe UI, Microsoft YaHei UI"
	t.FontSize = 14
	t.Radius = 6
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
	ui.Text(c, appearanceText.title).FontSize(14)
	selected := normalizeAppearance(s.Appearance)
	ui.RadioGroup(c, func() {
		ui.Row(c).Gap(20).Children(func() {
			for _, choice := range []struct{ value, label string }{
				{appearanceLight, appearanceText.light}, {appearanceDark, appearanceText.dark}, {appearanceSystem, appearanceText.system},
			} {
				ui.Radio(c.Key("appearance-"+choice.value), &selected, choice.value, choice.label).OnChange(func() { s.setAppearance(selected) }).Disabled(s.Busy)
			}
		})
	})
	caption(c, appearanceText.detail)
}

var spectrumEase = ui.CubicBezier(0, 0, .4, 1)
var feedbackMotion = ui.Animation{Duration: 130 * time.Millisecond, Ease: spectrumEase}
var contentMotion = ui.Animation{Duration: 190 * time.Millisecond, Ease: spectrumEase}
var hoverTransition = ui.ElementTransition{Colors: true, Duration: 130 * time.Millisecond, Ease: spectrumEase}
var pageTransition = ui.ElementTransition{Duration: 190 * time.Millisecond, Ease: spectrumEase, Position: true, Enter: &ui.Motion{Y: 6}, Exit: &ui.Motion{Y: -4}}
var popupTransition = ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Enter: &ui.Motion{Y: 4, Scale: .985}, Exit: &ui.Motion{Y: 2, Scale: .99}}
var fadeTransition = ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Colors: true, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
var rowTransition = ui.ElementTransition{Duration: 190 * time.Millisecond, Ease: spectrumEase, Position: true, Colors: true, Enter: &ui.Motion{Y: 4}, Exit: &ui.Motion{}}

func tactile(e ui.Element) ui.Element {
	e.Transition(hoverTransition)
	pose := ui.Pose{}
	if e.Pressed() {
		pose.Scale = .97
	}
	return e.AnimatePose(pose, feedbackMotion)
}
