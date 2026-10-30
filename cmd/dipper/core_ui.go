package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
	"dipper.local/dipper/internal/proxy"
	"dipper.local/dipper/internal/source"
)

type engineAdapter struct{ engine *downloader.Engine }

func (a engineAdapter) Ensure(t core.DownloadTask) error {
	return a.engine.Ensure(downloader.Task{ID: t.Hash, Magnet: t.Resource.Magnet, TorrentURL: t.Resource.TorrentURL, Folder: t.Directory, Context: t.Context})
}

// Remove reports a task the engine no longer holds as removed, so recovery of
// an interrupted removal finalizes instead of retrying forever.
func (a engineAdapter) Remove(hash string) error {
	err := a.engine.Remove(hash)
	if err == nil || errors.Is(err, downloader.ErrNotFound) || errors.Is(err, downloader.ErrRemoveCleanup) {
		return nil
	}
	return err
}

func coreResources(rows []source.Resource) []core.Resource {
	out := make([]core.Resource, 0, len(rows))
	for _, r := range rows {
		out = append(out, core.Resource{ID: r.ID, Title: r.Title, Published: r.Published, Magnet: r.Magnet, TorrentURL: r.TorrentURL, Size: r.Size})
	}
	return out
}

func coreRule(r Rule) core.Rule {
	return core.Rule{All: slices.Clone(r.All), None: slices.Clone(r.None), Any: r.Any}
}
func uiRule(r core.Rule) Rule { return cloneRule(Rule{All: r.All, None: r.None, Any: r.Any}) }

type preferences struct {
	IntroHidden   bool
	ReduceEffects bool
	Appearance    string
	CloseBehavior string
	// SourceOrigin persists the ASCII origin, not the Chinese label, so
	// rewording the route label does not reset the user's choice.
	SourceOrigin string
}

var proxyModes = []string{proxy.ModeAuto, proxy.ModeDirect, proxy.ModeCustom}

func proxyModeIndex(mode string) int {
	if mode == proxy.ModeDirect || mode == proxy.ModeCustom {
		return slices.Index(proxyModes, mode)
	}
	return 0
}

func uploadLimit(label string) int64 {
	switch label {
	case "256 KB/s":
		return 256 * 1024
	case "512 KB/s":
		return 512 * 1024
	case "2 MB/s":
		return 2 * 1024 * 1024
	}
	return 1024 * 1024
}
func uploadLabel(limit int64) string {
	switch limit {
	case 256 * 1024:
		return "256 KB/s"
	case 512 * 1024:
		return "512 KB/s"
	case 2 * 1024 * 1024:
		return "2 MB/s"
	}
	return "1 MB/s"
}

var seedOptions = []string{"30 分钟", "1 小时", "2 小时", "不限时长"}

func seedMinutes(label string) int64 {
	switch label {
	case "30 分钟":
		return 30
	case "2 小时":
		return 120
	case "不限时长":
		return -1
	}
	return 60
}

func seedLabel(minutes int64) string {
	switch minutes {
	case 30:
		return "30 分钟"
	case 120:
		return "2 小时"
	case -1:
		return "不限时长"
	}
	return "1 小时"
}

func seedDuration(minutes int64) time.Duration {
	if minutes == -1 {
		return -1
	}
	if minutes <= 0 {
		return 0
	}
	return time.Duration(minutes) * time.Minute
}

func (s *State) loadSaved() error {
	state, err := s.Core.Snapshot()
	if err != nil {
		return err
	}
	s.Root = state.Settings.DownloadRoot
	s.Upload = uploadLabel(state.Settings.UploadLimitBytes)
	s.Seed = seedLabel(state.Settings.SeedMinutes)
	s.ProxyMode = proxyModeIndex(state.Settings.ProxyMode)
	s.ProxyURL = state.Settings.ProxyURL
	proxy.Configure(state.Settings.ProxyMode, state.Settings.ProxyURL)
	s.ProxyAuto = s.describeAutoProxy()
	if err = s.Engine.SetUploadLimit(state.Settings.UploadLimitBytes); err != nil {
		return err
	}
	if err := s.loadPreferences(); err != nil {
		return err
	}
	s.project(state, s.Engine.Snapshot())
	return nil
}

func (s *State) loadPreferences() error {
	pref, err := readPreferences(s.ConfigDir)
	if err != nil {
		return err
	}
	s.IntroHidden = pref.IntroHidden
	s.ReduceEffects = pref.ReduceEffects
	s.Appearance = normalizeAppearance(pref.Appearance)
	s.CloseBehavior = normalizeCloseBehavior(pref.CloseBehavior)
	origin := pref.SourceOrigin
	if origin == "" {
		origin = source.DefaultOrigin
	}
	if err := s.Source.Retarget(origin); err != nil {
		return fmt.Errorf("应用线路设置：%w", err)
	}
	s.SourceRoute = s.sourceLabel(origin)
	return nil
}

