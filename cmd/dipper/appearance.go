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
	scrim, popoverShadow, cardShadow                   ui.Color
	glass, sunken, glowBlue, glowViolet                ui.Color
}

var lightPalette = colorPalette{
	blue: ui.Hex("#4661e8"), blueHover: ui.Hex("#3a52d0"), bluePressed: ui.Hex("#2f44b4"), blueWash: ui.Hex("#eef1ff"), accentText: ui.Hex("#ffffff"),
	muted: ui.RGBA(35, 40, 56, .62), ink: ui.Hex("#171a22"), line: ui.RGBA(23, 26, 34, .07), outline: ui.RGBA(23, 26, 34, .16), hoverLine: ui.RGBA(23, 26, 34, .30),
	chrome: ui.Hex("#fcfcff"), paper: ui.Hex("#ffffff"), canvas: ui.Hex("#f7f8fc"), brandWash: ui.Hex("#eef2ff"),
	green: ui.Hex("#287958"), greenWash: ui.Hex("#e8f6f0"), amber: ui.Hex("#9c5a0a"), amberWash: ui.Hex("#fdf2e3"),
	avoidInk: ui.Hex("#6b6257"), avoidWash: ui.Hex("#f4f1ec"), errorInk: ui.Hex("#c33a34"), errorWash: ui.Hex("#fdeeec"),
	scrim: ui.RGBA(16, 18, 26, .32), popoverShadow: ui.RGBA(20, 24, 40, .12), cardShadow: ui.RGBA(20, 24, 40, .10),
	glass: ui.RGBA(255, 255, 255, .58), sunken: ui.RGBA(23, 26, 34, .045),
	glowBlue: ui.RGBA(122, 156, 255, .40), glowViolet: ui.RGBA(196, 168, 255, .32),
}

var darkPalette = colorPalette{
	blue: ui.Hex("#9db4ff"), blueHover: ui.Hex("#b3c6ff"), bluePressed: ui.Hex("#8098e8"), blueWash: ui.Hex("#1d2540"), accentText: ui.Hex("#101526"),
	muted: ui.RGBA(232, 236, 248, .78), ink: ui.Hex("#f2f4fa"), line: ui.RGBA(255, 255, 255, .08), outline: ui.RGBA(255, 255, 255, .18), hoverLine: ui.RGBA(255, 255, 255, .32),
	chrome: ui.RGBA(19, 22, 30, .72), paper: ui.Hex("#161a24"), canvas: ui.Hex("#0f1117"), brandWash: ui.Hex("#edf2ff"),
	green: ui.Hex("#6fd3ab"), greenWash: ui.Hex("#143229"), amber: ui.Hex("#f0bf7a"), amberWash: ui.Hex("#3a2c17"),
	avoidInk: ui.Hex("#b9b3a7"), avoidWash: ui.Hex("#242219"), errorInk: ui.Hex("#ff9d95"), errorWash: ui.Hex("#3a2020"),
	scrim: ui.RGBA(0, 0, 0, .60), popoverShadow: ui.RGBA(0, 0, 0, .45), cardShadow: ui.RGBA(0, 0, 0, .45),
	glass: ui.RGBA(10, 13, 20, .42), sunken: ui.RGBA(255, 255, 255, .04),
	glowBlue: ui.RGBA(78, 107, 255, .22), glowViolet: ui.RGBA(150, 110, 255, .16),
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
var feedbackMotion = ui.Animation{Spring: ui.SpringOptions{Duration: 220 * time.Millisecond, Bounce: .18}}
var contentMotion = ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: .22}}
var hoverTransition = ui.ElementTransition{Colors: true, Duration: 130 * time.Millisecond, Ease: spectrumEase}
var pageTransition = ui.ElementTransition{Duration: 240 * time.Millisecond, Ease: ui.CubicBezier(.22, 1, .36, 1), Position: true, Enter: &ui.Motion{Y: 10}, Exit: &ui.Motion{Y: -6}}
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
