package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/trackers"
)

type trackerManagerDraft struct {
	manual          []string
	lists           []core.TrackerList
	editing         string
	input, message  string
	editIndex       int
	failed, reading bool
	preview         core.TrackerList
	selectedList    string
	listBrowser     trackerBrowser
	addressBrowser  trackerBrowser
	previewBrowser  trackerBrowser
	automaticOff    bool
	showRemoved     bool
	disabled        []string
}

const (
	automaticTrackerList = "automatic"
)

type trackerBrowser struct {
	query string
	page  int
}

func (s *State) openTrackerManager() {
	s.trackerManager = &trackerManagerDraft{
		manual:       core.ParseTrackers(s.ExtraTrackers),
		lists:        slices.Clone(s.TrackerLists),
		disabled:     slices.Clone(s.DisabledTrackers),
		automaticOff: s.AutomaticTrackersDisabled,
	}
	s.trackerManagerOpen = true
}

func (draft *trackerManagerDraft) selectList(key string) {
	draft.selectedList = key
	draft.addressBrowser = trackerBrowser{}
	draft.showRemoved = false
	draft.report("", false)
}

func (browser *trackerBrowser) matches(text string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(strings.TrimSpace(browser.query)))
}

func (browser *trackerBrowser) bounds(count, size int) (start, end, pages int) {
	pages = max(1, (count+size-1)/size)
	browser.page = min(browser.page, pages-1)
	start = browser.page * size
	return start, min(count, start+size), pages
}

func (draft *trackerManagerDraft) setAddressRemoved(address string, removed bool) {
	if index := slices.Index(draft.disabled, address); index >= 0 {
		if !removed {
			draft.disabled = slices.Delete(draft.disabled, index, index+1)
		}
	} else if removed {
		draft.disabled = append(draft.disabled, address)
	}
}

func (draft *trackerManagerDraft) edit(kind string, index int, input string) {
	draft.editing, draft.editIndex, draft.input = kind, index, input
	draft.message, draft.failed, draft.preview = "", false, core.TrackerList{}
	draft.previewBrowser = trackerBrowser{}
}

func (draft *trackerManagerDraft) report(message string, failed bool) {
	draft.message, draft.failed = message, failed
}

func (draft *trackerManagerDraft) addManual() {
	if err := core.ValidateTrackers(draft.input); err != nil {
		var invalid *core.TrackerError
		if errors.As(err, &invalid) {
			draft.report("无法识别这个下载来源地址 "+strconv.Quote(invalid.Line)+"。", true)
		}
		return
	}
	urls := core.ParseTrackers(draft.input)
	if len(urls) == 0 || (draft.editIndex >= 0 && len(urls) != 1) {
		draft.report("请填写下载来源地址。编辑时一次只能填写一个地址。", true)
		return
	}
	combined := draft.manual
	if draft.editIndex >= 0 {
		combined = slices.Replace(combined, draft.editIndex, draft.editIndex+1, urls...)
	} else {
		combined = append(combined, urls...)
	}
	draft.manual = core.ParseTrackers(strings.Join(combined, "\n"))
	for _, address := range urls {
		draft.setAddressRemoved(address, false)
	}
	draft.edit("", -1, "")
}

