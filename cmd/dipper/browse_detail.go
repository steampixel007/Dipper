package main

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/source"
)

const browseDetailReturn = "browse-detail"

type cachedDetail struct {
	origin string
	detail source.Detail
}

type browseDetail struct {
	anime         Anime
	groups        []source.Group
	loaded        bool
	loading       bool
	failed        bool
	cancel        context.CancelFunc
	anchorID      string
	catalogScroll ui.ScrollState
	summaryScroll ui.ScrollState
}

func (s *State) sourceOriginKey() string {
	if s.Source == nil {
		return ""
	}
	return s.Source.Origin()
}

func (s *State) cacheDetail(id, origin string, detail source.Detail) {
	if id == "" {
		return
	}
	if s.detailCache == nil {
		s.detailCache = map[string]cachedDetail{}
	}
	detail.Groups = slices.Clone(detail.Groups)
	s.detailCache[id] = cachedDetail{origin: origin, detail: detail}
}

func (s *State) cachedDetail(id string) (source.Detail, bool) {
	entry, ok := s.detailCache[id]
	if !ok || entry.origin != s.sourceOriginKey() {
		return source.Detail{}, false
	}
	entry.detail.Groups = slices.Clone(entry.detail.Groups)
	return entry.detail, true
}

func (s *State) openBrowseDetail(anime Anime) {
	detail := s.browseDetail
	if detail == nil {
		detail = &browseDetail{anchorID: anime.ID}
		s.browseDetail = detail
	}
	if detail.anime.ID == anime.ID {
		return
	}
	if detail.cancel != nil {
		detail.cancel()
		detail.cancel = nil
	}
	if known, ok := s.KnownAnime[anime.ID]; ok {
		anime = mergeAnimeDetails(anime, known)
	}
	detail.anime = anime
	detail.groups = nil
	detail.loaded = false
	detail.loading, detail.failed = false, false
	detail.summaryScroll = ui.ScrollState{}
	if cached, ok := s.cachedDetail(anime.ID); ok {
		detail.anime = mergeAnimeDetails(detail.anime, fromAnime(cached.Anime))
		detail.groups = slices.Clone(cached.Groups)
		detail.loaded = true
		return
	}
	if anime.Summary == "" {
		s.loadBrowseSummary(detail)
	}
}

func (s *State) closeBrowseDetail() {
	if s.browseDetail != nil && s.browseDetail.cancel != nil {
		s.browseDetail.cancel()
	}
	s.browseDetail = nil
}

func (s *State) resetBrowseScroll() {
	s.CatalogScroll = ui.ScrollState{}
	if s.browseDetail != nil {
		s.browseDetail.catalogScroll = ui.ScrollState{}
		s.browseDetail.anchorID = ""
	}
}

func (s *State) loadBrowseSummary(detail *browseDetail) {
	if s.Source == nil {
		return
	}
	client, id := s.Source, detail.anime.ID
	origin := client.Origin()
	ctx, cancel := context.WithCancel(s.Ctx)
	detail.cancel = cancel
	detail.loading, detail.failed = true, false
	go func() {
		result, err := client.Detail(ctx, id)
		s.Update(func() {
			if ctx.Err() != nil || s.browseDetail != detail {
				return
			}
			cancel()
			detail.cancel = nil
			detail.loading = false
			detail.failed = err != nil
			if err != nil {
				return
			}
			s.applyBrowseDetailResult(detail, origin, result)
		})
	}()
}

func (s *State) applyBrowseDetailResult(detail *browseDetail, origin string, result source.Detail) {
	id := detail.anime.ID
	fetched := fromAnime(result.Anime)
	detail.anime.Summary = fetched.Summary
	detail.anime.BroadcastStart = fetched.BroadcastStart
	detail.anime.OfficialURL = fetched.OfficialURL
	detail.anime.BangumiURL = fetched.BangumiURL
	detail.groups = slices.Clone(result.Groups)
	detail.loaded = true
	s.cacheDetail(id, origin, result)
	known, ok := s.KnownAnime[id]
	if !ok {
		known = detail.anime
	} else {
		known.Summary = fetched.Summary
		known.BroadcastStart = fetched.BroadcastStart
		known.OfficialURL = fetched.OfficialURL
		known.BangumiURL = fetched.BangumiURL
	}
	s.KnownAnime[id] = known
}

func (s *State) animeSubscriptions(id string) []Subscription {
	var subs []Subscription
	for _, sub := range s.Subscriptions {
		if sub.AnimeID == id {
			subs = append(subs, sub)
		}
	}
	return subs
}

func (s *State) isFollowingAnime(id string) bool {
	return slices.ContainsFunc(s.Subscriptions, func(sub Subscription) bool { return sub.AnimeID == id })
}

func followedGroupsText(subs []Subscription) string {
	names := make([]string, 0, len(subs))
	for _, sub := range subs {
		name := strings.TrimSpace(sub.GroupName)
		if name == "" {
			continue
		}
		if sub.Paused {
			name += "（已暂停）"
		}
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return strings.Join(names, "、")
}

func formatBroadcastStart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if parsed, err := time.Parse("1/2/2006", raw); err == nil {
		return fmt.Sprintf("%d 年 %d 月 %d 日开播", parsed.Year(), parsed.Month(), parsed.Day())
	}
	return raw + " 开播"
}

