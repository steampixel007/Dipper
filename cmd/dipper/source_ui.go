package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

func newState(dir string) *State {
	s := &State{Dir: dir, Page: "browse", Day: -1, Group: -1, Checked: map[string]bool{}, KnownAnime: map[string]Anime{}, Ctx: context.Background()}
	s.Update = func(fn func()) { fn() }
	s.setCurrentSeason()
	s.Root, s.SaveError = initialDownloadRoot()
	s.Upload = "1 MB/s"
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
	return Anime{ID: a.ID, Title: a.Title, Cover: a.Cover, Day: day, Source: "蜜柑计划"}
}

func (s *State) refreshCatalog() {
	if s.Source == nil {
		return
	}
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
				s.SourceError = err.Error()
				return
			}
			for _, a := range rows {
				v := fromAnime(a)
				s.Catalog = append(s.Catalog, v)
				s.KnownAnime[a.ID] = v
			}
			s.loadCovers(ctx, rows)
		})
	}()
}

func (s *State) openAnime(id string) {
	s.AnimeID = id
	s.Page = "detail"
	s.Group = -1
	s.Groups = nil
	s.GroupIDs = nil
	s.GroupCounts = nil
	s.History = nil
	s.ResourceList = nil
	s.ChoosingGroup = false
	s.Rule = Rule{}
	s.Checked = map[string]bool{}
	s.EditID = ""
	s.Editing = false
	s.Backfill = false
	s.Keyword = ""
	s.WordOpen = false
	s.ShowAliases = false
	s.UndoFilters, s.RedoFilters = nil, nil
	s.Suggestions, s.Confirm = false, false
	s.FilterFeedback = ""
	s.AnyDraft = ""
	s.ResourceScroll = ui.ScrollState{}
	s.OnlyMatches = false
	s.Notice = ""
	s.HistoryError = ""
	s.refreshDetail()
}

func (s *State) editSubscription(index int) {
	if index < 0 || index >= len(s.Subscriptions) {
		return
	}
	sub := s.Subscriptions[index]
	s.openAnime(sub.AnimeID)
	s.EditID = sub.ID
	s.Editing = true
	s.Rule = cloneRule(sub.Rule)
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
				s.DetailError = err.Error()
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
	}
	s.ChoosingGroup = false
	s.ResourceScroll = ui.ScrollState{}
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
				s.HistoryError = err.Error()
				return
			}
			s.History = rows
			for _, r := range rows {
				s.ResourceList = append(s.ResourceList, Resource{ID: r.ID, Title: r.Title, Published: r.Published, Episode: core.ParseEpisode(r.Title), MB: int((r.Size + 999999) / 1000000), Tokens: core.SuggestWords(r.Title)})
			}
			s.initSelection()
			s.Notice = fmt.Sprintf("已加载 %d 条历史资源", len(rows))
		})
	}()
}

func (s *State) resources() []Resource { return s.ResourceList }
