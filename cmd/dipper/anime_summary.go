package main

import "github.com/egoist/mygo/ui"

func (s *State) animeSummary(c *ui.Context, anime Anime) {
	ui.Column(c).Gap(8).Children(func() {
		heading(c, "番剧简介")
		switch {
		case anime.Summary != "":
			ui.Text(c, anime.Summary).FontSize(textSize(c, textBody)).TextColor(colors(c).muted).MaxLines(4)
			quiet(c, "read-summary", "查看完整简介", func() { s.SummaryOpen = true }).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Padding(3, 0).AlignSelf(ui.Start)
		case s.DetailLoading:
			caption(c, "正在加载简介…")
		case s.DetailError != "":
			caption(c, "简介暂时无法加载，可重试下方的详情。")
		default:
			caption(c, "来源暂未提供简介。")
		}
		ui.Divider(c).Background(colors(c).line).Margin(4, 0)
	})
}

func (s *State) summaryDialog(c *ui.Context) {
	ui.DialogBase(c, &s.SummaryOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(colors(c).scrim).Transition(fadeTransition)
		dialogContentWithFooter(c, panel, "summary-content", 560, func() {
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, s.anime(s.AnimeID).Title).FontSize(textSize(c, textSection)).FontWeight(700).Grow(1).MinWidth(0)
				iconButton(c, "close-summary", "关闭简介", "close", func() { s.SummaryOpen = false })
			})
			ui.Text(c, s.anime(s.AnimeID).Summary).FontSize(textSize(c, textBody)).TextColor(colors(c).ink)
			caption(c, "简介来自蜜柑计划")
		}, func() {
			secondary(c, "close-summary-footer", "关闭", func() { s.SummaryOpen = false })
		})
	})
}