func (s *State) save() {
	if s.Core == nil {
		return
	}
	mode := proxyModes[s.ProxyMode]
	if mode != proxy.ModeCustom {
		s.ProxyURL = ""
	}
	settings := core.Settings{DownloadRoot: s.Root, UploadLimitBytes: uploadLimit(s.Upload), SeedMinutes: seedMinutes(s.Seed), ProxyMode: mode, ProxyURL: s.ProxyURL}
	if err := s.Core.SetSettings(settings); err != nil {
		s.SaveError = "设置保存失败：" + err.Error()
		return
	}
	proxy.Configure(mode, settings.ProxyURL)
	if mode != proxy.ModeCustom {
		s.ProxyAuto = s.describeAutoProxy()
	}
	if err := s.Engine.SetUploadLimit(settings.UploadLimitBytes); err != nil {
		s.SaveError = "限速设置失败：" + err.Error()
		return
	}
	s.Engine.SetSeedTarget(seedDuration(settings.SeedMinutes))
	if err := s.savePreferences(); err != nil {
		s.SaveError = "界面设置保存失败：" + err.Error()
	} else {
		s.SaveError = ""
	}
}

func (s *State) savePreferences() error {
	b, _ := json.Marshal(preferences{IntroHidden: s.IntroHidden, ReduceEffects: s.ReduceEffects, Appearance: normalizeAppearance(s.Appearance), CloseBehavior: s.CloseBehavior, SourceOrigin: s.sourceOrigin(s.SourceRoute)})
	path := filepath.Join(s.ConfigDir, "preferences.json")
	err := os.WriteFile(path+".tmp", b, 0644)
	if err == nil {
		err = os.Rename(path+".tmp", path)
	}
	return err
}

func readPreferences(dir string) (preferences, error) {
	var pref preferences
	b, err := os.ReadFile(filepath.Join(dir, "preferences.json"))
	if os.IsNotExist(err) {
		return pref, nil
	}
	if err != nil {
		return pref, fmt.Errorf("读取界面设置：%w", err)
	}
	if err := json.Unmarshal(b, &pref); err != nil {
		return pref, fmt.Errorf("读取界面设置：%w", err)
	}
	return pref, nil
}

func (s *State) testProxy() {
	if s.ProxyTesting || s.Source == nil {
		return
	}
	s.save()
	if s.SaveError != "" {
		return
	}
	s.ProxyTesting = true
	s.ProxyFeedback = "正在测试连接…"
	go func() {
		ctx, cancel := context.WithTimeout(s.Ctx, 45*time.Second)
		defer cancel()
		err := s.Source.Probe(ctx)
		s.Update(func() {
			s.ProxyTesting = false
			if err != nil {
				s.ProxyFeedback = "连接失败：" + err.Error()
				return
			}
			s.ProxyFeedback = "连接成功。"
		})
	}()
}

func (s *State) project(state core.State, snapshots []downloader.Snapshot) {
	for _, record := range state.Anime {
		a := record.Anime
		day := -1
		if n, err := strconv.Atoi(a.Day); err == nil {
			day = n
		}
		s.KnownAnime[a.ID] = Anime{ID: a.ID, Title: a.Title, Cover: a.Cover, Day: day, Source: "蜜柑计划", Summary: s.KnownAnime[a.ID].Summary}
	}
	s.Subscriptions = nil
	for _, sub := range state.Subscriptions {
		check := "尚未检查"
		if !sub.LastCheck.IsZero() {
			check = sub.LastCheck.Local().Format("01-02 15:04:05")
		}
		if sub.LastError != "" {
			check += " · " + sub.LastError
		}
		v := Subscription{ID: sub.ID, AnimeID: sub.Anime.ID, GroupID: sub.Group.ID, GroupName: sub.Group.Name, Group: -1, Rule: uiRule(sub.Rule), Folder: sub.Directory, Paused: sub.Paused, LastCheck: check, CheckFailed: sub.LastError != ""}
		for key, item := range sub.Items {
			if item.Status == core.ItemSkipped {
				v.Skipped = append(v.Skipped, key)
			}
			if item.Read {
				v.Read = append(v.Read, key)
			}
		}
		s.Subscriptions = append(s.Subscriptions, v)
	}
	byID := map[string]downloader.Snapshot{}
	for _, snap := range snapshots {
		byID[snap.ID] = snap
	}
	s.Downloads = nil
	for _, record := range state.Tasks {
		task := record.Task
		r := task.Resource
		d := Download{ID: task.Hash, AnimeID: task.AnimeID, Title: r.Title, Episode: core.ParseEpisode(r.Title), Folder: task.Directory, MB: int((r.Size + 999999) / 1000000), Error: record.LastError, Failed: record.LastError != ""}
		if snap, ok := byID[task.Hash]; ok {
			d.Progress = snap.Progress
			d.Paused = snap.Paused
			d.Stopped = snap.ManuallyStopped || snap.AutoStopped
			d.SeedSeconds = int(snap.SeedingSeconds)
			d.UploadedMB = float64(snap.UploadedBytes) / 1e6
			d.DownSpeed = float64(snap.DownloadSpeed) / 1e6
			d.UpSpeed = float64(snap.UploadSpeed) / 1e6
			if snap.TotalBytes > 0 {
				d.MB = int((snap.TotalBytes + 999999) / 1000000)
			}
			if snap.Error != "" {
				d.Error = snap.Error
				d.Failed = true
			}
		}
		s.Downloads = append(s.Downloads, d)
	}
}

