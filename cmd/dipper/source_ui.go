package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/source"
)

type sourceRoute struct {
	Label  string
	Origin string
}

var defaultSourceRoutes = []sourceRoute{{Label: "线路一（默认）：mikanime.tv", Origin: "https://mikanime.tv"}, {Label: "线路二：mikanani.me", Origin: "https://mikanani.me"}}

func (s *State) routeLabels() []string {
	labels := make([]string, len(s.SourceRoutes))
	for i, r := range s.SourceRoutes {
		labels[i] = r.Label
	}
	return labels
}

func (s *State) sourceOrigin(label string) string {
	for _, r := range s.SourceRoutes {
		if label == r.Label {
			return r.Origin
		}
	}
	return source.DefaultOrigin
}

func (s *State) sourceLabel(origin string) string {
	for _, r := range s.SourceRoutes {
		if origin == r.Origin {
			return r.Label
		}
	}
	return s.SourceRoutes[0].Label
}

func newState(dir string) *State {
	s := &State{Dir: dir, ConfigDir: dir, Platform: runtime.GOOS, Appearance: appearanceLight, CloseBehavior: closeAsk, Page: "browse", Day: -1, Group: -1, Checked: map[string]bool{}, KnownAnime: map[string]Anime{}, Ctx: context.Background(), SourceRoutes: defaultSourceRoutes}
	s.Seed = "1 小时"
	s.SeedRatio = "1:1"
	if dir != "" {
		s.CoverDir = filepath.Join(dir, "covers")
	}
	s.Update = func(fn func()) { fn() }
	s.setCurrentSeason()
	s.Root, s.SaveError = initialDownloadRoot()
	s.Upload = "1 MiB/s"
	s.SourceRoute = defaultSourceRoutes[0].Label
	return s
}

func initialDownloadRoot() (string, string) {
	root, err := core.DefaultDownloadRoot()
	if err != nil {
		return "", "无法读取用户视频目录：" + err.Error()
	}
	return root, ""
}

func (s *State) setCurrentSeason() {
	now := time.Now()
	s.Year = strconv.Itoa(now.Year())
	s.Season = seasons[(int(now.Month())-1)/3]
}

func (s *State) currentSeason() {
	s.setCurrentSeason()
	s.Day = -1
	s.Search = ""
	s.CatalogScroll = ui.ScrollState{}
	s.refreshCatalog()
}

func fromAnime(a source.Anime) Anime {
	day := -1
	if n, err := strconv.Atoi(a.Day); err == nil {
		day = n
	}
	return Anime{ID: a.ID, Title: a.Title, Cover: a.Cover, Day: day, Source: "蜜柑计划", Summary: a.Summary}
}

func (s *State) refreshCatalog() {
	if s.Source == nil {
		return
	}
	var following []source.Anime
	for _, sub := range s.Subscriptions {
		a := s.anime(sub.AnimeID)
		following = append(following, source.Anime{ID: a.ID, Cover: a.Cover})
	}
	// Saved covers must survive a season change or a failed catalog request.
	s.loadCovers(s.Ctx, following)
	if s.catalogCancel != nil {
		s.catalogCancel()
	}
	ctx, cancel := context.WithCancel(s.Ctx)
	s.catalogCancel = cancel
	s.catalogRevision++
	rev := s.catalogRevision
	year, _ := strconv.Atoi(s.Year)
	season := s.Season
	s.SourceLoading = true
	s.SourceError = ""
	s.Catalog = nil
	go func() {
		rows, err := s.Source.Catalog(ctx, year, season)
		s.Update(func() {
			if rev != s.catalogRevision {
				return
			}
			s.SourceLoading = false
			if err != nil {
				s.SourceError = sourceErrorMessage(err)
				return
			}
			for _, a := range rows {
				v := fromAnime(a)
				v.Summary = s.KnownAnime[a.ID].Summary
				s.Catalog = append(s.Catalog, v)
				s.KnownAnime[a.ID] = v
			}
			s.loadCovers(ctx, rows)
		})
	}()
}

