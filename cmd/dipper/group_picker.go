package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (s *State) groupPicker(c *ui.Context) {
	caption(c, "不同字幕组可能发布不同版本。先选一组，查看资源。")
	query := strings.TrimSpace(normalize(s.GroupSearch))
	var matches []int
	for index, name := range s.Groups {
		if strings.Contains(normalize(name), query) {
			matches = append(matches, index)
		}
	}
	if len(s.Groups) > 1 || s.GroupSearch != "" {
		ui.TextInput(c.Key("group-search"), &s.GroupSearch).Label("搜索字幕组").Placeholder("输入字幕组名称").FillWidth().Height(34).Disabled(s.Busy)
		ui.Row(c).Gap(6).Children(func() {
			if query == "" {
				caption(c, fmt.Sprintf("%d 个字幕组", len(s.Groups)))
			} else {
				caption(c, fmt.Sprintf("找到 %d / %d 个字幕组", len(matches), len(s.Groups)))
			}
			ui.Spacer(c)
			if s.GroupSearch != "" {
				quiet(c, "clear-group-search", "清除搜索", func() { s.GroupSearch = "" }).FontSize(12).Padding(3, 0).TextColor(colors(c).blue).Disabled(s.Busy)
			}
		})
	}
	if len(matches) == 0 {
		if query != "" {
			caption(c, "没有找到这个字幕组，试试简称或清除搜索。")
		} else {
			caption(c, "来源暂未提供字幕组。")
		}
		return
	}
	ui.Column(c).Gap(8).Children(func() {
		for _, index := range matches {
			name := s.Groups[index]
			button := ui.ButtonBase(c.Key(fmt.Sprintf("group-%d", index))).Label("选择字幕组 "+name).Column().AlignItems(ui.Start).Gap(5).FillWidth().Padding(11, 13).Border(1, colors(c).line).Radius(8).Background(colors(c).paper).Cursor(ui.CursorPointer)
			if s.Group == index {
				button.BorderColor(colors(c).blue).Background(colors(c).blueWash)
			} else if button.Hovered() {
				button.BorderColor(colors(c).hoverLine)
			}
			tactile(button).Children(func() {
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					ui.Text(c, name).FontWeight(600).FontSize(13).Grow(1).MinWidth(0).MaxLines(2)
					if s.Group == index {
						ui.Box(c).TextColor(colors(c).blue).Children(func() { icon(c, "check") })
					}
				})
				count := "查看发布记录"
				if index < len(s.GroupCounts) && s.GroupCounts[index] >= 0 {
					count = fmt.Sprintf("%d 条资源", s.GroupCounts[index])
				}
				caption(c, count)
			}).OnClick(func() { s.chooseGroup(index) }).Disabled(s.Busy || index >= len(s.GroupIDs) || s.GroupIDs[index] == "")
		}
	})
}
