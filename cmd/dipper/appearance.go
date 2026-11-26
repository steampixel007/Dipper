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
	canvas, chrome, surface, sunken ui.Color
	ink, muted                      ui.Color
	accent, accentHover             ui.Color
	accentPressed, accentWash       ui.Color
	accentInk, accentText           ui.Color
	warningInk, warningWash         ui.Color
	errorInk, errorWash             ui.Color
	line, outline, hoverLine        ui.Color
	scrim, brandWash                ui.Color
}

var lightPalette = colorPalette{
	canvas:        ui.Hex("#FFFFFF"),
	chrome:        ui.Hex("#F5F5F7"),
	surface:       ui.Hex("#FFFFFF"),
	sunken:        ui.Hex("#EFEFF2"),
	ink:           ui.Hex("#181818"),
	muted:         ui.Hex("#6A6A6A"),
	accent:        ui.Hex("#0071E3"),
	accentHover:   ui.Hex("#0064CC"),
	accentPressed: ui.Hex("#0055B3"),
	accentWash:    ui.Hex("#E8F2FC"),
	// accentInk uses the darker accentHover stop so accent text on accentWash
	// and chrome exceeds 5.0:1 contrast while staying within the single blue.
	accentInk:   ui.Hex("#0064CC"),
	accentText:  ui.Hex("#FFFFFF"),
	warningInk:  ui.Hex("#9A6018"),
	warningWash: ui.Hex("#FBF4EA"),
	errorInk:    ui.Hex("#B93232"),
	errorWash:   ui.Hex("#FBF0F0"),
	line:        ui.RGBA(24, 24, 24, .08),
	outline:     ui.RGBA(24, 24, 24, .16),
	hoverLine:   ui.RGBA(24, 24, 24, .30),
	scrim:       ui.RGBA(0, 0, 0, .32),
	brandWash:   ui.Hex("#EFEFF2"),
}

var darkPalette = colorPalette{
	canvas:        ui.Hex("#121212"),
	chrome:        ui.Hex("#1C1C1E"),
	surface:       ui.Hex("#1E1E20"),
	sunken:        ui.Hex("#262629"),
	ink:           ui.Hex("#F5F5F5"),
	muted:         ui.Hex("#9A9A9A"),
	accent:        ui.Hex("#2997FF"),
	accentHover:   ui.Hex("#47A7FF"),
	accentPressed: ui.Hex("#1482EB"),
	accentWash:    ui.Hex("#172A3E"),
	accentInk:     ui.Hex("#2997FF"),
	accentText:    ui.Hex("#0B1219"),
	warningInk:    ui.Hex("#E5B567"),
	warningWash:   ui.Hex("#30281B"),
	errorInk:      ui.Hex("#F28B82"),
	errorWash:     ui.Hex("#332020"),
	line:          ui.RGBA(245, 245, 245, .08),
	outline:       ui.RGBA(245, 245, 245, .18),
	hoverLine:     ui.RGBA(245, 245, 245, .32),
	scrim:         ui.RGBA(0, 0, 0, .60),
	brandWash:     ui.Hex("#262629"),
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
	t.SurfaceHover = p.chrome
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
	t.Selection = p.accent.Alpha(.25)
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
	ui.Text(c, appearanceText.title).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(colors(c).ink).LineHeight(1.5)
	selected := normalizeAppearance(s.Appearance)
	ui.RadioGroup(c, func() {
		ui.Row(c).Wrap().Gap(24).Children(func() {
			for _, choice := range []struct{ value, label string }{
				{appearanceLight, appearanceText.light}, {appearanceDark, appearanceText.dark}, {appearanceSystem, appearanceText.system},
			} {
				ui.Radio(c.Key("appearance-"+choice.value), &selected, choice.value, choice.label).OnChange(func() { s.setAppearance(selected) }).Disabled(s.Busy)
			}
		})
	})
}

var spectrumEase = ui.CubicBezier(0, 0, .4, 1)
var feedbackMotion = ui.Animation{Duration: 130 * time.Millisecond, Ease: spectrumEase}
var contentMotion = ui.Animation{Duration: 160 * time.Millisecond, Ease: spectrumEase}
var hoverTransition = ui.ElementTransition{Colors: true, Duration: 130 * time.Millisecond, Ease: spectrumEase}
var revealTransition = ui.ElementTransition{Duration: 130 * time.Millisecond, Ease: spectrumEase, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
var pageTransition = ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
var popupTransition = ui.ElementTransition{Duration: 140 * time.Millisecond, Ease: spectrumEase, Enter: &ui.Motion{Y: 4}, Exit: &ui.Motion{Y: 4}}
var fadeTransition = ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Colors: true, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
var rowTransition = ui.ElementTransition{Duration: 130 * time.Millisecond, Ease: spectrumEase, Colors: true}

func tactile(e ui.Element) ui.Element {
	return e.Transition(hoverTransition)
}