type sourceFailureKind int

const (
	sourceFailureUnknown sourceFailureKind = iota
	sourceFailureTimeout
	sourceFailureConnection
	sourceFailureChallenge
	sourceFailureStructure
	sourceFailureNotFound
	sourceFailureTooLarge
	sourceFailureIncomplete
	sourceFailureRateLimited
	sourceFailureStatus
)

type sourceFailure struct {
	kind   sourceFailureKind
	status int
}

var sourceStatusPattern = regexp.MustCompile(`(?i)(?:http(?: status)?|status(?: code)?)\s*[:=]?\s*(\d{3})`)

func classifySourceErrorText(raw string) sourceFailure {
	text := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case text == "":
		return sourceFailure{}
	case strings.Contains(text, "deadline exceeded"), strings.Contains(text, "timeout"), strings.Contains(text, "timed out"), strings.Contains(text, "超时"):
		return sourceFailure{kind: sourceFailureTimeout}
	case strings.Contains(text, "access challenge"), strings.Contains(text, "人机验证"), strings.Contains(text, "验证码"):
		return sourceFailure{kind: sourceFailureChallenge}
	case strings.Contains(text, "response structure"), strings.Contains(text, "unrecognized"),
		strings.Contains(text, "inconsistent response"), strings.Contains(text, "unexpected page structure"),
		strings.Contains(text, "页面结构"), strings.Contains(text, "结构发生变化"):
		return sourceFailure{kind: sourceFailureStructure}
	case strings.Contains(text, "exceeds size limit"), strings.Contains(text, "response too large"),
		strings.Contains(text, "body too large"), strings.Contains(text, "内容过大"):
		return sourceFailure{kind: sourceFailureTooLarge}
	case strings.Contains(text, "cannot confirm complete history"), strings.Contains(text, "incomplete history"),
		strings.Contains(text, "未能完整"):
		return sourceFailure{kind: sourceFailureIncomplete}
	case strings.Contains(text, "not found"), strings.Contains(text, "没有找到"), strings.Contains(text, "未找到"):
		return sourceFailure{kind: sourceFailureNotFound}
	case strings.Contains(text, "connection refused"), strings.Contains(text, "connection reset"),
		strings.Contains(text, "could not connect"), strings.Contains(text, "no connection could be made"),
		strings.Contains(text, "no such host"), strings.Contains(text, "network is unreachable"),
		strings.Contains(text, "dial tcp"), strings.Contains(text, "connectex"),
		strings.Contains(text, "连接失败"), strings.Contains(text, "连接不上"), strings.Contains(text, "无法连接"):
		return sourceFailure{kind: sourceFailureConnection}
	case strings.HasPrefix(text, "mikan:") && (strings.HasSuffix(text, ": eof") || strings.HasSuffix(text, "unexpected eof")):
		return sourceFailure{kind: sourceFailureConnection}
	}
	if match := sourceStatusPattern.FindStringSubmatch(text); match != nil {
		status, _ := strconv.Atoi(match[1])
		return sourceFailure{kind: sourceFailureStatus, status: status}
	}
	return sourceFailure{}
}

func classifySourceError(err error) sourceFailure {
	var httpErr *source.HTTPError
	switch {
	case errors.Is(err, source.ErrChallenge):
		return sourceFailure{kind: sourceFailureChallenge}
	case errors.Is(err, source.ErrStructure):
		return sourceFailure{kind: sourceFailureStructure}
	case errors.Is(err, source.ErrNotFound):
		return sourceFailure{kind: sourceFailureNotFound}
	case errors.Is(err, source.ErrResponseTooLarge):
		return sourceFailure{kind: sourceFailureTooLarge}
	case errors.Is(err, source.ErrIncompleteHistory):
		return sourceFailure{kind: sourceFailureIncomplete}
	case errors.As(err, &httpErr):
		return sourceFailure{kind: sourceFailureStatus, status: httpErr.StatusCode}
	case errors.Is(err, context.DeadlineExceeded):
		return sourceFailure{kind: sourceFailureTimeout}
	case errors.Is(err, context.Canceled):
		return sourceFailure{kind: sourceFailureConnection}
	case err != nil:
		return classifySourceErrorText(err.Error())
	}
	return sourceFailure{}
}

