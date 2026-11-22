package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
	"github.com/steampixel007/Dipper/internal/proxy"
	"github.com/steampixel007/Dipper/internal/source"
)

type engineAdapter struct{ engine *downloader.Engine }

func (a engineAdapter) Ensure(t core.DownloadTask) error {
	return a.engine.Ensure(downloader.Task{ID: t.Hash, Magnet: t.Resource.Magnet, TorrentURL: t.Resource.TorrentURL, Folder: t.Directory, Shared: t.SharedDirectory, Context: t.Context})
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
	return core.Rule{All: slices.Clone(r.All), None: slices.Clone(r.None), Any: r.Any, Version: r.Version}
}
func uiRule(r core.Rule) Rule {
	return cloneRule(Rule{All: r.All, None: r.None, Any: r.Any, Version: r.Version})
}

type preferences struct {
	IntroHidden   bool
	ReduceEffects bool
	Appearance    string
	CloseBehavior string
	// SourceOrigin persists the ASCII origin, not the Chinese label, so
	// rewording the route label does not reset the user's choice.
	SourceOrigin         string
	SoftwareAuto         *bool
	SoftwareAutoDownload bool
	SoftwarePending      *softwarePending
	SoftwareCheckedAt    time.Time
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
	case "不限":
		return -1
	case "256 KiB/s", "256 KB/s":
		return 256 * 1024
	case "512 KiB/s", "512 KB/s":
		return 512 * 1024
	case "1 MiB/s", "1 MB/s":
		return 1024 * 1024
	case "2 MiB/s", "2 MB/s":
		return 2 * 1024 * 1024
	}
	if rate, ok := parseByteRate(label); ok {
		return rate
	}
	return 1024 * 1024
}

func parseByteRate(label string) (int64, bool) {
	var value float64
	var unit string
	if _, err := fmt.Sscanf(strings.TrimSpace(label), "%f %s", &value, &unit); err != nil || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	multiplier := int64(0)
	switch unit {
	case "B/s":
		multiplier = 1
	case "KiB/s", "KB/s":
		multiplier = 1024
	case "MiB/s", "MB/s":
		multiplier = 1024 * 1024
	case "GiB/s", "GB/s":
		multiplier = 1024 * 1024 * 1024
	}
	if multiplier == 0 || value > float64(math.MaxInt64)/float64(multiplier) {
		return 0, false
	}
	rate := math.Round(value * float64(multiplier))
	if rate <= 0 || rate >= float64(math.MaxInt64) {
		return 0, false
	}
	return int64(rate), true
}

func uploadLabel(limit int64) string {
	if limit < 0 {
		return "不限"
	}
	if limit%(1024*1024) == 0 {
		return fmt.Sprintf("%d MiB/s", limit/(1024*1024))
	}
	if limit%1024 == 0 {
		return fmt.Sprintf("%d KiB/s", limit/1024)
	}
	return fmt.Sprintf("%d B/s", limit)
}

func seedLabel(minutes int64) string {
	switch minutes {
	case -1:
		return "不限时长"
	case 0:
		return "1 小时"
	}
	return fmt.Sprintf("%d 分钟", minutes)
}

func downloadLimitText(limit int) string {
	if limit == 0 {
		limit = downloader.DefaultDownloadLimit
	}
	return strconv.Itoa(limit)
}

func (s *State) saveDownloadLimit() {
	if s.Busy {
		return
	}
	limit, err := strconv.Atoi(strings.TrimSpace(s.downloadLimitDraft))
	if err != nil || limit < 1 || limit > core.MaxDownloadLimit {
		s.downloadLimitNotice = fmt.Sprintf("请输入 1–%d 之间的整数。", core.MaxDownloadLimit)
		s.downloadLimitError = true
		return
	}
	if s.Core != nil {
		settings, err := s.Core.GetSettings()
		if err == nil {
			settings.DownloadLimit = limit
			err = s.Core.SetSettings(settings)
		}
		if err != nil {
			s.downloadLimitNotice = "无法保存同时下载任务数，请重试。"
			s.downloadLimitError = true
			return
		}
	}
	s.DownloadLimit = limit
	s.downloadLimitDraft = downloadLimitText(limit)
	s.downloadLimitNotice = "同时下载任务数已保存。"
	s.downloadLimitError = false
	if s.Engine != nil {
		s.Engine.SetDownloadLimit(limit)
	}
}