func (s *State) refreshState() {
	if s.Core == nil {
		return
	}
	state, err := s.Core.Snapshot()
	if err != nil {
		s.Update(func() { s.SaveError = err.Error() })
		return
	}
	snaps := s.Engine.Snapshot()
	s.Update(func() { s.project(state, snaps) })
}

func (s *State) targetFolder() string {
	if s.ProposedFolder != "" {
		return s.ProposedFolder
	}
	for _, sub := range s.Subscriptions {
		if sub.AnimeID == s.AnimeID {
			return sub.Folder
		}
	}
	return filepath.Join(s.Root, safeName(s.anime(s.AnimeID).Title))
}

func (s *State) commit() {
	if s.Core == nil || s.Busy || s.Group < 0 || s.Group >= len(s.GroupIDs) || s.DetailLoading || s.DetailError != "" || s.HistoryLoading || s.HistoryError != "" {
		return
	}
	s.Busy = true
	s.UndoSaved = nil
	editID := s.EditID
	backfill := s.Backfill
	rule := coreRule(s.Rule)
	a := s.anime(s.AnimeID)
	anime := core.Anime{ID: a.ID, Title: a.Title, Cover: a.Cover, Day: strconv.Itoa(a.Day)}
	group := core.Group{ID: s.GroupIDs[s.Group], Name: s.Groups[s.Group]}
	history := coreResources(s.History)
	selected, _ := s.selected()
	var ids []string
	for _, r := range selected {
		ids = append(ids, r.ID)
	}
	var previous *Subscription
	for _, sub := range s.Subscriptions {
		if sub.ID == editID {
			v := sub
			v.Rule = cloneRule(sub.Rule)
			previous = &v
		}
	}
	go func() {
		var err error
		saved := false
		if backfill {
			err = s.Core.DownloadExisting(s.Ctx, editID, history, ids)
			saved = err == nil
		} else if editID != "" {
			err = s.Core.UpdateRule(s.Ctx, editID, rule)
			saved = err == nil
		} else {
			var sub core.Subscription
			sub, err = s.Core.CreateSubscription(s.Ctx, anime, group, rule, history, ids)
			saved = sub.ID != ""
		}
		s.refreshState()
		s.Update(func() {
			s.Busy = false
			if err != nil && !saved {
				s.SaveError = "追番保存失败：" + err.Error()
				return
			}
			s.Confirm = false
			s.Page = "following"
			if !backfill {
				s.UndoSaved = previous
			}
			s.Notice = "已保存追番，后续每 15 分钟自动检查更新。"
			if editID != "" {
				s.Notice = "已保存条件，仅对未来未读资源生效。"
			}
			if backfill {
				s.Notice = "已提交选中的历史资源，追番条件保持原样。"
				s.Page = "downloads"
				s.Backfill = false
			}
			if err != nil {
				s.SaveError = "追番已保存，部分下载提交失败，将自动重试：" + err.Error()
			} else {
				s.SaveError = ""
			}
		})
	}()
}

func (s *State) beginBackfill() {
	if s.EditID == "" {
		return
	}
	s.Backfill = true
	s.Editing = false
	s.Checked = map[string]bool{}
	for _, r := range s.resources() {
		s.Checked[r.ID] = false
	}
	s.Notice = "勾选需要补下的已有资源。本次勾选不修改保存的追番条件。"
}