func validExternalURL(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return "", false
	}
	return u.String(), true
}

func (s *State) openExternalURL(raw string) {
	if s.OpenURL == nil {
		return
	}
	target, ok := validExternalURL(raw)
	if !ok {
		return
	}
	s.OpenURL(target)
}

func browseDetailWidth(c *ui.Context) float32 {
	width, _ := c.Size()
	panelWidth := 384 + 80*(min(textScale(c), 2)-1)
	if width < sidebarWidth+panelWidth+600 {
		return 0
	}
	return panelWidth
}

func (s *State) browseDetailDialog(c *ui.Context) {
	open := true
	pal := colors(c)
	ui.DialogBase(c.Key("browse-detail-dialog"), &open, func(backdrop, panel ui.Element) {
		width, height := c.Size()
		backdrop.Background(pal.scrim).Transition(fadeTransition)
		panel.Width(min(width-48, 520*max(1, min(textScale(c), 1.5)))).
			Height(min(height-48, 640)).
			Radius(radiusSheet).
			Background(pal.surface).
			Border(1, pal.outline).
			Clip()
		s.browseDetailContent(c, s.browseDetail)
	})
	if !open {
		s.closeBrowseDetail()
	}
}

func (s *State) browseDetailContent(c *ui.Context, detail *browseDetail) {
	pal := colors(c)
	anime := detail.anime
	subs := s.animeSubscriptions(anime.ID)
	following := len(subs) > 0
	ui.Column(c).Fill().MinHeight(0).Children(func() {
		ui.Row(c).Padding(16, 16, 0).Justify(ui.End).Children(func() {
			iconButton(c, "close-browse-detail", "关闭番剧详情", "close", s.closeBrowseDetail)
		})
		ui.Scroll(c.Key("browse-summary-scroll")).TrackScroll(&detail.summaryScroll).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(8, 24, 24).Gap(24).Children(func() {
				ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
					s.featuredCover(c, anime, 72, 100)
					ui.Column(c).Grow(1).MinWidth(0).Gap(8).Children(func() {
						pageTitle(c, anime.Title)
						meta := dayName(anime.Day)
						if start := formatBroadcastStart(anime.BroadcastStart); start != "" {
							meta += " · " + start
						}
						caption(c, meta)
						if following {
							ui.Text(c, "已追番").FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.accentInk)
							if groupsText := followedGroupsText(subs); groupsText != "" {
								caption(c, "正在追："+groupsText)
							}
						}
					})
				})
				ui.Column(c).Gap(12).Children(func() {
					heading(c, "简介")
					switch {
					case anime.Summary != "":
						ui.Text(c, anime.Summary).FontSize(textSize(c, textBody)).FontWeight(400).LineHeight(1.5).TextColor(pal.ink)
					case detail.loading:
						caption(c, "正在加载简介…")
					case detail.failed:
						caption(c, "简介加载失败")
						compactQuiet(c, "retry-browse-summary", "重试", func() { s.loadBrowseSummary(detail) }).AlignSelf(ui.Start)
					default:
						caption(c, "暂无简介")
					}
				})
				if detail.loaded {
					ui.Column(c).Gap(12).Children(func() {
						if len(detail.groups) > 0 {
							heading(c, fmt.Sprintf("字幕组（%d）", len(detail.groups)))
							ui.Row(c).Wrap().Gap(8).Children(func() {
								for _, group := range detail.groups {
									if slices.ContainsFunc(subs, func(sub Subscription) bool { return sub.GroupID == group.ID }) {
										statusPill(c, group.Name, pal.accentInk, pal.accentWash)
									} else {
										ui.Text(c, group.Name).FontSize(textSize(c, textMeta)).FontWeight(400).TextColor(pal.ink).Background(pal.sunken).Radius(radiusSmall).Padding(4, 8)
									}
								}
							})
						} else {
							heading(c, "字幕组")
							caption(c, "暂无字幕组发布资源")
						}
					})
				}
				if anime.BangumiURL != "" || anime.OfficialURL != "" {
					ui.Column(c).Gap(12).Children(func() {
						heading(c, "相关链接")
						ui.Row(c).Wrap().Gap(8).Children(func() {
							if anime.BangumiURL != "" {
								compactSecondary(c, "browse-bangumi-link", "Bangumi 番组计划", func() {
									s.openExternalURL(anime.BangumiURL)
								}).Disabled(s.OpenURL == nil)
							}
							if anime.OfficialURL != "" {
								compactSecondary(c, "browse-official-link", "官方网站", func() {
									s.openExternalURL(anime.OfficialURL)
								}).Disabled(s.OpenURL == nil)
							}
						})
					})
				}
			})
		})
		ui.Divider(c).Background(pal.line)
		ui.Column(c).Padding(20, 24, 24).Gap(12).Children(func() {
			label := "选择观看版本"
			if following {
				label = "查看资源"
			}
			primary(c, "browse-detail-action", label, func() {
				s.openFollowedResources(anime.ID, browseDetailReturn)
			}).FillWidth()
			if following {
				secondary(c, "browse-detail-library", "进入剧集库", func() {
					s.openLibraryFrom(subs[0].ID, browseDetailReturn)
				}).FillWidth()
			}
		})
	})
}