func (s *State) loadSaved() error {
	state, err := s.Core.Snapshot()
	if err != nil {
		return err
	}
	s.Root = state.Settings.DownloadRoot
	s.DownloadLimit = state.Settings.DownloadLimit
	s.downloadLimitDraft = downloadLimitText(s.DownloadLimit)
	s.loadTransferLimits(state.Settings)
	s.ExtraTrackers = state.Settings.ExtraTrackers
	s.DisabledTrackers = state.Settings.DisabledTrackers
	s.AutomaticTrackersDisabled = state.Settings.AutomaticTrackersDisabled
	s.TrackerLists = state.Settings.TrackerLists
	s.ProxyMode = proxyModeIndex(state.Settings.ProxyMode)
	s.proxyActiveMode = s.ProxyMode
	s.ProxyURL, s.proxyUsername, s.proxyPassword = proxy.SplitAddress(state.Settings.ProxyURL)
	s.proxyActiveAddress = s.ProxyURL
	proxy.Configure(state.Settings.ProxyMode, state.Settings.ProxyURL)
	s.ProxyAuto = s.describeAutoProxy()
	if err = s.Engine.SetUploadLimit(state.Settings.EffectiveUploadLimit()); err != nil {
		return err
	}
	s.Engine.SetDownloadLimit(state.Settings.DownloadLimit)
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
	s.SoftwareAuto = true
	if pref.SoftwareAuto != nil {
		s.SoftwareAuto = *pref.SoftwareAuto
	}
	s.SoftwareAutoDownload = pref.SoftwareAutoDownload
	s.SoftwareCheckedAt = pref.SoftwareCheckedAt
	origin := pref.SourceOrigin
	if origin == "" {
		origin = source.DefaultOrigin
	}
	if err := s.Source.Retarget(origin); err != nil {
		return fmt.Errorf("应用线路设置：%w", err)
	}
	s.SourceRoute = s.sourceLabel(origin)
	s.SoftwarePending = pref.SoftwarePending
	return nil
}

func (s *State) save() {
	if s.Core == nil {
		return
	}
	settings, err := s.Core.GetSettings()
	if err != nil {
		s.SaveError = "无法读取设置：" + err.Error()
		return
	}
	settings.DownloadRoot = s.Root
	if err := s.Core.SetSettings(settings); err != nil {
		s.SaveError = "设置保存失败：" + err.Error()
		return
	}
	if err := s.savePreferences(); err != nil {
		s.SaveError = "界面设置保存失败：" + err.Error()
	} else {
		s.SaveError = ""
	}
}

