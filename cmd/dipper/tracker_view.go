package main

import (
	"fmt"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/trackers"
)

func (s *State) trackerSettings(c *ui.Context) {
	ui.Row(c).Wrap().AlignItems(ui.Center).Gap(16).Children(func() {
		ui.Text(c, "查找下载来源").FontSize(textSize(c, textBody)).FontWeight(500).TextColor(colors(c).ink).LineHeight(1.5).Grow(1).MinWidth(200)
		secondary(c, "manage-trackers", "管理地址", s.openTrackerManager).Disabled(s.Busy)
	})
}

func (s *State) trackerDialog(c *ui.Context) {
	draft := s.trackerManager
	if !s.trackerManagerOpen || draft == nil {
		return
	}
	pal := colors(c)
	cancel := func() { s.trackerManagerOpen, s.trackerManager = false, nil }
	ui.DialogBase(c, &s.trackerManagerOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.scrim).Transition(fadeTransition)
		dialogContentWithFooter(c, panel, "tracker-content-"+draft.selectedList, 720, radiusLargeSheet, func() {
			if draft.selectedList != "" {
				s.trackerListDetails(c, draft)
				return
			}
			pageTitle(c, "查找下载来源")
			s.trackerSubscriptionSection(c, draft)
			s.manualTrackerSection(c, draft)
		}, func() {
			message, ink := draft.message, pal.muted
			if draft.failed {
				ink = pal.errorInk
			}
			if message != "" {
				ui.Text(c, message).FontSize(textSize(c, textMeta)).TextColor(ink).LineHeight(1.5).Grow(1).MinWidth(0)
			}
			secondary(c, "cancel-trackers", "取消", cancel)
			primary(c, "save-trackers", "保存修改", func() { s.saveTrackerManager(draft) }).
				Disabled(s.Core == nil || s.Busy || draft.reading || draft.editing != "")
		})
	})
}

func trackerSection(c *ui.Context, content func()) {
	ui.Column(c).Gap(16).Children(content)
}

func trackerAddress(c *ui.Context, address string) {
	ui.Text(c, address).FontSize(textSize(c, textMeta)).TextColor(colors(c).ink).LineHeight(1.5).Selectable().Grow(1).MinWidth(0)
}

func (s *State) manualTrackerSection(c *ui.Context, draft *trackerManagerDraft) {
	locked := draft.reading || draft.editing != ""
	trackerSection(c, func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
			heading(c, "手动添加的地址")
			ui.Spacer(c)
			compactSecondary(c, "add-tracker-server", "添加地址", func() { draft.edit("manual", -1, "") }).Disabled(locked)
		})
		if len(draft.manual) == 0 && draft.editing != "manual" {
			caption(c, "还没有手动添加的地址。")
		}
		for index, address := range draft.manual {
			if draft.editing == "manual" && index == draft.editIndex {
				s.manualTrackerEditor(c, draft)
				continue
			}
			key := "tracker-server-" + strconv.Itoa(index)
			ui.Row(c.Key(key)).Wrap().AlignItems(ui.Center).Gap(12).Padding(8, 0).Children(func() {
				trackerAddress(c, address)
				ui.PopoverMenuButton(c.Key(key+"-menu"), "管理", func(menu *ui.Menu) {
					if menu.Item("编辑").Chosen() {
						draft.edit("manual", index, address)
					}
					if menu.Item("复制地址").Disabled(s.CopyText == nil).Chosen() {
						s.CopyText(address)
					}
					if menu.Item("删除").Chosen() {
						draft.manual = slices.Delete(draft.manual, index, index+1)
					}
				}, ui.PopoverMenuButtonOptions{Panel: menuPanelStyle(c)}).Label("管理 " + address).Disabled(locked)
				if slices.Contains(draft.disabled, address) {
					compactSecondary(c, "restore", "添加回来", func() { draft.setAddressRemoved(address, false) }).Label("添加回来 " + address).Disabled(locked)
				}
			})
		}
		if draft.editing == "manual" && draft.editIndex < 0 {
			s.manualTrackerEditor(c, draft)
		}
	})
}

func (s *State) manualTrackerEditor(c *ui.Context, draft *trackerManagerDraft) {
	ui.TextArea(c.Key("tracker-server-input"), &draft.input).Label("下载来源地址").
		Placeholder("udp://tracker.example.net:1337/announce").FillWidth().Height(80).Radius(radiusSmall).Background(colors(c).surface)
	label := "添加到列表"
	if draft.editIndex >= 0 {
		label = "确认编辑"
	}
	ui.Row(c).Wrap().Gap(12).Children(func() {
		primary(c, "confirm-tracker-server", label, draft.addManual)
		secondary(c, "cancel-tracker-server", "取消编辑", func() { draft.edit("", -1, "") })
	})
}

