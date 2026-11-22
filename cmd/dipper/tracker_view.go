package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
)

func (s *State) trackerSettings(c *ui.Context) {
	ui.Row(c).Wrap().AlignItems(ui.Center).Gap(16).Children(func() {
		ui.Column(c).Grow(1).MinWidth(200).Gap(5).Children(func() {
			ui.Text(c, "下载连接").FontSize(textSize(c, textBody))
			parts := []string{"默认连接已开启"}
			if count := len(core.ParseTrackers(s.ExtraTrackers)); count > 0 {
				parts = append(parts, fmt.Sprintf("%d 个手动地址", count))
			}
			if count := len(s.TrackerLists); count > 0 {
				parts = append(parts, fmt.Sprintf("%d 个订阅列表", count))
			}
			caption(c, strings.Join(parts, " · "))
		})
		secondary(c, "manage-trackers", "管理 Tracker", s.openTrackerManager).Disabled(s.Busy)
	})
}

func (s *State) trackerDialog(c *ui.Context) {
	draft := s.trackerManager
	if !s.trackerManagerOpen || draft == nil {
		return
	}
	cancel := func() { s.trackerManagerOpen, s.trackerManager = false, nil }
	ui.DialogBase(c, &s.trackerManagerOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(colors(c).scrim).Transition(fadeTransition)
		dialogContentWithFooter(c, panel, "tracker-content", 720, func() {
			heading(c, "管理下载连接")
			caption(c, "Tracker 帮助找到可以下载或分享文件的人。Dipper 已提供默认地址，也支持添加你自己的地址。")
			s.manualTrackerSection(c, draft)
			s.trackerSubscriptionSection(c, draft)
			s.automaticTrackerSection(c, draft)
		}, func() {
			if draft.message != "" {
				ink := colors(c).muted
				if draft.failed {
					ink = colors(c).errorInk
				}
				ui.Text(c, draft.message).FontSize(textSize(c, textCaption)).TextColor(ink).Grow(1).MinWidth(0)
			}
			secondary(c, "cancel-trackers", "取消", cancel)
			primary(c, "save-trackers", "保存修改", func() { s.saveTrackerManager(draft) }).
				Disabled(s.Core == nil || s.Busy || draft.reading || draft.editing != "")
		})
	})
}

func trackerSection(c *ui.Context, content func()) {
	ui.Column(c).Padding(16).Gap(10).Background(colors(c).canvas).Border(1, colors(c).line).Radius(8).Children(content)
}

func trackerAddress(c *ui.Context, address string) {
	ui.Text(c, address).FontSize(textSize(c, textCaption)).Selectable().Grow(1).MinWidth(0)
}

func (s *State) manualTrackerSection(c *ui.Context, draft *trackerManagerDraft) {
	locked := draft.reading || draft.editing != ""
	trackerSection(c, func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "手动添加的服务器").FontSize(textSize(c, textBody)).FontWeight(600).Grow(1)
			secondary(c, "add-tracker-server", "添加服务器", func() { draft.edit("manual", -1, "") }).Disabled(locked)
		})
		caption(c, "手头有一个或多个 Tracker 地址时使用。")
		if len(draft.manual) == 0 && draft.editing != "manual" {
			caption(c, "还没有手动添加的地址。")
		}
		for index, address := range draft.manual {
			if draft.editing == "manual" && index == draft.editIndex {
				s.manualTrackerEditor(c, draft)
				continue
			}
			key := "tracker-server-" + strconv.Itoa(index)
			ui.Row(c.Key(key)).AlignItems(ui.Center).Gap(10).Children(func() {
				trackerAddress(c, address)
				ui.PopoverMenuButton(c.Key(key+"-menu"), "管理", func(menu *ui.Menu) {
					if menu.Item("编辑").Chosen() {
						draft.edit("manual", index, address)
					}
					if menu.Item("复制地址").Disabled(s.CopyText == nil).Chosen() {
						s.CopyText(address)
					}
					if menu.Item("移除").Chosen() {
						draft.manual = slices.Delete(draft.manual, index, index+1)
					}
				}).Label(fmt.Sprintf("管理服务器 %d", index+1)).Disabled(locked)
			})
		}
		if draft.editing == "manual" && draft.editIndex < 0 {
			s.manualTrackerEditor(c, draft)
		}
	})
}

func (s *State) manualTrackerEditor(c *ui.Context, draft *trackerManagerDraft) {
	caption(c, "每行一个地址，支持 UDP、HTTP、HTTPS、WS 和 WSS。")
	ui.TextArea(c.Key("tracker-server-input"), &draft.input).Label("服务器地址").
		Placeholder("udp://tracker.example.net:1337/announce").FillWidth().Height(76)
	label := "添加到列表"
	if draft.editIndex >= 0 {
		label = "确认编辑"
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		primary(c, "confirm-tracker-server", label, draft.addManual)
		secondary(c, "cancel-tracker-server", "取消编辑", func() { draft.edit("", -1, "") })
	})
}

func (s *State) trackerSubscriptionSection(c *ui.Context, draft *trackerManagerDraft) {
	locked := draft.reading || draft.editing != ""
	trackerSection(c, func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "订阅的列表").FontSize(textSize(c, textBody)).FontWeight(600).Grow(1)
			secondary(c, "add-tracker-list", "订阅列表", func() { draft.edit("list", -1, "") }).Disabled(locked)
		})
		caption(c, "有一份在线地址清单时，只需添加清单链接。Dipper 会定期更新。")
		if len(draft.lists) == 0 && draft.editing != "list" {
			caption(c, "还没有订阅列表。")
		}
		for index, list := range draft.lists {
			s.trackerSubscriptionRow(c, draft, index, list, locked)
		}
		if draft.editing == "list" {
			s.trackerSubscriptionEditor(c, draft)
		}
	})
}