func (s *State) fetchTrackerList(ctx context.Context, address string) (core.TrackerList, error) {
	fetch := s.trackerFetcher
	if fetch == nil {
		fetch = trackers.FetchHTTP
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return trackers.ReadSubscription(ctx, address, fetch)
}

func (s *State) readTrackerList(draft *trackerManagerDraft, index int) {
	if draft.reading || s.Update == nil {
		return
	}
	address := strings.TrimSpace(draft.input)
	if index >= 0 {
		address = draft.lists[index].URL
	} else {
		draft.preview = core.TrackerList{}
		draft.previewBrowser = trackerBrowser{}
		if !core.ValidTrackerListURL(address) {
			draft.report("请输入以 http:// 或 https:// 开头的在线列表链接。", true)
			return
		}
		if slices.ContainsFunc(draft.lists, func(list core.TrackerList) bool { return list.URL == address }) {
			draft.report("已经订阅了这个列表。", true)
			return
		}
		if len(draft.lists) >= core.MaxTrackerLists {
			draft.report(fmt.Sprintf("最多可订阅 %d 个列表。", core.MaxTrackerLists), true)
			return
		}
	}
	draft.reading = true
	draft.report("正在读取在线列表…", false)
	go func() {
		list, err := s.fetchTrackerList(s.Ctx, address)
		s.Update(func() {
			if !s.trackerManagerOpen || s.trackerManager != draft {
				return
			}
			draft.reading = false
			if err != nil {
				message := "无法读取列表。请检查链接和网络，确认它是每行一个地址的纯文本文件。"
				if index >= 0 {
					message = "更新失败，已保留上次读取的地址。"
				}
				draft.report(message, true)
				return
			}
			if index >= 0 {
				list.Disabled = draft.lists[index].Disabled
				draft.lists[index] = list
				draft.report("列表已更新，保存修改后生效。", false)
				return
			}
			draft.preview = list
			draft.report("", false)
		})
	}()
}

func (s *State) saveTrackerSettings(manual string, lists []core.TrackerList, disabled []string, automaticOff bool) error {
	settings, err := s.Core.GetSettings()
	if err != nil {
		return err
	}
	settings.ExtraTrackers, settings.TrackerLists = manual, lists
	settings.DisabledTrackers = disabled
	settings.AutomaticTrackersDisabled = automaticOff
	if err := s.Core.SetSettings(settings); err != nil {
		return err
	}
	s.ExtraTrackers, s.TrackerLists = manual, lists
	s.DisabledTrackers = slices.Clone(disabled)
	if s.Engine != nil {
		s.Engine.SetTrackerSettings(settings.ActiveExtraTrackers(), disabled)
	}
	if s.AutomaticTrackersDisabled != automaticOff {
		s.AutomaticTrackersDisabled = automaticOff
		s.applyAutomaticTrackerList()
	}
	return nil
}

func (s *State) saveTrackerManager(draft *trackerManagerDraft) {
	if s.Core == nil || s.Busy || draft.reading || draft.editing != "" {
		return
	}
	restoreAutomatic := s.AutomaticTrackersDisabled && !draft.automaticOff
	if err := s.saveTrackerSettings(strings.Join(draft.manual, "\n"), draft.lists, draft.disabled, draft.automaticOff); err != nil {
		draft.report("无法保存下载来源设置，请重试。", true)
		return
	}
	s.trackerManagerOpen, s.trackerManager = false, nil
	if restoreAutomatic {
		s.refreshAutomaticTrackerList()
	}
}

func activeAutomaticTrackers(list trackers.List, disabled bool) []string {
	if disabled {
		return nil
	}
	return list.URLs
}

func (s *State) applyAutomaticTrackerList() {
	if s.Engine != nil {
		s.Engine.SetRemoteTrackers(activeAutomaticTrackers(s.trackerAutomatic, s.AutomaticTrackersDisabled))
	}
}

func (s *State) refreshAutomaticTrackerList() {
	if s.AutomaticTrackersDisabled || s.automaticTrackerRefreshRunning || s.Update == nil ||
		s.Ctx == nil || !s.trackerAutomatic.Stale(time.Now()) {
		return
	}
	path := filepath.Join(s.Dir, "trackers.json")
	fetch := s.trackerFetcher
	if fetch == nil {
		fetch = trackers.FetchHTTP
	}
	s.automaticTrackerRefreshRunning = true
	go func() {
		ctx, cancel := context.WithTimeout(s.Ctx, 30*time.Second)
		defer cancel()
		list, err := trackers.Refresh(ctx, path, time.Now(), fetch)
		if err != nil {
			slog.Warn("tracker list refresh failed", "error", err)
		}
		s.Update(func() {
			s.automaticTrackerRefreshRunning = false
			s.trackerAutomatic = list
			s.applyAutomaticTrackerList()
		})
	}()
}

func (s *State) refreshTrackerSubscriptions() {
	if s.trackerRefreshRunning || s.trackerManagerOpen || s.Core == nil || s.Update == nil {
		return
	}
	var stale []core.TrackerList
	now := time.Now()
	for _, list := range s.TrackerLists {
		if !list.Disabled && now.Sub(list.FetchedAt) >= trackers.RefreshAfter {
			stale = append(stale, list)
		}
	}
	if len(stale) == 0 {
		return
	}
	s.trackerRefreshRunning = true
	go func() {
		fresh := make(map[string]core.TrackerList)
		for _, list := range stale {
			if s.Ctx.Err() != nil {
				break
			}
			if updated, err := s.fetchTrackerList(s.Ctx, list.URL); err == nil {
				fresh[list.URL] = updated
			}
		}
		s.Update(func() {
			s.trackerRefreshRunning = false
			if s.trackerManagerOpen || len(fresh) == 0 {
				return
			}
			lists := slices.Clone(s.TrackerLists)
			changed := false
			for index, list := range lists {
				for _, previous := range stale {
					if updated, ok := fresh[list.URL]; ok && !list.Disabled && previous.URL == list.URL && previous.FetchedAt.Equal(list.FetchedAt) {
						lists[index], changed = updated, true
						break
					}
				}
			}
			if changed {
				if err := s.saveTrackerSettings(s.ExtraTrackers, lists, s.DisabledTrackers, s.AutomaticTrackersDisabled); err != nil {
					slog.Warn("save refreshed tracker subscriptions", "error", err)
				}
			}
		})
	}()
}