func (s *State) trackerSubscriptionSection(c *ui.Context, draft *trackerManagerDraft) {
	locked := draft.reading || draft.editing != ""
	trackerSection(c, func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
			heading(c, "在线清单")
			ui.Spacer(c)
			compactSecondary(c, "add-tracker-list", "订阅清单", func() { draft.edit("list", -1, "") }).Disabled(locked)
		})
		trackerListRow(c.Key(automaticTrackerList), draft, automaticTrackerList, "Dipper 推荐清单", "自动更新 · 来自 XIU2", s.trackerAutomatic.URLs, draft.automaticOff)
		if len(draft.lists) == 0 && draft.editing != "list" {
			caption(c, "还没有自己订阅的清单。")
		}
		if len(draft.lists) > 0 && draft.editing != "list" {
			browser := &draft.listBrowser
			if len(draft.lists) > 4 || browser.query != "" {
				trackerSearch(c.Key("search-lists"), browser, "搜索清单", "按名称或链接搜索")
			}
			var matching []core.TrackerList
			for _, list := range draft.lists {
				if browser.matches(list.URL) {
					matching = append(matching, list)
				}
			}
			start, end, pages := browser.bounds(len(matching), 4)
			for _, list := range matching[start:end] {
				name, host := trackerListName(list.URL)
				trackerListRow(c.Key(list.URL), draft, list.URL, name, host, list.Trackers, list.Disabled)
			}
			if len(matching) == 0 {
				caption(c, "没有找到匹配的清单。")
			}
			trackerPagination(c.Key("list-pages"), browser, pages, fmt.Sprintf("%d 份清单", len(matching)), "清单")
		}
		if draft.editing == "list" {
			s.trackerSubscriptionEditor(c, draft)
		}
	})
}

func trackerListName(address string) (name, source string) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" {
		return address, ""
	}
	name = path.Base(strings.TrimSuffix(parsed.Path, "/"))
	if name == "." || name == "/" {
		return parsed.Host, ""
	}
	return name, parsed.Host + strings.TrimSuffix(parsed.EscapedPath(), "/")
}

func trackerListRow(c *ui.Context, draft *trackerManagerDraft, key, title, source string, addresses []string, removed bool) {
	pal := colors(c)
	ui.Row(c).AlignItems(ui.Center).Gap(16).Padding(8, 0).Children(func() {
		ui.Column(c).MinWidth(0).Grow(1).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5)
			count := fmt.Sprintf("%d 个地址", trackerAddressCount(draft, addresses))
			if removed {
				count = "已删除"
			}
			if source != "" {
				count += " · " + source
			}
			caption(c, count)
		})
		label := "查看地址"
		if removed {
			label = "查看清单"
		}
		accessible := key
		if key == automaticTrackerList {
			accessible = title
		}
		compactSecondary(c, "details", label, func() { draft.selectList(key) }).Label("查看 " + accessible).
			Disabled(draft.reading || draft.editing != "")
	})
}

func trackerAddressCount(draft *trackerManagerDraft, addresses []string) int {
	count := 0
	for _, address := range addresses {
		if !slices.Contains(draft.disabled, address) {
			count++
		}
	}
	return count
}

func trackerSearch(c *ui.Context, browser *trackerBrowser, label, placeholder string) {
	ui.SearchField(c, &browser.query).Label(label).Placeholder(placeholder).MinHeight(inputHeight(c)).FillWidth().
		Radius(radiusSmall).Background(colors(c).surface).
		OnChange(func() { browser.page = 0 })
}

func trackerPagination(c *ui.Context, browser *trackerBrowser, pages int, count, label string) {
	if pages <= 1 {
		return
	}
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, fmt.Sprintf("第 %d / %d 页 · %s", browser.page+1, pages, count)).
			FontSize(textSize(c, textMeta)).TextColor(colors(c).muted).Grow(1)
		compactSecondary(c, "previous", "上一页", func() { browser.page-- }).Label(label + "上一页").Disabled(browser.page == 0)
		compactSecondary(c, "next", "下一页", func() { browser.page++ }).Label(label + "下一页").Disabled(browser.page+1 == pages)
	})
}