func (s *State) savePreferences() error {
	automatic := s.SoftwareAuto
	b, _ := json.Marshal(preferences{IntroHidden: s.IntroHidden, ReduceEffects: s.ReduceEffects, Appearance: normalizeAppearance(s.Appearance), CloseBehavior: s.CloseBehavior, SourceOrigin: s.sourceOrigin(s.SourceRoute), SoftwareAuto: &automatic, SoftwareAutoDownload: s.SoftwareAutoDownload, SoftwarePending: s.SoftwarePending, SoftwareCheckedAt: s.SoftwareCheckedAt})
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

func (s *State) project(state core.State, snapshots []downloader.Snapshot) {
	s.resourceHashes = make(map[string]string)
	for _, sub := range state.Subscriptions {
		for id, item := range sub.Items {
			if item.Hash != "" {
				s.resourceHashes[id] = item.Hash
			}
		}
	}
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
		v := Subscription{ID: sub.ID, AnimeID: sub.Anime.ID, GroupID: sub.Group.ID, GroupName: sub.Group.Name, Group: -1, Rule: uiRule(sub.Rule), Folder: sub.Directory, Paused: sub.Paused, LastCheck: check, LastError: sub.LastError, CheckFailed: sub.LastError != "", NewTitles: map[string]bool{}}
		for key, item := range sub.Items {
			if item.Status == core.ItemSkipped {
				v.Skipped = append(v.Skipped, key)
			}
			if item.Read {
				v.Read = append(v.Read, key)
			}
			if item.Status == core.ItemPending {
				v.NewCount++
				v.NewTitles[normalize(item.Resource.Title)] = true
				if len(v.Recent) < 3 {
					v.Recent = append(v.Recent, item.Resource.Title)
				}
			}
		}
		if !sub.Paused {
			for _, d := range s.Downloads {
				if d.AnimeID == sub.Anime.ID {
					switch downloadState(d) {
					case downloadsDownloading:
						v.Active = true
					case downloadsQueued:
						v.Queued = true
					}
				}
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
		d := Download{ID: task.Hash, AnimeID: task.AnimeID, ResourceID: r.ID, Title: r.Title, Episode: core.ParseEpisode(r.Title), Folder: task.Directory, Bytes: r.Size, MB: legacySizeMB(r.Size), Error: record.LastError, Failed: record.LastError != ""}
		if snap, ok := byID[task.Hash]; ok {
			d.Progress = snap.Progress
			d.Queued = snap.Status == downloader.StatusQueued
			if d.Queued && d.Progress >= 1 {
				d.Progress = math.Nextafter(1, 0)
			}
			d.Paused = snap.Paused
			d.Stopped = snap.ManuallyStopped || snap.AutoStopped
			d.SeedSeconds = int(snap.SeedingSeconds)
			d.UploadedBytes = snap.UploadedBytes
			d.DownloadedBytes = snap.DownloadedBytes
			d.UploadedMB = float64(snap.UploadedBytes) / 1e6
			d.DownSpeed = float64(snap.DownloadSpeed) / 1e6
			d.UpSpeed = float64(snap.UploadSpeed) / 1e6
			if snap.TotalBytes > 0 {
				d.Bytes = snap.TotalBytes
				d.MB = legacySizeMB(snap.TotalBytes)
				d.CompletedBytes = snap.CompletedBytes
			} else if snap.TotalBytes < 0 {
				d.Bytes, d.MB = -1, 0
			}
			if snap.Error != "" {
				d.Error = snap.Error
				d.Failed = true
			}
		}
		s.Downloads = append(s.Downloads, d)
	}
	type followProgress struct{ known, done float64 }
	weights := map[string]*followProgress{}
	for _, d := range s.Downloads {
		w := weights[d.AnimeID]
		if w == nil {
			w = &followProgress{}
			weights[d.AnimeID] = w
		}
		if bytes := sizeBytes(d.Bytes, d.MB); bytes > 0 {
			w.known += float64(bytes)
			w.done += float64(downloadDoneBytes(d))
		}
	}
	completed := map[string]int{}
	for _, d := range s.Downloads {
		if d.Progress >= 1 && !d.Failed {
			completed[d.AnimeID]++
		}
	}
	for i := range s.Subscriptions {
		v := &s.Subscriptions[i]
		v.Completed = completed[v.AnimeID]
		w := weights[v.AnimeID]
		if w == nil {
			continue
		}
		if w.known > 0 {
			v.Progress = w.done / w.known
			continue
		}
		var count, sum float64
		for _, d := range s.Downloads {
			if d.AnimeID == v.AnimeID {
				count++
				sum += d.Progress
			}
		}
		if count > 0 {
			v.Progress = sum / count
		}
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
	if s.Core == nil || s.Busy || !s.canSaveDetailRule() {
		return
	}
	plan := s.planSubmission()
	if s.Backfill && len(plan.IDs) == 0 {
		return
	}
	s.Busy = true
	s.WordOpen, s.Suggestions = false, false
	s.SaveError = ""
	s.Notice = "正在提交，请稍候。"
	s.UndoSaved = nil
	editID := s.EditID
	backfill := s.Backfill
	rule := coreRule(s.Rule)
	a := s.anime(s.AnimeID)
	anime := core.Anime{ID: a.ID, Title: a.Title, Cover: a.Cover, Day: strconv.Itoa(a.Day)}
	group := core.Group{ID: s.GroupIDs[s.Group], Name: s.Groups[s.Group]}
	history := coreResources(s.History)
	ids := plan.IDs
	var previous *Subscription
	backfillPaused := false
	for _, sub := range s.Subscriptions {
		if sub.ID == editID {
			v := sub
			v.Rule = cloneRule(sub.Rule)
			previous = &v
			backfillPaused = sub.Paused
		}
	}
	go func() {
		var err error
		var deliveryErr error
		saved := false
		if backfill {
			saved, err = s.Core.DownloadExisting(s.Ctx, editID, history, ids)
			deliveryErr = err
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
				if backfill {
					s.SaveError = "下载提交失败：" + err.Error()
				}
				s.Notice = ""
				return
			}
			s.enterPage("following")
			if !backfill {
				s.UndoSaved = previous
			}
			s.Notice = "已保存追番，之后会自动检查更新。"
			if editID != "" {
				s.Notice = "已保存，只自动下载以后发布的资源。"
			}
			if backfill {
				if backfillPaused {
					s.Notice = fmt.Sprintf("已排队 %d 个下载任务，恢复追番后自动开始。", len(ids))
				} else {
					s.Notice = fmt.Sprintf("已开始下载 %d 个资源，追番条件没有改变。", len(ids))
					if deliveryErr != nil {
						s.Notice = fmt.Sprintf("已记录 %d 个下载任务，追番条件没有改变。", len(ids))
					}
				}
				s.enterPage("downloads")
				s.Backfill = false
			}
			if backfill && deliveryErr != nil {
				if backfillPaused {
					s.SaveError = fmt.Sprintf("下载已排队，恢复追番后开始：%v", deliveryErr)
				} else {
					s.SaveError = fmt.Sprintf("已记录 %d 个下载任务，部分还没开始，将自动重试：%v", len(ids), deliveryErr)
				}
			} else if err != nil {
				s.SaveError = "追番已保存，部分下载没有开始，将自动重试：" + err.Error()
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
	s.VersionSetup = false
	s.TitleSetup = false
	s.Rule.Version = nil
	s.OnlyMatches = false
	s.Checked = map[string]bool{}
	for _, r := range s.resources() {
		s.Checked[r.ID] = false
	}
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

func (s *State) pauseSubscription(id string) {
	if s.Core == nil {
		return
	}
	sub, ok := s.subscription(id)
	if !ok {
		return
	}
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
func (s *State) check(id string) {
	if s.Core == nil || s.Busy {
		return
	}
	if sub, ok := s.subscription(id); !ok || sub.Paused {
		return
	}
	s.Busy = true
	s.SaveError = ""
	s.Notice = "正在检查更新…"
	go func() {
		err := s.Core.Check(s.Ctx, id)
		s.refreshState()
		s.Update(func() {
			s.Busy = false
			s.Notice = ""
			if err != nil {
				s.SaveError = sourceLastErrorMessage(err.Error())
			} else {
				s.SaveError = ""
				s.Notice = "检查更新完成"
			}
		})
	}()
}

func (s *State) retryFailedChecks() {
	if s.Core == nil || s.Busy {
		return
	}
	pending := s.retryableChecks()
	if len(pending) == 0 {
		return
	}
	s.Busy = true
	s.SaveError = ""
	s.Notice = fmt.Sprintf("正在检查 %d 个追番的更新…", len(pending))
	go func() {
		failures := 0
		succeeded := 0
		unsavedError := ""
		for _, id := range pending {
			if err := s.Core.Check(s.Ctx, id); err != nil {
				failures++
				if unsavedError == "" {
					snapshot, snapshotErr := s.Core.Snapshot()
					saved := false
					if snapshotErr == nil {
						for _, sub := range snapshot.Subscriptions {
							if sub.ID == id && sub.LastError == err.Error() {
								saved = true
								break
							}
						}
					}
					if !saved {
						unsavedError = sourceLastErrorMessage(err.Error())
					}
				}
			} else {
				succeeded++
			}
		}
		s.refreshState()
		s.Update(func() {
			s.Busy = false
			switch {
			case failures == 0:
				s.SaveError = ""
				s.Notice = fmt.Sprintf("已检查 %d 个追番的更新。", len(pending))
			case succeeded == 0:
				s.Notice = ""
				s.SaveError = fmt.Sprintf("%d 个追番检查失败，请打开对应剧集库查看原因并重试。", failures)
			default:
				s.Notice = fmt.Sprintf("已检查 %d 个追番，%d 个仍需重试。", len(pending), failures)
				s.SaveError = fmt.Sprintf("%d 个追番检查失败，请打开对应剧集库查看原因并重试。", failures)
			}
			if unsavedError != "" {
				s.SaveError += "首次错误：" + unsavedError
			}
		})
	}()
}

func (s *State) runAction(action func() error, message string) {
	if s.Busy {
		return
	}
	s.Busy = true
	go func() {
		err := action()
		s.refreshState()
		s.Update(func() {
			s.Busy = false
			if err != nil {
				s.SaveError = err.Error()
			} else {
				s.SaveError = ""
				s.Notice = message
			}
		})
	}()
}
func (s *State) playDownload(index int) {
	if s.Engine == nil || index < 0 || index >= len(s.Downloads) {
		return
	}
	d := s.Downloads[index]
	s.runAction(func() error {
		files, err := s.Engine.PlayableFiles(d.ID)
		if err != nil {
			return err
		}
		for _, path := range files {
			if isPlayableMedia(path) {
				return s.PlayFile(path)
			}
		}
		return errors.New("未找到可播放的文件")
	}, "已打开播放器")
}
func (s *State) openFiles(index int) {
	if s.Engine == nil || index < 0 || index >= len(s.Downloads) {
		return
	}
	d := s.Downloads[index]
	s.FilesOpen, s.FilesID, s.FilesError = true, d.ID, ""
	s.Files, s.FilesLoading = nil, true
	s.filesRevision++
	revision := s.filesRevision
	go func() {
		files, err := s.Engine.DownloadFiles(d.ID)
		s.Update(func() {
			if revision != s.filesRevision {
				return
			}
			if err != nil {
				s.FilesError = err.Error()
			} else {
				s.Files = files
			}
			s.FilesLoading = false
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
	s.runAction(func() error { return s.Engine.Stop(id) }, "已停止分享")
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
	message := "已删除任务"
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
	var queued, ids []string
	for _, d := range s.Downloads {
		switch {
		case paused && downloadState(d) == downloadsQueued:
			queued = append(queued, d.ID)
		case paused && downloadState(d) == downloadsDownloading:
			ids = append(ids, d.ID)
		case !paused && downloadInQueue(d) && !d.Failed && (d.Paused || (d.Stopped && d.Progress < 1)):
			ids = append(ids, d.ID)
		}
	}
	s.runAction(func() error {
		for _, id := range append(queued, ids...) {
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