func (s *State) trackerSubscriptionRow(c *ui.Context, draft *trackerManagerDraft, index int, list core.TrackerList, locked bool) {
	key := "tracker-list-" + strconv.Itoa(index)
	ui.Column(c.Key(key)).Gap(5).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			trackerAddress(c, list.URL)
			open := "查看地址"
			if draft.expanded[list.URL] {
				open = "收起地址"
			}
			secondary(c, key+"-details", open, func() { draft.expanded[list.URL] = !draft.expanded[list.URL] })
			ui.PopoverMenuButton(c.Key(key+"-menu"), "管理", func(menu *ui.Menu) {
				if menu.Item("立即更新").Chosen() {
					s.readTrackerList(draft, index)
				}
				toggle := "停用"
				if list.Disabled {
					toggle = "启用"
				}
				if menu.Item(toggle).Chosen() {
					draft.lists[index].Disabled = !list.Disabled
				}
				if menu.Item("复制列表链接").Disabled(s.CopyText == nil).Chosen() {
					s.CopyText(list.URL)
				}
				if menu.Item("移除订阅").Chosen() {
					draft.lists = slices.Delete(draft.lists, index, index+1)
				}
			}).Label(fmt.Sprintf("管理订阅 %d", index+1)).Disabled(locked)
		})
		status := "已启用"
		if list.Disabled {
			status = "已停用"
		}
		caption(c, fmt.Sprintf("%s · %d 个地址 · 上次更新 %s", status, len(list.Trackers), list.FetchedAt.Local().Format("2006-01-02 15:04")))
		if draft.expanded[list.URL] {
			trackerAddressList(c, list.Trackers)
		}
	})
}

func (s *State) trackerSubscriptionEditor(c *ui.Context, draft *trackerManagerDraft) {
	caption(c, "粘贴 HTTP 或 HTTPS 文本列表链接，每行应有一个 Tracker 地址。")
	ui.TextInput(c.Key("tracker-list-input"), &draft.input).Label("在线列表链接").
		Placeholder("https://lists.example.net/trackers.txt").FillWidth().MinHeight(inputHeight(c)).
		Disabled(draft.reading).OnChange(func() {
		draft.preview = core.TrackerList{}
		draft.report("", false)
	}).OnSubmit(func() { s.readTrackerList(draft, -1) })
	if draft.preview.URL != "" {
		current := core.Settings{ExtraTrackers: strings.Join(draft.manual, "\n"), TrackerLists: draft.lists}
		existing := append(current.ActiveExtraTrackers(), s.trackerAutomatic...)
		existing = append(existing, downloader.BuiltinTrackers()...)
		known := make(map[string]bool, len(existing))
		for _, address := range existing {
			known[address] = true
		}
		duplicates := 0
		for _, address := range draft.preview.Trackers {
			if known[address] {
				duplicates++
			}
		}
		caption(c, fmt.Sprintf("读到 %d 个有效地址，其中 %d 个已配置。", len(draft.preview.Trackers), duplicates))
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		secondary(c, "preview-tracker-list", "预览地址", func() { s.readTrackerList(draft, -1) }).Disabled(draft.reading)
		if draft.preview.URL != "" {
			primary(c, "confirm-tracker-list", "添加订阅", func() {
				draft.lists = append(draft.lists, draft.preview)
				draft.edit("", -1, "")
			})
		}
		secondary(c, "cancel-tracker-list", "取消编辑", func() { draft.edit("", -1, "") }).Disabled(draft.reading)
	})
	if draft.preview.URL != "" {
		trackerAddressList(c, draft.preview.Trackers)
	}
}

func (s *State) automaticTrackerSection(c *ui.Context, draft *trackerManagerDraft) {
	trackerSection(c, func() {
		ui.Text(c, "Dipper 自动提供的地址").FontSize(textSize(c, textBody)).FontWeight(600)
		builtins := downloader.BuiltinTrackers()
		caption(c, fmt.Sprintf("在线列表 %d 个 · 内置备用 %d 个", len(s.trackerAutomatic), len(builtins)))
		ui.Row(c).Wrap().Gap(8).Children(func() {
			label := "查看在线列表"
			if draft.automatic {
				label = "收起在线列表"
			}
			secondary(c, "tracker-automatic", label, func() { draft.automatic = !draft.automatic })
			label = "查看内置地址"
			if draft.builtin {
				label = "收起内置地址"
			}
			secondary(c, "tracker-builtin", label, func() { draft.builtin = !draft.builtin })
		})
		if draft.automatic {
			if len(s.trackerAutomatic) == 0 {
				caption(c, "尚未取得在线列表，内置地址仍可使用。")
			} else {
				caption(c, "XIU2 精选列表")
				trackerAddressList(c, s.trackerAutomatic)
			}
		}
		if draft.builtin {
			caption(c, "内置备用地址")
			trackerAddressList(c, builtins)
		}
	})
}

func trackerAddressList(c *ui.Context, addresses []string) {
	ui.Column(c).Gap(4).Children(func() {
		for _, address := range addresses {
			ui.Text(c, address).FontSize(textSize(c, textCaption)).Selectable().MinWidth(0)
		}
	})
}
