package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

type textRole float32

const (
	textMeta    textRole = 14
	textControl textRole = 14
	textBody    textRole = 16
	textSection textRole = 18
	textTitle   textRole = 18
)

const (
	radiusSmall      float32 = 8
	radiusControl    float32 = 12
	radiusPanel      float32 = 16
	radiusSheet      float32 = 24
	radiusDialog     float32 = 32
	radiusLargeSheet float32 = 40

	controlGap      float32 = 8
	controlPaddingY float32 = 12
	controlPaddingX float32 = 20
	compactPaddingY float32 = 8
	compactPaddingX float32 = 16

	sidebarWidth float32 = 224
	topbarHeight float32 = 64
	coverRadius  float32 = radiusControl
	posterGapX   float32 = 24
	posterGapY   float32 = 36
)

func textScale(c *ui.Context) float32 {
	scale := c.Preferences().TextScale
	if scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return 1
	}
	return scale
}

func textSize(c *ui.Context, role textRole) float32 {
	return float32(role) * textScale(c)
}

func pageInsetX(c *ui.Context) float32 {
	width, _ := c.Size()
	if width <= 1080 {
		return 32
	}
	return 40
}

func installTheme(c *ui.Context, dark bool) {
	theme := spectrumTheme(dark)
	theme.FontSize = textSize(c, textControl)
	if c.Preferences().HighContrast {
		theme.Border = theme.Border.Mix(theme.Text, .45)
		theme.TextMuted = theme.TextMuted.Mix(theme.Text, .4)
		theme.Focus.A = 255
		theme.Scrollbar = theme.Scrollbar.Alpha(1.6)
	}
	c.SetTheme(theme)
}

func inputHeight(c *ui.Context) float32 {
	return max(float32(44), textSize(c, textControl)*1.5+20)
}

func buttonHeight(c *ui.Context) float32 {
	return max(float32(44), textSize(c, textControl)*1.5+20)
}

func dialogContentWithFooter(c *ui.Context, panel ui.Element, key string, width, radius float32, content, footer func()) {
	pal := colors(c)
	windowWidth, windowHeight := c.Size()
	pad := float32(40)
	footerPadTop, footerPadBottom := float32(20), float32(32)
	if textScale(c) > 1.5 || windowHeight <= 720 {
		pad = 32
		footerPadTop, footerPadBottom = 16, 24
	}
	panel.Width(min(windowWidth-48, width*max(1, min(textScale(c), 1.5)))).
		MaxHeight(windowHeight-32).
		Radius(radius).
		Background(pal.surface).
		Border(1, pal.outline).
		Shadow(0, 16, 40, 0, pal.popoverShadow).
		Transition(dialogTransition).
		Clip()
	if c.Theme().Dark {
		panel.InsetShadow(0, 1, 0, 0, pal.topHighlight)
	}
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		ui.Scroll(c.Key(key)).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(pad, pad, 24).Gap(24).Children(content)
		})
		ui.Divider(c).Background(pal.line)
		ui.Row(c).Padding(footerPadTop, pad, footerPadBottom).Wrap().Justify(ui.End).Gap(12).Children(footer)
	})
}

func menuPanelStyle(c *ui.Context) func(ui.Element) {
	pal := colors(c)
	return func(panel ui.Element) {
		panel.Background(pal.surface).
			Border(1, pal.outline).
			Radius(radiusPanel).
			Padding(12).
			NoShadow().
			Transition(popupTransition)
	}
}
