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
	blue: ui.Hex("#35664b"), blueHover: ui.Hex("#2b573f"), bluePressed: ui.Hex("#214631"), blueWash: ui.Hex("#edf3ed"), accentText: ui.Hex("#ffffff"),
	muted: ui.Hex("#625e56"), ink: ui.Hex("#2e2c27"), line: ui.Hex("#e7e3db"), outline: ui.Hex("#bbb6ab"), hoverLine: ui.Hex("#918b7e"),
	chrome: ui.Hex("#f2efe9"), paper: ui.Hex("#fffefa"), canvas: ui.Hex("#f8f6f0"), brandWash: ui.Hex("#f5f3ed"),
	green: ui.Hex("#35664b"), greenWash: ui.Hex("#edf3ed"), amber: ui.Hex("#865a18"), amberWash: ui.Hex("#fbf1dc"),
	avoidInk: ui.Hex("#6f6659"), avoidWash: ui.Hex("#f3efe7"), errorInk: ui.Hex("#a53d38"), errorWash: ui.Hex("#fcf0ec"),
	scrim: ui.RGBA(0, 0, 0, .24), popoverShadow: ui.RGBA(0, 0, 0, .10),
}

var darkPalette = colorPalette{
	blue: ui.Hex("#a8c7ac"), blueHover: ui.Hex("#bfd8c2"), bluePressed: ui.Hex("#92b797"), blueWash: ui.Hex("#2e3d31"), accentText: ui.Hex("#202b23"),
	muted: ui.Hex("#c1bbae"), ink: ui.Hex("#eee9df"), line: ui.Hex("#47443d"), outline: ui.Hex("#827c6f"), hoverLine: ui.Hex("#aaa291"),
	chrome: ui.Hex("#24231f"), paper: ui.Hex("#2c2a25"), canvas: ui.Hex("#272620"), brandWash: ui.Hex("#f5f3ed"),
	green: ui.Hex("#a8c7ac"), greenWash: ui.Hex("#2e3d31"), amber: ui.Hex("#e4c58e"), amberWash: ui.Hex("#453b29"),
	avoidInk: ui.Hex("#c9bfaf"), avoidWash: ui.Hex("#3a352d"), errorInk: ui.Hex("#efb0a5"), errorWash: ui.Hex("#48312d"),
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
