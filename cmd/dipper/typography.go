package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

type textRole float32

const (
	textCaption textRole = 12
	textBody    textRole = 14
	textSection textRole = 18
	textTitle   textRole = 26
)

const (
	controlRadius   float32 = 8
	controlGap      float32 = 8
	controlPaddingY float32 = 8
	controlPaddingX float32 = 12
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

func installTheme(c *ui.Context, dark bool) {
	theme := spectrumTheme(dark)
	theme.FontSize = textSize(c, textBody)
	if c.Preferences().HighContrast {
		theme.Border = theme.Border.Mix(theme.Text, .45)
		theme.TextMuted = theme.TextMuted.Mix(theme.Text, .4)
		theme.Focus.A = 255
		theme.Scrollbar = theme.Scrollbar.Alpha(1.6)
	}
	c.SetTheme(theme)
}

func inputHeight(c *ui.Context) float32 {
	return textSize(c, textBody)*1.5 + 16
}

func dialogContent(c *ui.Context, panel ui.Element, key string, width float32, content func()) {
	windowWidth, windowHeight := c.Size()
	panel.Width(min(windowWidth-48, width*max(1, min(textScale(c), 1.5)))).Radius(12).Background(colors(c).paper)
	ui.Scroll(c.Key(key)).MaxHeight(windowHeight - 48).MinHeight(0).Children(func() {
		ui.Column(c).Padding(28).Gap(18).Children(content)
	})
}

func dialogContentWithFooter(c *ui.Context, panel ui.Element, key string, width float32, content, footer func()) {
	windowWidth, windowHeight := c.Size()
	panel.Width(min(windowWidth-48, width*max(1, min(textScale(c), 1.5)))).MaxHeight(windowHeight - 32).Radius(12).Background(colors(c).paper)
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		ui.Scroll(c.Key(key)).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(28, 28, 16).Gap(18).Children(content)
		})
		ui.Divider(c).Background(colors(c).line)
		ui.Row(c).Padding(16, 28, 24).Wrap().Justify(ui.End).Gap(controlGap).Children(footer)
	})
}