func sourceFailureMessage(failure sourceFailure) string {
	switch failure.kind {
	case sourceFailureTimeout:
		return "网络连接超时，请稍后再试。"
	case sourceFailureConnection:
		return "网络连接失败，请检查网络后重试。"
	case sourceFailureChallenge:
		return "来源网站要求人机验证，请稍后再试。"
	case sourceFailureStructure:
		return "来源页面的结构发生变化，请稍后再试。"
	case sourceFailureNotFound:
		return "没有找到这部番剧或字幕组，请返回后重试。"
	case sourceFailureTooLarge:
		return "来源网站返回的内容过大，请稍后再试。"
	case sourceFailureIncomplete:
		return "资源还没加载完整，请稍后再试。"
	case sourceFailureRateLimited:
		return "请求有些频繁，请稍后再试。"
	case sourceFailureStatus:
		switch failure.status {
		case 404:
			return "没有找到这部番剧或字幕组，请返回后重试。"
		case 429:
			return "请求有些频繁，请稍后再试。"
		}
		if failure.status >= 500 {
			return fmt.Sprintf("来源网站暂时不可用（%d），请稍后再试。", failure.status)
		}
		return fmt.Sprintf("来源网站拒绝了请求（%d），请稍后再试。", failure.status)
	}
	return "暂时无法获取来源网站的信息，请稍后再试。"
}

func sourceRouteFailureMessage(failure sourceFailure) string {
	switch failure.kind {
	case sourceFailureTimeout:
		return "这条线路连接超时，换一条线路试试。"
	case sourceFailureConnection:
		return "这条线路连接不上，换一条线路试试。"
	case sourceFailureChallenge:
		return "这条线路需要人机验证，稍后再试或换一条线路。"
	case sourceFailureStructure:
		return "这条线路返回的页面无法识别，换一条线路试试。"
	case sourceFailureNotFound:
		return "这条线路没有找到所需内容，换一条线路试试。"
	case sourceFailureTooLarge:
		return "这条线路返回的内容过大，换一条线路试试。"
	case sourceFailureIncomplete:
		return "这条线路没能完整加载资源，换一条线路试试。"
	case sourceFailureRateLimited:
		return "这条线路请求有些频繁，换一条线路试试。"
	case sourceFailureStatus:
		if failure.status == 404 {
			return "这条线路没有找到所需内容，换一条线路试试。"
		}
		if failure.status == 429 {
			return "这条线路请求有些频繁，换一条线路试试。"
		}
		if failure.status >= 500 {
			return fmt.Sprintf("这条线路暂时不可用（%d），换一条线路试试。", failure.status)
		}
		return fmt.Sprintf("这条线路暂时无法响应（%d），换一条线路试试。", failure.status)
	}
	return "这条线路暂时连不上，换一条线路试试。"
}

func sourceErrorMessage(err error) string {
	return sourceFailureMessage(classifySourceError(err))
}

func sourceLastErrorMessage(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "检查更新失败："))
	if raw != "" && !strings.HasPrefix(raw, "mikan:") && raw != context.DeadlineExceeded.Error() && raw != "RSS 请求超时" {
		return raw
	}
	return sourceFailureMessage(classifySourceErrorText(raw))
}

func routeError(err error) string {
	if err == nil {
		return ""
	}
	return sourceRouteFailureMessage(classifySourceError(err))
}

