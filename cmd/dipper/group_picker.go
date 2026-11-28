package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (s *State) groupPicker(c *ui.Context) {
	pal := colors(c)
	query := strings.TrimSpace(normalize(s.GroupSearch))
	var matches []int
	for index, name := range s.Groups {
		if strings.Contains(normalize(name), query) {
			matches = append(matches, index)
		}
	}
	if len(s.Groups) > 1 || s.GroupSearch != "" {
		ui.SearchField(c.Key("group-search"), &s.GroupSearch).Label("搜索字幕组").Placeholder("输入字幕组名称").FillWidth().MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).Disabled(s.Busy)
		if query == "" {
			caption(c, fmt.Sprintf("%d 个字幕组", len(s.Groups)))
		} else {
			caption(c, fmt.Sprintf("找到 %d / %d 个字幕组", len(matches), len(s.Groups)))
		}
	}
	if len(matches) == 0 {
		if query != "" {
			caption(c, "没有找到这个字幕组。")
		} else {
			caption(c, "来源暂未提供字幕组。")
		}
		return
	}
	ui.Column(c).Gap(8).Children(func() {
		for _, index := range matches {
			name := s.Groups[index]
			button := ui.ButtonBase(c.Key(fmt.Sprintf("group-%d", index))).
				Label("选择字幕组 "+name).
				Column().AlignItems(ui.Start).Gap(4).FillWidth().
				Padding(12, 16).Radius(radiusControl).Cursor(ui.CursorPointer)
			if s.Group == index {
				button.Background(pal.accentWash)
			} else if button.Hovered() {
				button.Background(pal.chrome)
			}
			followed := index < len(s.GroupIDs) && s.followedGroup(s.GroupIDs[index])
			tactile(button).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					if followed {
						statusPill(c, "正在追", pal.accentInk, pal.accentWash)
					}
					textColor := pal.ink
					if s.Group == index {
						textColor = pal.accentInk
					}
					ui.Text(c, name).FontWeight(500).FontSize(textSize(c, textBody)).TextColor(textColor).LineHeight(1.5).Grow(1).MinWidth(0)
					if s.Group == index {
						ui.Box(c).TextColor(pal.accentInk).Children(func() { icon(c, "check") })
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

// followedGroup reports whether the current anime already follows the named
// group, so the picker can mark it 正在追.
func (s *State) followedGroup(groupID string) bool {
	for _, sub := range s.Subscriptions {
		if sub.AnimeID == s.AnimeID && sub.GroupID == groupID {
			return true
		}
	}
	return false
}
