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
	errorInk, errorWash                                ui.Color
	scrim, popoverShadow, cardShadow                   ui.Color
	sunken                                             ui.Color
}

var lightPalette = colorPalette{
	blue: ui.Hex("#285994"), blueHover: ui.Hex("#234d80"), bluePressed: ui.Hex("#1c406c"), blueWash: ui.Hex("#e1ebf8"), accentText: ui.Hex("#ffffff"),
	muted: ui.Hex("#526071"), ink: ui.Hex("#222b37"), line: ui.RGBA(34, 43, 55, .09), outline: ui.RGBA(34, 43, 55, .20), hoverLine: ui.RGBA(34, 43, 55, .35),
	chrome: ui.Hex("#edf0f4"), paper: ui.Hex("#ffffff"), canvas: ui.Hex("#f7f8fa"), brandWash: ui.Hex("#e8ecf1"),
	green: ui.Hex("#28664f"), greenWash: ui.Hex("#e9f2ee"), amber: ui.Hex("#7d541e"), amberWash: ui.Hex("#f7efe2"),
	errorInk: ui.Hex("#a63c38"), errorWash: ui.Hex("#f8ecec"),
	scrim: ui.RGBA(19, 25, 34, .32), popoverShadow: ui.RGBA(19, 25, 34, .14), cardShadow: ui.RGBA(19, 25, 34, .08),
	sunken: ui.Hex("#e6eaf0"),
}

var darkPalette = colorPalette{
	blue: ui.Hex("#96b9ec"), blueHover: ui.Hex("#afcaf2"), bluePressed: ui.Hex("#83a9df"), blueWash: ui.Hex("#293b53"), accentText: ui.Hex("#182535"),
	muted: ui.Hex("#b1bdcc"), ink: ui.Hex("#e8edf4"), line: ui.RGBA(232, 237, 244, .09), outline: ui.RGBA(232, 237, 244, .22), hoverLine: ui.RGBA(232, 237, 244, .36),
	chrome: ui.Hex("#1d2229"), paper: ui.Hex("#252b33"), canvas: ui.Hex("#171a1f"), brandWash: ui.Hex("#e8ecf1"),
	green: ui.Hex("#9bcdb6"), greenWash: ui.Hex("#263a34"), amber: ui.Hex("#e2bf89"), amberWash: ui.Hex("#3a3228"),
	errorInk: ui.Hex("#f0aaa8"), errorWash: ui.Hex("#3d2b30"),
	scrim: ui.RGBA(0, 0, 0, .60), popoverShadow: ui.RGBA(0, 0, 0, .45), cardShadow: ui.RGBA(0, 0, 0, .45),
	sunken: ui.Hex("#1b2027"),
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
	t.FontSize = float32(textBody)
	t.Radius = controlRadius
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
	ui.Text(c, appearanceText.title).FontSize(textSize(c, textBody))
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
var contentMotion = ui.Animation{Duration: 160 * time.Millisecond, Ease: spectrumEase}
var hoverTransition = ui.ElementTransition{Colors: true, Duration: 130 * time.Millisecond, Ease: spectrumEase}
var pageTransition = ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
var popupTransition = ui.ElementTransition{Duration: 140 * time.Millisecond, Ease: spectrumEase, Enter: &ui.Motion{Y: 2}, Exit: &ui.Motion{Y: 2}}
var fadeTransition = ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Colors: true, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
var rowTransition = ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Position: true, Colors: true, Enter: &ui.Motion{Y: 2}, Exit: &ui.Motion{}}

func tactile(e ui.Element) ui.Element {
	e.Transition(hoverTransition)
	pose := ui.Pose{}
	if e.Pressed() {
		pose.Scale = .99
	}
	return e.AnimatePose(pose, feedbackMotion)
}