func (s *State) switchSourceRoute(label string) {
	if s.Source == nil || s.SourceTesting || label == s.SourceRoute {
		return
	}
	previousLabel := s.SourceRoute
	previousOrigin := s.Source.Origin()
	targetOrigin := s.sourceOrigin(label)
	s.SourceRoute = label
	s.SourceTesting = true
	s.SourceRouteError = ""
	s.Notice = ""
	go func() {
		if err := s.Source.Retarget(targetOrigin); err != nil {
			s.Update(func() {
				s.SourceRoute = previousLabel
				s.SourceTesting = false
				s.SourceRouteError = sourceRouteFailureMessage(classifySourceError(err))
			})
			return
		}
		year, _ := strconv.Atoi(s.Year)
		_, err := s.Source.Catalog(s.Ctx, year, s.Season)
		s.Update(func() {
			s.SourceTesting = false
			if err != nil {
				s.Source.Retarget(previousOrigin)
				s.SourceRoute = previousLabel
				s.SourceRouteError = routeError(err)
				return
			}
			s.SourceRouteError = ""
			s.Notice = "已切换线路，现有追番不受影响。"
			s.save()
			s.refreshCatalog()
		})
	}()
}

func (s *State) openAnime(id, from string) {
	s.AnimeID = id
	s.Page = "detail"
	s.Group = -1
	s.Groups = nil
	s.GroupIDs = nil
	s.GroupCounts = nil
	s.History = nil
	s.ResourceList = nil
	s.ChoosingGroup = false
	s.GroupSearch = ""
	s.SummaryOpen = false
	s.Rule = Rule{}
	s.Checked = map[string]bool{}
	s.EditID = ""
	s.Editing = false
	s.Backfill = false
	s.ViewResources = false
	s.DetailReturn = from
	s.Keyword = ""
	s.WordOpen = false
	s.ShowAliases = false
	s.UndoFilters, s.RedoFilters = nil, nil
	s.Suggestions = false
	s.FilterFeedback = ""
	s.AnyDraft = ""
	s.ResourceScroll = ui.ScrollState{}
	s.OnlyMatches = false
	s.EpisodeFilter = ""
	s.OnlyDownloaded = false
	s.OnlyNew = false
	s.Notice = ""
	s.HistoryError = ""
	s.refreshDetail()
}

// openFollowedResources opens an anime that already has a follow straight
// into its resource list; anime without a follow fall back to the normal
// version-selection flow.
func (s *State) openFollowedResources(id, from string) {
	s.openAnime(id, from)
	for _, sub := range s.Subscriptions {
		if sub.AnimeID == id {
			s.EditID = sub.ID
			s.Backfill = true
			s.ViewResources = true
			s.Rule = cloneRule(sub.Rule)
			break
		}
	}
}

func (s *State) editSubscription(id string) {
	sub, ok := s.subscription(id)
	if !ok {
		return
	}
	s.openAnime(sub.AnimeID, "following")
	s.EditID = sub.ID
	s.Editing = true
	s.Rule = cloneRule(sub.Rule)
}

func (s *State) viewSubscriptionResources(id string) {
	sub, ok := s.subscription(id)
	if !ok {
		return
	}
	s.openAnime(sub.AnimeID, "following")
	s.EditID = sub.ID
	s.Backfill = true
	s.ViewResources = true
	s.Rule = cloneRule(sub.Rule)
	s.Notice = ""
}

// openLibrary shows one follow's episode library: the anime's downloaded
// episodes, ready to play. The resource view behind it stays the way to pick
// releases or change the rule.
func (s *State) openLibrary(id string) {
	sub, ok := s.subscription(id)
	if !ok {
		return
	}
	s.LibraryID = id
	s.LibraryMenuOpen = false
	s.Page = "episodes"
	s.Notice = ""
	a := s.anime(sub.AnimeID)
	if s.Source != nil && a.Cover != "" {
		s.loadCovers(s.Ctx, []source.Anime{{ID: a.ID, Cover: a.Cover}})
	}
}