func (s *State) trackerListDetails(c *ui.Context, draft *trackerManagerDraft) {
	key := draft.selectedList
	index := slices.IndexFunc(draft.lists, func(list core.TrackerList) bool { return list.URL == key })
	var title string
	var list core.TrackerList
	switch {
	case key == automaticTrackerList:
		title = "Dipper 推荐清单"
		list = core.TrackerList{URL: trackers.FallbackURL, Trackers: s.trackerAutomatic.URLs, FetchedAt: s.trackerAutomatic.FetchedAt, Disabled: draft.automaticOff}
	case index >= 0:
		list = draft.lists[index]
		title, _ = trackerListName(list.URL)
	default:
		return
	}
	ui.Column(c).Gap(12).Children(func() {
		ui.Row(c).Wrap().Gap(12).Children(func() {
			compactSecondary(c, "back-to-lists", "返回下载来源", func() { draft.selectList("") }).Disabled(draft.reading)
		})
		pageTitle(c, title)
		if list.URL != "" {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				trackerAddress(c, list.URL)
				compactSecondary(c, "copy-list-url", "复制链接", func() { s.CopyText(list.URL) }).Disabled(s.CopyText == nil)
			})
			caption(c, trackerListUpdated(list.FetchedAt))
			ui.Row(c).Wrap().Gap(8).Children(func() {
				if index >= 0 {
					compactSecondary(c, "refresh-list", "立即更新", func() { s.readTrackerList(draft, index) }).Disabled(draft.reading)
					if list.Disabled {
						compactSecondary(c, "add-back-list", "添加清单", func() { draft.lists[index].Disabled = false })
					}
					compactSecondary(c, "remove-list", "删除订阅", func() {
						draft.lists = slices.Delete(draft.lists, index, index+1)
						draft.selectList("")
					}).Disabled(draft.reading)
				}
				if key == automaticTrackerList {
					label := "删除推荐清单"
					if list.Disabled {
						label = "添加回来"
					}
					compactSecondary(c, "change-automatic-list", label, func() { draft.automaticOff = !draft.automaticOff })
				}
			})
		}
		if list.Disabled {
			caption(c, "这份清单已删除。")
		}
		if len(list.Trackers) == 0 {
			caption(c, "还没有获取到地址。")
			return
		}
		removedCount := len(list.Trackers) - trackerAddressCount(draft, list.Trackers)
		if removedCount > 0 || draft.showRemoved {
			ui.Row(c).Children(func() {
				label := fmt.Sprintf("已删除的地址（%d）", removedCount)
				if draft.showRemoved {
					label = "查看现有地址"
				}
				compactSecondary(c, "show-removed", label, func() {
					draft.showRemoved = !draft.showRemoved
					draft.addressBrowser = trackerBrowser{}
				})
			})
		}
		var addresses []string
		for _, address := range list.Trackers {
			if slices.Contains(draft.disabled, address) == draft.showRemoved {
				addresses = append(addresses, address)
			}
		}
		trackerAddressPage(c.Key("list-addresses"), &draft.addressBrowser, addresses, func(c *ui.Context, address string) {
			label := "删除"
			if draft.showRemoved {
				label = "添加回来"
			}
			compactSecondary(c, "change-address", label, func() { draft.setAddressRemoved(address, !draft.showRemoved) }).Label(label + " " + address)
		})
	})
}

func trackerListUpdated(fetchedAt time.Time) string {
	if fetchedAt.IsZero() {
		return "尚未更新"
	}
	return "上次更新 " + fetchedAt.Local().Format("2006-01-02 15:04")
}

func (s *State) trackerSubscriptionEditor(c *ui.Context, draft *trackerManagerDraft) {
	ui.Field(c.Key("tracker-list-field"), "在线清单链接", func() {
		ui.TextInput(c.Key("tracker-list-input"), &draft.input).
			Placeholder("https://lists.example.net/trackers.txt").FillWidth().MinHeight(inputHeight(c)).
			Radius(radiusSmall).Background(colors(c).surface).
			Disabled(draft.reading).OnChange(func() {
			draft.preview = core.TrackerList{}
			draft.report("", false)
		}).OnSubmit(func() { s.readTrackerList(draft, -1) })
	})
	if draft.preview.URL != "" {
		current := core.Settings{ExtraTrackers: strings.Join(draft.manual, "\n"), TrackerLists: draft.lists}
		existing := append(current.ActiveExtraTrackers(), activeAutomaticTrackers(s.trackerAutomatic, draft.automaticOff)...)
		known := make(map[string]bool, len(existing))
		for _, address := range existing {
			known[address] = true
		}
		duplicates := 0
		for _, address := range draft.preview.Trackers {
			if known[address] && !slices.Contains(draft.disabled, address) {
				duplicates++
			}
		}
		caption(c, fmt.Sprintf("读到 %d 个有效地址，其中 %d 个已配置。", len(draft.preview.Trackers), duplicates))
	}
	ui.Row(c).Wrap().Gap(12).Children(func() {
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
		trackerAddressPage(c.Key("preview-addresses"), &draft.previewBrowser, draft.preview.Trackers, nil)
	}
}

func trackerAddressPage(c *ui.Context, browser *trackerBrowser, addresses []string, action func(*ui.Context, string)) {
	const pageSize = 8
	if len(addresses) > pageSize || browser.query != "" {
		trackerSearch(c.Key("search"), browser, "搜索地址", "按域名或地址搜索")
	}
	var matching []string
	for _, address := range addresses {
		if browser.matches(address) {
			matching = append(matching, address)
		}
	}
	start, end, pages := browser.bounds(len(matching), pageSize)
	for _, address := range matching[start:end] {
		row := c.Key(address)
		ui.Row(row).AlignItems(ui.Center).Gap(12).Padding(4, 0).Children(func() {
			trackerAddress(row, address)
			if action != nil {
				action(row, address)
			}
		})
	}
	if len(matching) == 0 {
		label := "这里还没有地址。"
		if browser.query != "" {
			label = "没有找到匹配的地址。"
		}
		caption(c, label)
	}
	trackerPagination(c.Key("pages"), browser, pages, fmt.Sprintf("%d 个地址", len(matching)), "地址")
}
