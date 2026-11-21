package main

import (
	"context"
	"slices"

	"github.com/egoist/mygo/ui"
)

const browseDetailReturn = "browse-detail"

type browseDetail struct {
	anime         Anime
	loading       bool
	failed        bool
	cancel        context.CancelFunc
	anchorID      string
	catalogScroll ui.ScrollState
	summaryScroll ui.ScrollState
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
	if known := s.KnownAnime[anime.ID]; known.Summary != "" {
		anime.Summary = known.Summary
	}
	detail.anime = anime
	detail.loading, detail.failed = false, false
	detail.summaryScroll = ui.ScrollState{}
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
			detail.anime.Summary = result.Anime.Summary
			if err == nil && detail.anime.Summary != "" {
				known, ok := s.KnownAnime[id]
				if !ok {
					known = detail.anime
				}
				known.Summary = detail.anime.Summary
				s.KnownAnime[id] = known
			}
		})
	}()
}

func (s *State) isFollowingAnime(id string) bool {
	return slices.ContainsFunc(s.Subscriptions, func(sub Subscription) bool { return sub.AnimeID == id })
}

func browseDetailWidth(c *ui.Context) float32 {
	width, _ := c.Size()
	panelWidth := 360 + 80*(min(textScale(c), 2)-1)
	if width < sidebarWidth+panelWidth+600 {
		return 0
	}
	return panelWidth
}

func (s *State) browseDetailDialog(c *ui.Context) {
	open := true
	ui.DialogBase(c.Key("browse-detail-dialog"), &open, func(backdrop, panel ui.Element) {
		width, height := c.Size()
		backdrop.Background(colors(c).scrim).Transition(fadeTransition)
		panel.Width(min(width-48, 520*max(1, min(textScale(c), 1.5)))).
			Height(min(height-64, 640)).Radius(12).Background(colors(c).paper).Clip()
		s.browseDetailContent(c, s.browseDetail)
	})
	if !open {
		s.closeBrowseDetail()
	}
}

func (s *State) browseDetailContent(c *ui.Context, detail *browseDetail) {
	pal := colors(c)
	anime := detail.anime
	following := s.isFollowingAnime(anime.ID)
	ui.Column(c).Fill().MinHeight(0).Children(func() {
		ui.Row(c).Padding(12, 12, 0).Justify(ui.End).Children(func() {
			iconButton(c, "close-browse-detail", "关闭番剧详情", "close", s.closeBrowseDetail)
		})
		ui.Scroll(c.Key("browse-summary-scroll")).TrackScroll(&detail.summaryScroll).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(0, 24, 24).Gap(20).Children(func() {
				ui.Row(c).Gap(14).AlignItems(ui.Start).Children(func() {
					s.featuredCover(c, anime, 68, 96)
					ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
						ui.Text(c, anime.Title).FontSize(textSize(c, textSection)).FontWeight(600).TextColor(pal.ink)
						caption(c, dayName(anime.Day))
						if following {
							ui.Text(c, "已追番").FontSize(textSize(c, textCaption)).TextColor(pal.blue)
						}
					})
				})
				ui.Column(c).Gap(8).Children(func() {
					heading(c, "简介")
					switch {
					case anime.Summary != "":
						ui.Text(c, anime.Summary).FontSize(textSize(c, textBody)).LineHeight(1.6).TextColor(pal.ink)
					case detail.loading:
						caption(c, "正在加载简介…")
					case detail.failed:
						caption(c, "简介加载失败")
						quiet(c, "retry-browse-summary", "重试", func() { s.loadBrowseSummary(detail) }).AlignSelf(ui.Start)
					default:
						caption(c, "暂无简介")
					}
				})
			})
		})
		ui.Divider(c).Background(pal.line)
		ui.Column(c).Padding(16, 24, 20).Children(func() {
			label := "选择字幕组"
			if following {
				label = "查看资源"
			}
			primary(c, "browse-detail-action", label, func() {
				s.openFollowedResources(anime.ID, browseDetailReturn)
			}).FillWidth()
		})
	})
}