func (s *State) refreshDetail() {
	if s.Source == nil {
		return
	}
	if s.detailCancel != nil {
		s.detailCancel()
	}
	if s.historyCancel != nil {
		s.historyCancel()
	}
	s.historyRevision++
	ctx, cancel := context.WithCancel(s.Ctx)
	s.detailCancel = cancel
	s.detailRevision++
	rev := s.detailRevision
	id := s.AnimeID
	s.DetailLoading = true
	s.DetailError = ""
	go func() {
		d, err := s.Source.Detail(ctx, id)
		folder := ""
		if err == nil && s.Core != nil {
			folder, _ = s.Core.PreviewDirectory(core.Anime{ID: id, Title: d.Anime.Title})
		}
		s.Update(func() {
			if rev != s.detailRevision || s.AnimeID != id {
				return
			}
			s.DetailLoading = false
			if err != nil {
				s.DetailError = sourceErrorMessage(err)
				return
			}
			a := fromAnime(d.Anime)
			if old, ok := s.KnownAnime[id]; ok && a.Day < 0 {
				a.Day = old.Day
			}
			s.KnownAnime[id] = a
			s.ProposedFolder = folder
			s.loadCovers(ctx, []source.Anime{d.Anime})
			s.Groups = nil
			s.GroupIDs = nil
			s.GroupCounts = nil
			for _, g := range d.Groups {
				s.Groups = append(s.Groups, g.Name)
				s.GroupIDs = append(s.GroupIDs, g.ID)
				s.GroupCounts = append(s.GroupCounts, g.Count)
			}
			if s.EditID != "" {
				for _, sub := range s.Subscriptions {
					if sub.ID == s.EditID {
						for i, gid := range s.GroupIDs {
							if gid == sub.GroupID {
								s.Group = i
								s.refreshHistory()
								return
							}
						}
					}
				}
			}
			s.Group = -1
		})
	}()
}

func (s *State) chooseGroup(index int) {
	if s.ViewResources {
		return
	}
	if index < 0 || index >= len(s.GroupIDs) {
		return
	}
	if s.Group != index {
		s.Group = index
		s.EditID = ""
		s.Editing = false
		s.Backfill = false
		s.Rule = Rule{}
		for _, sub := range s.Subscriptions {
			if sub.AnimeID == s.AnimeID && sub.GroupID == s.GroupIDs[index] {
				s.EditID = sub.ID
				s.Editing = true
				s.Rule = cloneRule(sub.Rule)
			}
		}
		s.Checked = map[string]bool{}
		s.UndoFilters, s.RedoFilters = nil, nil
		s.FilterFeedback = ""
		s.WordOpen = false
		s.Suggestions = false
		s.refreshHistory()
		s.ResourceScroll = ui.ScrollState{}
		s.EpisodeFilter = ""
	}
	s.ChoosingGroup = false
}

func (s *State) refreshHistory() {
	if s.Source == nil || s.Group < 0 || s.Group >= len(s.GroupIDs) {
		return
	}
	if s.historyCancel != nil {
		s.historyCancel()
	}
	ctx, cancel := context.WithCancel(s.Ctx)
	s.historyCancel = cancel
	s.historyRevision++
	rev := s.historyRevision
	animeID, groupID := s.AnimeID, s.GroupIDs[s.Group]
	s.HistoryLoading = true
	s.HistoryError = ""
	s.History = nil
	s.ResourceList = nil
	go func() {
		rows, err := s.Source.History(ctx, animeID, groupID)
		s.Update(func() {
			if rev != s.historyRevision || s.AnimeID != animeID {
				return
			}
			s.HistoryLoading = false
			if err != nil {
				s.HistoryError = sourceErrorMessage(err)
				return
			}
			s.History = rows
			for _, r := range rows {
				s.ResourceList = append(s.ResourceList, Resource{ID: r.ID, Title: r.Title, Published: r.Published, Episode: core.ParseEpisode(r.Title), Bytes: r.Size, MB: legacySizeMB(r.Size), Tokens: core.ReleaseTags(r.Title)})
			}
			s.initSelection()
			s.Notice = fmt.Sprintf("已加载 %d 条历史资源", len(rows))
		})
	}()
}

func (s *State) resources() []Resource { return s.ResourceList }
