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
	sunken                                             ui.Color
}

var lightPalette = colorPalette{
	blue: ui.Hex("#285ea8"), blueHover: ui.Hex("#204f91"), bluePressed: ui.Hex("#194177"), blueWash: ui.Hex("#eaf1fa"), accentText: ui.Hex("#ffffff"),
	muted: ui.Hex("#626963"), ink: ui.Hex("#222725"), line: ui.RGBA(34, 39, 37, .09), outline: ui.RGBA(34, 39, 37, .20), hoverLine: ui.RGBA(34, 39, 37, .35),
	chrome: ui.Hex("#f0f1ef"), paper: ui.Hex("#ffffff"), canvas: ui.Hex("#fafaf9"), brandWash: ui.Hex("#e7eae5"),
	green: ui.Hex("#287250"), greenWash: ui.Hex("#ebf5ef"), amber: ui.Hex("#8b590e"), amberWash: ui.Hex("#fbf2e1"),
	avoidInk: ui.Hex("#685e55"), avoidWash: ui.Hex("#f1eeea"), errorInk: ui.Hex("#b73731"), errorWash: ui.Hex("#fcedeb"),
	scrim: ui.RGBA(18, 22, 20, .32), popoverShadow: ui.RGBA(18, 22, 20, .14), cardShadow: ui.RGBA(18, 22, 20, .08),
	sunken: ui.Hex("#ebedea"),
}

var darkPalette = colorPalette{
	blue: ui.Hex("#9ec3f5"), blueHover: ui.Hex("#b6d2f8"), bluePressed: ui.Hex("#87ade1"), blueWash: ui.Hex("#233247"), accentText: ui.Hex("#162331"),
	muted: ui.Hex("#b4bbb5"), ink: ui.Hex("#ecefed"), line: ui.RGBA(236, 239, 237, .09), outline: ui.RGBA(236, 239, 237, .22), hoverLine: ui.RGBA(236, 239, 237, .36),
	chrome: ui.Hex("#1b1d20"), paper: ui.Hex("#202326"), canvas: ui.Hex("#141618"), brandWash: ui.Hex("#e7eae5"),
	green: ui.Hex("#8bcda8"), greenWash: ui.Hex("#20372a"), amber: ui.Hex("#e7bd7c"), amberWash: ui.Hex("#3b301f"),
	avoidInk: ui.Hex("#c5bab0"), avoidWash: ui.Hex("#322d28"), errorInk: ui.Hex("#f5a6a0"), errorWash: ui.Hex("#3d2727"),
	scrim: ui.RGBA(0, 0, 0, .60), popoverShadow: ui.RGBA(0, 0, 0, .45), cardShadow: ui.RGBA(0, 0, 0, .45),
	sunken: ui.Hex("#191c1f"),
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