func (s *State) undoSave() {
	if s.Core == nil || s.UndoSaved == nil {
		return
	}
	sub := *s.UndoSaved
	go func() {
		err := s.Core.UpdateRule(s.Ctx, sub.ID, coreRule(sub.Rule))
		s.refreshState()
		s.Update(func() {
			if err != nil {
				s.SaveError = err.Error()
				return
			}
			s.UndoSaved = nil
			s.Notice = "已恢复上次保存的追番条件。"
		})
	}()
}

func (s *State) pauseSubscription(index int) {
	if s.Core == nil || index < 0 || index >= len(s.Subscriptions) {
		return
	}
	sub := s.Subscriptions[index]
	s.runAction(func() error { return s.Core.SetPaused(s.Ctx, sub.ID, !sub.Paused) }, "追番状态已更新")
}

func (s *State) removeSubscription(id string) {
	if s.Core == nil {
		return
	}
	title := ""
	if sub, ok := s.subscription(id); ok {
		title = s.anime(sub.AnimeID).Title
	}
	s.runAction(func() error { return s.Core.RemoveSubscription(s.Ctx, id) }, "已取消追番《"+title+"》")
}
func (s *State) check(index int) {
	if s.Core == nil || index < 0 || index >= len(s.Subscriptions) || s.Subscriptions[index].Paused {
		return
	}
	id := s.Subscriptions[index].ID
	s.Notice = "正在检查 RSS…"
	s.runAction(func() error { return s.Core.Check(s.Ctx, id) }, "RSS 检查完成")
}
func (s *State) runAction(action func() error, message string) {
	go func() {
		err := action()
		s.refreshState()
		s.Update(func() {
			if err != nil {
				s.SaveError = err.Error()
			} else {
				s.SaveError = ""
				s.Notice = message
			}
		})
	}()
}
func (s *State) pauseDownload(index int) {
	if s.Engine == nil || index < 0 || index >= len(s.Downloads) {
		return
	}
	d := s.Downloads[index]
	s.runAction(func() error {
		if d.Paused || d.Stopped {
			return s.Engine.Resume(d.ID)
		}
		return s.Engine.Pause(d.ID)
	}, "下载状态已更新")
}
func (s *State) retryDownload(index int) {
	if s.Engine == nil || index < 0 || index >= len(s.Downloads) {
		return
	}
	id := s.Downloads[index].ID
	s.runAction(func() error {
		err := s.Engine.Resume(id)
		if err == downloader.ErrNotFound {
			return s.Core.RetryPending(s.Ctx)
		}
		return err
	}, "已重试下载")
}
func (s *State) stopDownload(index int) {
	if s.Engine == nil || index < 0 || index >= len(s.Downloads) {
		return
	}
	id := s.Downloads[index].ID
	s.runAction(func() error { return s.Engine.Stop(id) }, "已停止该资源的下载和做种")
}

// removeDownload drops a task for good: the engine deletes the transfer and
// its files, and core forgets the delivery so neither the scheduler nor a
// restart brings it back. A task the engine never admitted (a submission that
// kept failing) has nothing on disk to remove, so its ErrNotFound is fine.
func (s *State) removeDownload(id string, redownload bool) {
	if s.Core == nil || s.Engine == nil {
		return
	}
	index := slices.IndexFunc(s.Downloads, func(d Download) bool { return d.ID == id })
	if index < 0 {
		return
	}
	message := "已删除该下载任务"
	if redownload {
		message = "已删除任务，将重新下载"
	}
	s.runAction(func() error {
		return s.Core.RemoveTask(s.Ctx, id, redownload, func() (bool, error) {
			err := s.Engine.Remove(id)
			if errors.Is(err, downloader.ErrNotFound) {
				return true, nil
			}
			return err == nil || errors.Is(err, downloader.ErrRemoveCleanup), err
		})
	}, message)
}
func (s *State) pauseAllDownloads(paused bool) {
	if s.Engine == nil {
		return
	}
	var ids []string
	for _, d := range s.Downloads {
		if !d.Failed && ((paused && d.Progress < 1) || (!paused && d.Paused)) {
			ids = append(ids, d.ID)
		}
	}
	s.runAction(func() error {
		for _, id := range ids {
			var err error
			if paused {
				err = s.Engine.Pause(id)
			} else {
				err = s.Engine.Resume(id)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}, "下载状态已更新")
}
func (s *State) downloadSpeed() float64 {
	var n float64
	for _, d := range s.Downloads {
		n += d.DownSpeed
	}
	return n
}
func (s *State) uploadSpeed() float64 {
	var n float64
	for _, d := range s.Downloads {
		n += d.UpSpeed
	}
	return n
}
