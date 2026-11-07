package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (s *State) libraryHero(c *ui.Context, sub Subscription) {
	anime := s.anime(sub.AnimeID)
	_, height := c.Size()
	ui.Row(c).Padding(12, 28, 16).Gap(20).AlignItems(ui.Start).Children(func() {
		cover(c, anime, 120, 168)
		ui.Scroll(c.Key("library-hero")).Grow(1).MinWidth(0).MaxHeight(height * .4).Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, anime.Title).FontSize(textSize(c, textTitle)).FontWeight(600)
				if sub.GroupName != "" {
					caption(c, sub.GroupName)
				}
				ui.Row(c).Wrap().Gap(controlGap).Children(func() {
					switch {
					case sub.Paused:
						badge(c, "已暂停", colors(c).muted, colors(c).canvas)
					case sub.Active:
						label := "下载中"
						if sub.Progress > 0 {
							label = fmt.Sprintf("下载中 %d%%", int(sub.Progress*100))
						}
						badge(c, label, colors(c).blue, colors(c).blueWash)
					default:
						badge(c, "自动追番中", colors(c).green, colors(c).greenWash)
					}
					if sub.NewCount > 0 {
						quiet(c, "library-new-"+sub.ID, fmt.Sprintf("新资源 %d 条", sub.NewCount), func() {
							s.viewSubscriptionResources(sub.ID)
							s.OnlyNew = true
						}).Label(fmt.Sprintf("新资源 %d 条 %s", sub.NewCount, sub.GroupName)).FontSize(textSize(c, textCaption)).FontWeight(600).TextColor(colors(c).amber).Background(colors(c).amberWash).Disabled(s.Busy)
					}
					caption(c, fmt.Sprintf("已完成 %d 个下载", s.completedDownloads(sub.AnimeID)))
				})
				if sub.CheckFailed {
					ui.Column(c).Gap(controlGap).Padding(8, 12).Radius(controlRadius).Background(colors(c).amberWash).Children(func() {
						ui.Text(c, "检查更新失败："+sourceLastErrorMessage(sub.LastError)).FontSize(textSize(c, textCaption)).TextColor(colors(c).amber)
						quiet(c, "retry-check-"+sub.ID, "重试检查", func() { s.check(sub.ID) }).Label("重试检查 " + sub.GroupName).FontSize(textSize(c, textCaption)).Disabled(sub.Paused || s.Busy)
					})
				}
			})
		})
		more := iconButton(c, "library-menu", "更多操作", "more", nil)
		if more.Clicked() {
			s.LibraryMenuOpen = !s.LibraryMenuOpen
		}
		ui.Popover(c, more, &s.LibraryMenuOpen, func() {
			ui.Column(c).Width(max(210, textSize(c, textBody)*9)).Padding(6).Gap(2).Children(func() {
				s.libraryMenuItem(c, "menu-view-resources", "查看资源", "查看资源 "+sub.GroupName, s.Busy, func() { s.viewSubscriptionResources(sub.ID) })
				s.libraryMenuItem(c, "menu-edit", "修改条件", "修改条件 "+sub.GroupName, s.Busy, func() { s.editSubscription(sub.ID) })
				pause := "暂停追番"
				if sub.Paused {
					pause = "恢复追番"
				}
				s.libraryMenuItem(c, "menu-pause", pause, pause+" "+sub.GroupName, s.Busy, func() { s.pauseSubscription(sub.ID) })
				s.libraryMenuItem(c, "menu-check", "检查更新", "检查更新 "+sub.GroupName, sub.Paused || s.Busy, func() { s.check(sub.ID) })
				if sub.Folder != "" && s.OpenFolder != nil {
					folder := sub.Folder
					s.libraryMenuItem(c, "menu-folder", "打开番剧文件夹", "打开番剧文件夹 "+sub.GroupName, s.Busy, func() { s.OpenFolder(folder) })
				}
				ui.Divider(c).Background(colors(c).line)
				s.libraryMenuItem(c, "menu-cancel", "取消追番", "取消追番 "+sub.GroupName, s.Busy, func() { s.CancelID, s.CancelPrompt = sub.ID, true })
			})
		})
	})
}
