package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (s *State) libraryHero(c *ui.Context, sub Subscription, embedded bool) {
	anime := s.anime(sub.AnimeID)
	pal := colors(c)
	_, height := c.Size()
	heroContent := func() {
		ui.Column(c).Gap(12).Children(func() {
			pageTitle(c, anime.Title)
			if sub.GroupName != "" {
				caption(c, sub.GroupName)
			}
			ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
				switch {
				case sub.Paused:
					caption(c, "已暂停")
				case sub.Active:
					label := "下载中"
					if sub.Progress > 0 {
						label = fmt.Sprintf("下载中 %d%%", int(sub.Progress*100))
					}
					statusPill(c, label, pal.accentInk, pal.accentWash)
				default:
					caption(c, "自动追番中")
				}
				if sub.NewCount > 0 {
					compactQuiet(c, "library-new-"+sub.ID, fmt.Sprintf("新资源 %d 条", sub.NewCount), func() {
						s.viewSubscriptionResources(sub.ID)
						s.OnlyNew = true
					}).Label(fmt.Sprintf("新资源 %d 条 %s", sub.NewCount, sub.GroupName)).TextColor(pal.warningInk).Background(pal.warningWash).Disabled(s.Busy)
				}
				caption(c, fmt.Sprintf("已完成 %d 个下载", s.completedDownloads(sub.AnimeID)))
			})
			if sub.CheckFailed {
				ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Padding(12, 16).Radius(radiusControl).Background(pal.warningWash).Children(func() {
					ui.Text(c, "检查更新失败："+sourceLastErrorMessage(sub.LastError)).FontSize(textSize(c, textMeta)).TextColor(pal.warningInk).LineHeight(1.5).Grow(1).MinWidth(0)
					compactQuiet(c, "retry-check-"+sub.ID, "重试检查", func() { s.check(sub.ID) }).Label("重试检查 " + sub.GroupName).TextColor(pal.warningInk).Disabled(sub.Paused || s.Busy)
				})
			}
		})
	}
	ui.Row(c).Padding(16, pageInsetX(c), 24).Gap(24).AlignItems(ui.Start).Children(func() {
		s.featuredCover(c, anime, 96, 132)
		if embedded {
			ui.Column(c).Grow(1).MinWidth(0).Children(heroContent)
		} else {
			ui.Scroll(c.Key("library-hero")).Grow(1).MinWidth(0).MaxHeight(height * .4).Children(heroContent)
		}
		more := iconButton(c, "library-menu", "更多操作", "more", nil)
		if more.Clicked() {
			s.LibraryMenuOpen = !s.LibraryMenuOpen
		}
		pauseLabel := "暂停追番"
		if sub.Paused {
			pauseLabel = "恢复追番"
		}
		folder := sub.Folder
		fileItems := []cascadingMenuItem{
			{
				Key:      "folder",
				Label:    "打开番剧文件夹",
				Disabled: s.Busy || folder == "" || s.OpenFolder == nil,
				Action: func() {
					if s.OpenFolder != nil {
						s.OpenFolder(folder)
					}
				},
			},
			{
				Key:      "downloads",
				Label:    "查看下载任务",
				Disabled: s.Busy,
				Action: func() {
					s.DownloadFilter = downloadsAll
					s.DownloadSearch = anime.Title
					s.enterPage("downloads")
				},
			},
			{
				Key:      "history",
				Label:    "查看下载记录",
				Disabled: s.Busy,
				Action: func() {
					s.enterPage("downloads")
					s.DownloadHistory = true
					s.DownloadHistoryScroll = ui.ScrollState{}
				},
			},
		}
		menuItems := []cascadingMenuItem{
			{
				Key:      "view-resources",
				Label:    "查看资源",
				Disabled: s.Busy,
				Action:   func() { s.viewSubscriptionResources(sub.ID) },
			},
			{
				Key:      "edit",
				Label:    "修改条件",
				Disabled: s.Busy,
				Action:   func() { s.editSubscription(sub.ID) },
			},
			{
				Key:      "pause",
				Label:    pauseLabel,
				Disabled: s.Busy,
				Action:   func() { s.pauseSubscription(sub.ID) },
			},
			{
				Key:      "check",
				Label:    "检查更新",
				Disabled: sub.Paused || s.Busy,
				Action:   func() { s.check(sub.ID) },
			},
			{
				Key:      "files-group",
				Label:    "文件与记录",
				Disabled: s.Busy,
				Submenu:  fileItems,
			},
			{Key: "sep-cancel", Separator: true},
			{
				Key:      "cancel",
				Label:    "取消追番",
				Disabled: s.Busy,
				Danger:   true,
				Action:   func() { s.CancelID, s.CancelPrompt = sub.ID, true },
			},
		}
		cascadingMenu(c, "library-menu-popup-"+sub.ID, more, &s.LibraryMenuOpen, menuItems)
	})
}
