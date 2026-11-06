package main

import (
	"cmp"
	"embed"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"
)

//go:embed assets/*.jpg
var assets embed.FS

// The bird mark is a copy of build/icon/dipper-small.svg; regenerate the PNG
// copies with build/icon/gen.go (see README) and keep them in step.
//
//go:embed assets/dipper.svg
var brandSVG []byte

//go:embed assets/dipper-icon.png
var brandIconPNG []byte

//go:embed assets/dipper-tray.png
var brandTrayPNG []byte

//go:embed assets/dipper-tray-dark.png
var brandTrayDarkPNG []byte

// Cover loading and rendering touch these maps from different goroutines, so
// access is guarded instead of relying on the UI thread alone.
var (
	coversMu sync.Mutex
	covers   = map[string]*ui.Bitmap{}

	// Cover URLs whose load is in flight; loadCovers sets and clears them.
	loadingCovers = map[string]bool{}
)

var brandLogo *ui.SVG

var iconPaths = map[string]string{
	"browse":    `<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>`,
	"following": `<path d="M6 3h12v18l-6-4-6 4z"/>`,
	"downloads": `<path d="M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5"/>`,
	"settings":  `<path d="M4 6h16M4 12h16M4 18h16"/><circle cx="8" cy="6" r="2"/><circle cx="16" cy="12" r="2"/><circle cx="10" cy="18" r="2"/>`,
	"folder":    `<path d="M3 7V4h7l2 3h9v13H3z"/>`,
	"back":      `<path d="m14 5-7 7 7 7"/>`,
	"next":      `<path d="m10 5 7 7-7 7"/>`,
	"help":      `<circle cx="12" cy="12" r="9"/><path d="M9 8a3 3 0 1 1 4 3c-1 .5-1 1-1 2m0 3v.1"/>`,
	"more":      `<circle cx="5" cy="12" r="1.4"/><circle cx="12" cy="12" r="1.4"/><circle cx="19" cy="12" r="1.4"/>`,
	"check":     `<path d="m5 12 4 4L19 6"/>`,
	"search":    `<circle cx="10" cy="10" r="6"/><path d="m15 15 6 6"/>`,
	"close":     `<path d="m6 6 12 12M6 18 18 6"/>`,
	"pause":     `<path d="M8 5v14M16 5v14"/>`,
	"play":      `<path d="m8 4 12 8-12 8z"/>`,
	"clock":     `<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`,
	"refresh":   `<path d="M20 10a8 8 0 0 0-14-4L3 9m0-6v6h6M4 14a8 8 0 0 0 14 4l3-3m0 6v-6h-6"/>`,
	"download":  `<path d="M12 3v14m-5-5 5 5 5-5M4 21h16"/>`,
	"upload":    `<path d="M12 20V6m-5 5 5-5 5 5M4 3h16"/>`,
	"spark":     `<path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5z"/>`,
}
var icons = map[string]*ui.SVG{}

func loadImages() {
	brandLogo = ui.MustParseSVG(brandSVG)
	for name, path := range iconPaths {
		icons[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
	entries, _ := assets.ReadDir("assets")
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".jpg") {
			b, _ := assets.ReadFile("assets/" + entry.Name())
			if bm, err := ui.DecodeBitmap(b); err == nil {
				coversMu.Lock()
				covers[entry.Name()] = bm
				coversMu.Unlock()
			}
		}
	}
}

func icon(c *ui.Context, name string) {
	if svg := icons[name]; svg != nil {
		ui.Icon(c, svg).FontSize(18)
	}
}
func quiet(c *ui.Context, key, label string, action func()) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Padding(controlPaddingY, controlPaddingX).Radius(controlRadius).Gap(controlGap).FontSize(textSize(c, textBody)).TextColor(colors(c).ink).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(colors(c).chrome)
	}
	return tactile(b).Children(func() { ui.Text(c, label) }).OnClick(action)
}
func secondary(c *ui.Context, key, label string, action func()) ui.Element {
	return tactile(ui.Button(c.Key(key), label)).Radius(controlRadius).Padding(controlPaddingY, controlPaddingX).Gap(controlGap).FontSize(textSize(c, textBody)).Border(1, colors(c).outline).OnClick(action)
}
func primary(c *ui.Context, key, label string, action func()) ui.Element {
	return tactile(ui.PrimaryButton(c.Key(key), label)).Radius(controlRadius).Padding(controlPaddingY, controlPaddingX).Gap(controlGap).FontSize(textSize(c, textBody)).FontWeight(600).OnClick(action)
}
func iconButton(c *ui.Context, key, label, name string, action func()) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Size(36, 36).Shrink(0).Center().Radius(controlRadius).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(colors(c).line)
	}
	return tactile(b).Children(func() { icon(c, name) }).OnClick(action)
}
func caption(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted).LineHeight(1.5)
}
func badge(c *ui.Context, text string, col ui.Color, bg ui.Color) {
	ui.Text(c, text).FontSize(textSize(c, textCaption)).FontWeight(600).TextColor(col).Background(bg).Radius(4).Padding(3, 7)
}
func heading(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(textSize(c, textSection)).FontWeight(600).TextColor(colors(c).ink)
}
func cover(c *ui.Context, a Anime, w, h float32) {
	coversMu.Lock()
	b := covers[a.Cover]
	coversMu.Unlock()
	if b != nil {
		ui.Image(c, b).Size(w, h).Fit(ui.Cover).Radius(8)
	} else if a.Cover != "" {
		ui.Box(c).Size(w, h).Radius(8).Clip().Background(colors(c).blueWash).Center().Children(func() {
			loading.Blocks(c, loading.Options{Size: 26, Color: colors(c).blue}).Label("正在加载封面")
		})
	} else {
		size := min(w, h) * 0.62
		ui.Box(c).Size(w, h).Radius(8).Clip().Background(colors(c).blueWash).Center().Children(func() { ui.Image(c, brandLogo).Size(size, size).Background(colors(c).brandWash).Radius(8) })
	}
}

func (s *State) view(c *ui.Context) {
	installTheme(c, s.darkAppearance(c.Theme().Dark))
	if s.ReduceEffects || c.Preferences().ReduceMotion {
		c.DisableAnimations()
	}
	ui.Overlay(c, func() {})
	ui.Row(c).Fill().Gap(0).Background(colors(c).chrome).Children(func() {
		s.sidebar(c)
		ui.Column(c).Grow(1).FillHeight().MinWidth(0).Background(colors(c).paper).Children(func() {
			s.topbar(c)
			if s.SaveError != "" {
				ui.Text(c, s.SaveError).FontSize(textSize(c, textCaption)).TextColor(colors(c).errorInk).Background(colors(c).errorWash).Padding(8, 24)
			}
			ui.Box(c.Key("page-host")).Grow(1).MinHeight(0).FillWidth().Clip().Children(func() {
				ui.Column(c.Key("page-" + s.Page)).Fill().Transition(pageTransition).Children(func() {
					switch s.Page {
					case "detail":
						s.detail(c)
					case "following":
						s.following(c)
					case "episodes":
						s.library(c)
					case "downloads":
						s.downloads(c)
					case "settings":
						s.settings(c)
					default:
						s.browse(c)
					}
				})
			})
		})
	})
	s.cancelDialog(c)
	s.removeDialog(c)
	s.helpDialog(c)
	s.summaryDialog(c)
	s.closeDialog(c)
	s.filesDialog(c)
	if s.Page == "detail" && !s.Help && !s.SummaryOpen && !s.ClosePrompt {
		c.OnShortcut(ui.Ctrl, ui.KeyZ, s.undoFilter)
		c.OnShortcut(ui.Ctrl, ui.KeyY, s.redoFilter)
		c.OnShortcut(ui.Ctrl|ui.Shift, ui.KeyZ, s.redoFilter)
		if s.Suggestions && !s.WordOpen {
			c.OnShortcut(0, ui.KeyEscape, func() { s.Suggestions = false })
		}
	}
}

func (s *State) sidebar(c *ui.Context) {
	_, height := c.Size()
	ui.Scroll(c.Key("sidebar-scroll")).Width(204).FillHeight().Background(colors(c).chrome).Children(func() {
		ui.Column(c).MinHeight(height).Padding(24, 14, 16).Gap(8).Children(func() {
			ui.Row(c).Gap(10).Padding(0, 4, 20).Children(func() {
				ui.Box(c).Size(40, 40).Radius(11).Background(colors(c).brandWash).Center().Children(func() { ui.Image(c, brandLogo).Size(32, 32) })
				ui.Text(c, "Dipper").FontSize(textSize(c, textSection)).FontWeight(600).TextColor(colors(c).ink).Grow(1).MinWidth(0)
			})
			for _, nav := range []struct{ id, label string }{{"browse", "浏览番剧"}, {"following", "我的追番"}, {"downloads", "下载"}} {
				current := nav
				active := s.Page == nav.id || (s.Page == "detail" && nav.id == "browse") || (s.Page == "episodes" && nav.id == "following")
				b := ui.ButtonBase(c.Key("nav-"+nav.id)).Label(current.label).FillWidth().MinHeight(44).Padding(10, 14).Gap(12).Radius(controlRadius).Cursor(ui.CursorPointer)
				tactile(b)
				if active {
					b.Background(colors(c).paper).TextColor(colors(c).ink).FontWeight(600)
				} else if b.Hovered() {
					b.Background(colors(c).line)
				}
				b.Children(func() {
					if active {
						ui.Box(c).Absolute().Left(6).Top(18).Size(4, 4).Radius(2).Background(colors(c).blue)
					}
					icon(c, current.id)
					ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
						ui.Text(c, current.label).FontSize(textSize(c, textBody))
						if current.id == "following" && len(s.Subscriptions) > 0 {
							failed := 0
							for _, sub := range s.Subscriptions {
								if sub.CheckFailed {
									failed++
								}
							}
							if failed > 0 {
								ui.Textf(c, "%d 失败", failed).FontSize(textSize(c, textCaption)).FontWeight(600).TextColor(colors(c).amber)
							} else {
								ui.Textf(c, "%d", len(s.Subscriptions)).FontSize(textSize(c, textCaption))
							}
						}
						if current.id == "downloads" && len(s.Downloads) > 0 {
							failed := 0
							for _, d := range s.Downloads {
								if d.Failed {
									failed++
								}
							}
							if failed > 0 {
								ui.Textf(c, "%d 失败", failed).FontSize(textSize(c, textCaption)).FontWeight(600).TextColor(colors(c).amber)
							} else {
								ui.Textf(c, "%d", len(s.Downloads)).FontSize(textSize(c, textCaption))
							}
						}
					})
				}).OnClick(func() { s.enterPage(current.id); s.Notice = "" })
			}
			ui.Spacer(c)
			if textScale(c) < 2 {
				ui.Column(c).Padding(12).Gap(8).Children(func() {
					ui.Row(c).Gap(6).Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(colors(c).green)
						caption(c, "保存在本机")
					})
				})
			}
			ui.Divider(c).Background(colors(c).line)
			b := ui.ButtonBase(c.Key("nav-settings")).MinHeight(44).Padding(10, 14).Gap(12).Radius(controlRadius).Children(func() { icon(c, "settings"); ui.Text(c, "设置").FontSize(textSize(c, textBody)) }).OnClick(func() { s.enterPage("settings"); s.Notice = "" })
			tactile(b)
			if s.Page == "settings" {
				b.Background(colors(c).paper).TextColor(colors(c).ink)
			} else if b.Hovered() {
				b.Background(colors(c).line)
			}
			quiet(c, "help", "追番小指南", func() { s.Help = true }).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
		})
	})
}

func (s *State) topbar(c *ui.Context) {
	names := map[string]string{"browse": "浏览番剧", "detail": "浏览番剧 / 开始追番", "following": "我的追番", "downloads": "下载", "settings": "设置"}
	names["episodes"] = "我的追番 / 剧集库"
	if s.Backfill {
		if s.ViewResources {
			names["detail"] = "我的追番 / 查看资源"
		} else {
			names["detail"] = "浏览番剧 / 下载已有资源"
		}
	}
	bar := c.TitleBar()
	ui.Row(c).MinHeight(64).Shrink(0).Padding(12, bar.Right+24, 12, 28).Gap(12).Background(colors(c).chrome).DragWindow().Children(func() {
		ui.Text(c, names[s.Page]).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(colors(c).ink).MinWidth(0).Grow(1)
		iconButton(c, "top-help", "打开追番指南", "help", func() { s.Help = true })
	})
}

func (s *State) browse(c *ui.Context) {
	ui.Column(c).Grow(1).MinHeight(0).Padding(26, 28, 0).Gap(18).Children(func() {
		ui.Column(c).Gap(12).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(7).Children(func() {
				ui.Text(c, s.Year+" 年"+s.Season+"季番剧").FontSize(textSize(c, textTitle)).FontWeight(600).TextColor(colors(c).ink).ContentTransition(ui.ContentTransition{Animation: &contentMotion})
				caption(c, "选好番剧和版本，新更新会自动下载。")
			})
		})
		if s.SourceError != "" {
			ui.Row(c).Wrap().Gap(controlGap).Children(func() {
				quiet(c, "catalog-error-previous", "上一季", func() { s.changeSeason(-1) })
				quiet(c, "catalog-error-next", "下一季", func() { s.changeSeason(1) })
				quiet(c, "catalog-error-current", "回到当季", s.currentSeason)
			})
			s.empty(c, "来源暂时无法连接", s.SourceError, "重试加载", s.refreshCatalog)
			return
		}
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			if ui.TextInput(c.Key("search"), &s.Search).Label("搜索番剧").Placeholder("搜索本季番剧").Width(260).MinHeight(inputHeight(c)).Background(colors(c).paper).Changed() {
				s.CatalogScroll = ui.ScrollState{}
			}
			if strings.TrimSpace(s.Search) != "" {
				quiet(c, "clear-browse-search", "清除搜索", func() { s.Search = "" }).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue)
			}
		})
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			iconButton(c, "prev-season", "上一季", "back", func() { s.changeSeason(-1) })
			ui.Select(c.Key("year"), &s.Year, catalogYears(s.Year)).Label("年份").Width(max(96, textSize(c, textBody)*5)).MinHeight(inputHeight(c)).Background(colors(c).paper).OnChange(s.refreshCatalog)
			ui.Row(c).Gap(4).Children(func() {
				for _, season := range seasons {
					q := season
					b := quiet(c, "season-"+q, q+"季", func() { s.Season = q; s.CatalogScroll = ui.ScrollState{}; s.refreshCatalog() })
					if s.Season == q {
						b.Background(colors(c).blueWash).TextColor(colors(c).blue).FontWeight(600)
					}
				}
			})
			iconButton(c, "next-season", "下一季", "next", func() { s.changeSeason(1) })
			ui.Spacer(c)
			quiet(c, "current-season", "回到当季", s.currentSeason)
		})
		if !s.IntroHidden {
			ui.Row(c.Key("beginner-hint")).Padding(10, 14).Gap(12).Radius(8).Background(colors(c).canvas).Transition(ui.ElementTransition{Duration: 190 * time.Millisecond, Ease: spectrumEase, Exit: &ui.Motion{Collapse: true}, Size: true}).Children(func() {
				ui.Box(c).TextColor(colors(c).muted).Children(func() { icon(c, "spark") })
				ui.Text(c, "第一次追番？").FontSize(textSize(c, textBody)).FontWeight(600)
				ui.Text(c, "选字幕组，确认一次，后续自动更新。").FontSize(textSize(c, textCaption)).TextColor(colors(c).muted).Grow(1)
				quiet(c, "intro-help", "了解怎么追", func() { s.Help = true }).TextColor(colors(c).ink).FontSize(textSize(c, textCaption))
				iconButton(c, "dismiss-intro", "收起新手提示", "close", func() { s.IntroHidden = true; s.save() }).Disabled(s.Busy)
			})
		}
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			for i, label := range []string{"全部", "周一", "周二", "周三", "周四", "周五", "周六", "周日"} {
				d := i - 1
				if i == 7 {
					d = 0
				}
				if i > 0 && i < 7 {
					d = i
				}
				day := d
				b := quiet(c, fmt.Sprintf("day-%d", i), label, func() { s.Day = day; s.CatalogScroll = ui.ScrollState{} }).FontSize(textSize(c, textBody))
				if s.Day == day {
					b.Background(colors(c).blueWash).TextColor(colors(c).blue).FontWeight(600)
				}
			}
			ui.Spacer(c)
			caption(c, "来源：蜜柑计划")
		})
		if s.SourceLoading {
			pageLoading(c, "正在加载番剧目录…")
		} else {
			var filtered []Anime
			for _, a := range s.Catalog {
				if (s.Day < 0 || a.Day == s.Day) && strings.Contains(normalize(a.Title), normalize(strings.TrimSpace(s.Search))) {
					filtered = append(filtered, a)
				}
			}
			if len(filtered) == 0 {
				title, body := "没有找到这部番剧", "试试缩短番名，或切换到全部星期。"
				if strings.TrimSpace(s.Search) == "" {
					title = "暂无番剧"
					body = "当前季度或星期暂无番剧，试试切换季度或查看全部星期。"
				}
				s.empty(c, title, body, "清除筛选", func() { s.Search = ""; s.Day = -1 })
			} else {
				width, _ := c.Size()
				cols, cardWidth := gridColumns(width)
				ui.Scroll(c.Key("catalog")).TrackScroll(&s.CatalogScroll).Grow(1).MinHeight(0).Padding(0, 0, 24).Children(func() {
					ui.Grid(c).Columns(cols).GapX(18).GapY(24).Children(func() {
						for _, a := range filtered {
							anime := a
							s.animeCard(c, anime, cardWidth)
						}
					})
				})
			}
		}
	})
}

func (s *State) animeCard(c *ui.Context, a Anime, width float32) {
	b := ui.ButtonBase(c.Key("anime-" + a.ID)).Label("查看 " + a.Title).Width(width).Column().Gap(9).Padding(0).AlignItems(ui.Stretch).Radius(8).Cursor(ui.CursorPointer)
	pose := ui.Pose{}
	if b.Hovered() {
		pose.Y = -3
		pose.Scale = 1.008
	}
	if b.Pressed() {
		pose.Y = 0
		pose.Scale = .985
	}
	b.AnimatePose(pose, contentMotion)
	b.Children(func() {
		ui.Box(c).Children(func() {
			cover(c, a, width, width*1.36)
			for _, sub := range s.Subscriptions {
				if sub.AnimeID == a.ID {
					ui.Text(c, "已追番").FontSize(textSize(c, textCaption)).TextColor(colors(c).accentText).Background(colors(c).blue).Radius(4).Padding(4, 7).Absolute().Top(8).Left(8)
					break
				}
			}
		})
		ui.Text(c, a.Title).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(colors(c).ink)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			ui.Text(c, dayName(a.Day)).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
			label := "选择版本"
			for _, sub := range s.Subscriptions {
				if sub.AnimeID == a.ID {
					label = "查看资源"
					break
				}
			}
			hint := ui.Text(c, label).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
			if b.Hovered() {
				hint.TextColor(colors(c).blue)
			}
			if n := s.completedDownloads(a.ID); n > 0 {
				ui.Text(c, fmt.Sprintf("已完成 %d 个下载", n)).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
			}
		})
	}).OnClick(func() { s.openFollowedResources(a.ID, "browse") })
}
func dayName(d int) string {
	if d < 0 || d > 6 {
		return "更新时间未知"
	}
	return []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[d%7]
}

func catalogYears(selected string) []string {
	year, err := strconv.Atoi(selected)
	if err != nil || year < 1 {
		year = 2000
	}
	var years []string
	for y := max(time.Now().Year(), year); y >= min(2000, max(1, year)); y-- {
		years = append(years, strconv.Itoa(y))
	}
	return years
}

func pageLoading(c *ui.Context, text string) {
	ui.Column(c).Grow(1).Center().Gap(14).Children(func() { caption(c, text) })
}

// enterPage switches pages and reloads covers the switch may have evicted.
func (s *State) enterPage(page string) {
	if s.Page == page {
		return
	}
	s.Page = page
	s.LibraryMenuOpen = false
	switch page {
	case "browse", "following", "episodes":
		s.loadCovers(s.Ctx, s.pageCoverRows(page))
	}
}

func (s *State) empty(c *ui.Context, title, body, button string, action func()) {
	ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
		ui.Box(c).TextColor(colors(c).muted).Children(func() { icon(c, "browse") })
		heading(c, title)
		caption(c, body)
		secondary(c, "empty-action", button, action)
	})
}

func (s *State) detail(c *ui.Context) {
	a := s.anime(s.AnimeID)
	_, height := c.Size()
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		if detailUsesPageScroll(c, a) {
			ui.Scroll(c.Key("detail-content")).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Padding(14, 28, 24).Gap(20).Children(func() {
					s.detailHeader(c, a)
					ui.Divider(c).Background(colors(c).line)
					s.detailOptions(c, a, true)
					s.detailPreview(c, true)
					s.detailFooter(c, true)
				})
			})
		} else {
			ui.Scroll(c.Key("detail-header")).MaxHeight(height * .25).MinHeight(0).Children(func() {
				s.detailHeader(c, a)
			})
			ui.Divider(c).Background(colors(c).line)
			ui.Row(c).Grow(1).MinHeight(0).Gap(0).Children(func() {
				ui.Scroll(c.Key("detail-options")).Width(306).FillHeight().Padding(22, 22, 24, 28).BorderWidth(0, 1, 0, 0).BorderColor(colors(c).line).Children(func() {
					s.detailOptions(c, a, false)
				})
				s.detailPreview(c, false)
			})
			s.detailFooter(c, false)
		}
	})
}

func detailUsesPageScroll(c *ui.Context, a Anime) bool {
	return textScale(c) > 1 || len([]rune(a.Title)) > 24
}

func (s *State) detailHeader(c *ui.Context, a Anime) {
	ui.Column(c).Padding(18, 28, 16).Gap(12).Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			backLabel := "返回番剧列表"
			backPage := "browse"
			if s.DetailReturn == "following" {
				backLabel = "返回我的追番"
				backPage = "following"
			} else if s.DetailReturn == "downloads" {
				backLabel = "返回下载"
				backPage = "downloads"
			}
			iconButton(c, "back-browse", backLabel, "back", func() { s.enterPage(backPage) })
			quiet(c, "back-list", backLabel, func() { s.enterPage(backPage) }).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
		})
		ui.Row(c).Wrap().Gap(controlGap).AlignItems(ui.Start).Children(func() {
			cover(c, a, 52, 72)
			ui.Column(c).Grow(1).MinWidth(0).Gap(7).Children(func() {
				ui.Text(c, a.Title).FontSize(textSize(c, textTitle)).FontWeight(700)
				caption(c, dayName(a.Day)+"，来源：蜜柑计划")
				switch {
				case s.ViewResources:
					ui.Row(c).Wrap().Gap(controlGap).Children(func() {
						badge(c, "已追番资源", colors(c).blue, colors(c).blueWash)
						quiet(c, "fresh-selection", "重新选字幕组", func() {
							s.EditID = ""
							s.Backfill = false
							s.ViewResources = false
							s.ChoosingGroup = true
							s.Group = -1
							s.Checked = map[string]bool{}
							s.Keyword = ""
							s.WordOpen = false
							s.Rule = Rule{}
							s.OnlyNew = false
						}).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Disabled(s.Busy)
					})
				case s.Backfill:
					badge(c, "下载已有资源", colors(c).blue, colors(c).blueWash)
				}
			})
		})
	})
}

func (s *State) detailGroups(c *ui.Context) {
	heading(c, "字幕组")
	if s.DetailLoading {
		caption(c, "正在加载字幕组…")
	} else if s.DetailError != "" {
		ui.Text(c, s.DetailError).FontSize(textSize(c, textCaption)).TextColor(colors(c).amber)
		secondary(c, "retry-detail", "重试加载字幕组", s.refreshDetail).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
	} else if !s.hasGroup() || s.ChoosingGroup {
		s.groupPicker(c)
	} else {
		ui.Row(c).Padding(10, 12).Radius(6).Border(1, colors(c).blue).Background(colors(c).blueWash).Children(func() {
			ui.Text(c, s.Groups[s.Group]).FontSize(textSize(c, textBody)).FontWeight(600).Grow(1).MinWidth(0)
			if !s.ViewResources {
				quiet(c, "change-group", "更换", func() { s.ChoosingGroup = true }).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Padding(3, 4).Disabled(s.Busy)
			}
		})
	}
}

func (s *State) detailOptions(c *ui.Context, a Anime, groupFirst bool) {
	ui.Column(c).Gap(14).Children(func() {
		if groupFirst {
			s.detailGroups(c)
			s.animeSummary(c, a)
		} else {
			s.animeSummary(c, a)
			s.detailGroups(c)
		}
		if s.detailReady() {
			ui.Divider(c).Background(colors(c).line).Margin(2, 0)
			heading(c, "你想要什么版本")
			if s.Suggestions {
				s.titlePreferences(c)
			} else {
				s.ruleEditor(c)
			}
		}
	})
}

func (s *State) detailPreview(c *ui.Context, embedded bool) {
	column := ui.Column(c).MinWidth(0).Gap(12)
	if embedded {
		column.Padding(0)
	} else {
		column.Grow(1).FillHeight().Padding(20, 24, 0)
	}
	column.Children(func() {
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			heading(c, "可下载资源")
			ui.Spacer(c)
			if s.EpisodeFilter != "" {
				badge(c, "只看"+episodeLabel(s.EpisodeFilter), colors(c).blue, colors(c).blueWash)
				quiet(c, "clear-episode-filter", "清除", func() { s.toggleEpisodeFilter(s.EpisodeFilter) }).FontSize(textSize(c, textCaption)).Padding(3, 0).TextColor(colors(c).blue)
			}
			if s.detailReady() {
				ui.Checkbox(c.Key("only-matches"), &s.OnlyMatches, "只看匹配").FontSize(textSize(c, textBody))
				ui.Checkbox(c.Key("only-downloaded"), &s.OnlyDownloaded, "只看已有任务").FontSize(textSize(c, textBody))
				if s.ViewResources && s.EditID != "" && (s.OnlyNew || s.hasNewRelease()) {
					ui.Checkbox(c.Key("only-new"), &s.OnlyNew, "只看新资源").FontSize(textSize(c, textBody))
				}
			}
		})
		if s.DetailLoading {
			pageLoading(c, "正在加载字幕组…")
		} else if s.DetailError != "" {
			s.empty(c, "字幕组加载失败", s.DetailError, "重试加载", s.refreshDetail)
		} else if len(s.Groups) == 0 {
			s.empty(c, "暂无字幕组", "来源尚未提供这部番剧的字幕组信息。", "重新加载", s.refreshDetail)
		} else if !s.hasGroup() {
			pageLoading(c, "先选择一个字幕组，即可预览发布资源。")
		} else if s.HistoryLoading {
			pageLoading(c, "正在加载发布记录…")
		} else if s.HistoryError != "" {
			s.empty(c, "发布记录加载失败", s.HistoryError, "重试加载资源", s.refreshHistory)
		} else {
			s.preview(c, embedded)
		}
	})
}

func (s *State) hasGroup() bool {
	return s.Group >= 0 && s.Group < len(s.Groups) && s.Group < len(s.GroupIDs) && s.GroupIDs[s.Group] != ""
}

func (s *State) detailReady() bool {
	return s.hasGroup() && !s.DetailLoading && !s.HistoryLoading && s.DetailError == "" && s.HistoryError == ""
}

func (s *State) ruleEditor(c *ui.Context) {
	ui.Column(c).Gap(10).Children(func() {
		for _, kind := range []struct {
			title  string
			helper string
			words  []string
			color  ui.Color
		}{{"只要这些词", "标题里必须都有", s.Rule.All, colors(c).blue}, {"不要这些词", "标题里出现任意一个就不下载", s.Rule.None, colors(c).avoidInk}} {
			k := kind
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, k.title).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(k.color)
				if len(k.words) == 0 {
					caption(c, "还没选，暂不限制")
				} else {
					caption(c, k.helper)
				}
				ui.Row(c).Wrap().Gap(5).Children(func() {
					for i, word := range k.words {
						w := word
						key := fmt.Sprintf("rule-%s-%d", k.title, i)
						ui.Row(c.Key(key)).Gap(0).Transition(rowTransition).Children(func() {
							s.wordChip(c, key, w)
							iconButton(c, key+"-remove", "移除条件「"+w+"」", "close", func() { s.setWord(w, wordUnrestricted) }).Size(20, 20)
						})
					}
				})
			})
		}
		for idx, g := range s.Rule.Any {
			ui.Column(c).Padding(8, 10).Radius(6).Background(colors(c).blueWash).Gap(4).Children(func() {
				ui.Row(c).Gap(4).Children(func() {
					ui.Text(c, "任意一个写法都可以").FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Grow(1)
					iconButton(c, fmt.Sprintf("remove-alias-%d", idx), fmt.Sprintf("移除第%d组写法", idx+1), "close", func() {
						rule := cloneRule(s.Rule)
						rule.Any = slices.Delete(rule.Any, idx, idx+1)
						s.applyRule(rule, "移除任意词组")
					}).Size(20, 20)
				})
				ui.Text(c, strings.Join(g, " / ")).FontSize(textSize(c, textCaption)).TextColor(colors(c).ink)
			})
		}
		ui.Divider(c).Background(colors(c).line)
		ui.Text(c, "添加标题词").FontSize(textSize(c, textBody)).FontWeight(600)
		ui.TextInput(c.Key("keyword"), &s.Keyword).Label("标题关键词").Placeholder("输入标题中的词").FillWidth().MinHeight(inputHeight(c))
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			secondary(c, "want-custom", "只要这个词", func() { s.setWord(s.Keyword, wordWant); s.Keyword = "" }).FontSize(textSize(c, textBody)).Padding(6, 10).TextColor(colors(c).blue).Disabled(strings.TrimSpace(s.Keyword) == "")
			secondary(c, "avoid-custom", "不要这个词", func() { s.setWord(s.Keyword, wordAvoid); s.Keyword = "" }).FontSize(textSize(c, textBody)).Padding(6, 10).TextColor(colors(c).avoidInk).Disabled(strings.TrimSpace(s.Keyword) == "")
		})
		quiet(c, "toggle-aliases", "添加同义写法", func() { s.ShowAliases = !s.ShowAliases }).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Padding(3, 0)
		if s.ShowAliases {
			caption(c, "用 / 分隔同一类标签的不同写法，例如：简日内嵌 / CHS。")
			ui.TextInput(c.Key("aliases"), &s.AnyDraft).Label("同义写法").Placeholder("简日内嵌 / CHS").FillWidth().MinHeight(inputHeight(c))
			secondary(c, "add-alias-group", "添加这组写法", s.addAliases).FontSize(textSize(c, textBody)).Padding(6, 10).Disabled(strings.TrimSpace(s.AnyDraft) == "")
		}
		quiet(c, "clear-rules", "重置全部条件", func() { s.applyRule(Rule{}, "重置全部条件"); s.WordOpen = false }).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted).Padding(3, 0)
	})
}

// tagChip renders one recognized release label as a quiet tag. Clicking it
// still opens the same word filter popover as an explicit chip.
func (s *State) tagChip(c *ui.Context, key, word string) {
	col, bg := colors(c).ink, colors(c).canvas
	if choice := s.wordChoice(word); choice == wordWant {
		col, bg = colors(c).blue, colors(c).blueWash
	} else if choice == wordAvoid {
		col, bg = colors(c).avoidInk, colors(c).avoidWash
	}
	b := quiet(c, key, word, func() { s.WordAnchor = key; s.WordOpen = true }).Label("筛选 "+word).FontSize(textSize(c, textCaption)).Padding(4, 8).Radius(4).Border(1, colors(c).line).Background(bg).TextColor(col).Tooltip("选择只要、不要，或移除条件")
	if s.WordAnchor == key {
		s.wordPopover(c, b, key, word)
	}
}
func (s *State) wordChip(c *ui.Context, key, word string) {
	choice := s.wordChoice(word)
	label := word
	col, bg := colors(c).ink, colors(c).paper
	if choice == wordWant {
		label = "＋ " + word
		col, bg = colors(c).blue, colors(c).blueWash
	}
	if choice == wordAvoid {
		label = "− " + word
		col, bg = colors(c).avoidInk, colors(c).avoidWash
	}
	b := quiet(c, key, label, func() { s.WordAnchor = key; s.WordOpen = true }).Label("筛选 "+word).FontSize(textSize(c, textCaption)).Padding(5, 8).Radius(5).Border(1, colors(c).line).Background(bg).TextColor(col).Tooltip("选择只要、不要，或移除条件")
	if s.WordAnchor == key {
		s.wordPopover(c, b, key, word)
	}
}

func (s *State) wordPopover(c *ui.Context, anchor ui.Element, key, word string) {
	choice := s.wordChoice(word)
	ui.PopoverBase(c.Key("word-popup-"+key), anchor, &s.WordOpen, func(panel ui.Element) {
		panel.Width(320).Margin(6, 0, 0).Padding(10).Gap(6).Radius(10).Background(colors(c).paper).Border(1, colors(c).line).Shadow(0, 6, 20, 0, colors(c).popoverShadow).Transition(popupTransition)
		ui.Text(c, "筛选「"+word+"」").FontSize(textSize(c, textBody)).FontWeight(600).Padding(4, 6)
		for _, option := range []struct {
			label, description string
			choice             wordChoice
			color, bg          ui.Color
		}{{"只要包含", "标题里必须有这个词", wordWant, colors(c).blue, colors(c).blueWash}, {"不要包含", "标题里有这个词就不下载", wordAvoid, colors(c).avoidInk, colors(c).avoidWash}} {
			o := option
			count := s.countRuleMatches(ruleWithWord(s.Rule, word, o.choice))
			button := ui.ButtonBase(c.Key(fmt.Sprintf("word-choice-%d", o.choice))).Label(map[wordChoice]string{wordWant: "只要含「" + word + "」的", wordAvoid: "不要含「" + word + "」的"}[o.choice]).Column().FillWidth().Gap(5).Padding(12).Radius(6).Border(1, colors(c).line).Background(colors(c).paper).Cursor(ui.CursorPointer)
			tactile(button)
			if button.Hovered() || choice == o.choice {
				button.Background(o.bg).BorderColor(o.color)
			}
			button.Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, o.label).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(o.color)
					ui.Spacer(c)
					ui.Textf(c, "剩 %d 条", count).FontSize(textSize(c, textCaption)).TextColor(o.color)
				})
				caption(c, o.description)
			}).OnClick(func() { s.setWord(word, o.choice) })
		}
		if choice != wordUnrestricted {
			quiet(c, "unrestrict-word", "不再限制这个词", func() { s.setWord(word, wordUnrestricted) }).Label("不再限制「" + word + "」").FontSize(textSize(c, textCaption)).FillWidth()
		}
		caption(c, "只匹配原标题，不判断视频内容。")
	})
}

func (s *State) quickFilter(c *ui.Context) {
	ui.Column(c).Padding(12).Gap(8).Background(colors(c).paper).Border(1, colors(c).line).Radius(8).Children(func() {
		ui.Text(c, "点词筛选版本").FontSize(textSize(c, textBody)).FontWeight(600)
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for i, word := range s.quickWords() {
				s.wordChip(c, fmt.Sprintf("quick-word-%d", i), word)
			}
		})
	})
}

func (s *State) preview(c *ui.Context, embedded bool) {
	s.quickFilter(c)
	n, eps, _ := s.matchSummary()
	var matches []Resource
	for _, r := range s.resources() {
		if ok, _ := decide(r.Title, s.Rule); ok {
			matches = append(matches, r)
		}
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Textf(c, "匹配 %d 条", n).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(colors(c).blue).ContentTransition(ui.ContentTransition{Numeric: true, Animation: &contentMotion})
		caption(c, fmt.Sprintf("覆盖 %d 集，%s", eps, resourceSizes(matches)))
		if len(s.UndoFilters) > 0 {
			quiet(c, "undo-filter", "撤销上一步", s.undoFilter).Tooltip("撤销"+s.UndoFilters[len(s.UndoFilters)-1].Action+" (Ctrl+Z)").FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Padding(3, 0)
		}
		if len(s.RedoFilters) > 0 {
			quiet(c, "redo-filter", "重做筛选", s.redoFilter).Tooltip("重做"+s.RedoFilters[len(s.RedoFilters)-1].Action+" (Ctrl+Y)").FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Padding(3, 0)
		}
	})
	if s.FilterFeedback != "" {
		ui.Text(c, s.FilterFeedback).FontSize(textSize(c, textCaption)).TextColor(colors(c).ink).ContentTransition(ui.ContentTransition{Animation: &contentMotion})
	}
	if len(s.Rule.All) == 0 && len(s.Rule.Any) == 0 {
		if len(s.Rule.None) == 0 {
			caption(c, "还没限制版本，这组全部资源都会匹配。")
		} else {
			caption(c, "只排除写出这些词的标题。标题没写的，也会匹配。")
		}
	}
	if s.Backfill || s.ViewResources {
		caption(c, "勾选要下载的资源。筛选不会修改追番条件。")
	} else if s.Editing {
		caption(c, "调整条件只影响以后发布的资源；已经看过或跳过的不会自动下载。")
	} else {
		caption(c, "匹配的已有资源默认全选。取消勾选可跳过这次下载，不影响以后自动下载。")
	}
	if n == 0 {
		message := "没有资源符合这些条件。试着移除一个关键词，或检查标题里的实际写法。"
		if len(s.resources()) == 0 {
			message = "这组暂无已发布资源，开启追番后将按条件检查后续更新。"
		}
		ui.Text(c, message).FontSize(textSize(c, textCaption)).TextColor(colors(c).amber).Padding(10).Background(colors(c).amberWash).Radius(controlRadius)
	}
	if !s.Editing {
		selected, _ := s.selected()
		all := n > 0 && len(selected) == n
		ui.Row(c).Children(func() {
			ui.Checkbox(c.Key("select-all"), &all, "全选匹配资源").FontSize(textSize(c, textBody)).Disabled(n == 0).OnChange(func() {
				for _, r := range s.resources() {
					if ok, _ := decide(r.Title, s.Rule); ok {
						s.Checked[r.ID] = all
					}
				}
			})
			ui.Spacer(c)
			quiet(c, "select-none", "清空选择", func() {
				for _, r := range s.resources() {
					s.Checked[r.ID] = false
				}
			}).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted).Padding(4, 0)
		})
	}
	renderRows := func() {
		ui.Column(c).Gap(0).Children(func() {
			visible := 0
			for _, r := range s.previewResources() {
				res := r
				ok, _ := decide(res.Title, s.Rule)
				if s.OnlyMatches && !ok {
					continue
				}
				if s.EpisodeFilter != "" && res.Episode != s.EpisodeFilter {
					continue
				}
				if s.OnlyDownloaded {
					if _, _, found := s.downloadForResource(res); !found {
						continue
					}
				}
				if s.OnlyNew && s.ViewResources && s.EditID != "" && !s.isNewRelease(res.Title) {
					continue
				}
				visible++
				s.resourceRow(c, res)
			}
			if visible == 0 && len(s.previewResources()) > 0 {
				s.empty(c, "没有符合筛选的资源", "试试清除筛选，看看这个字幕组的全部资源。", "清除筛选", s.clearPreviewFilters)
			}
		})
	}
	if embedded {
		ui.Column(c).Padding(0, 0, 18).Children(renderRows)
	} else {
		ui.Scroll(c.Key("resources")).TrackScroll(&s.ResourceScroll).Grow(1).MinHeight(0).Padding(0, 0, 18).Children(renderRows)
	}
}

// previewResources orders the preview rows by episode number so a season
// reads top to bottom; releases without an episode sort last.
func (s *State) previewResources() []Resource {
	list := slices.Clone(s.resources())
	slices.SortStableFunc(list, func(a, b Resource) int {
		ea, oka := episodeNumber(a.Episode)
		eb, okb := episodeNumber(b.Episode)
		switch {
		case oka && okb:
			return cmp.Compare(ea, eb)
		case oka:
			return -1
		case okb:
			return 1
		default:
			return 0
		}
	})
	return list
}

func episodeNumber(episode string) (float64, bool) {
	if episode == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(episode, 64)
	return value, err == nil
}

func episodeLabel(episode string) string {
	episode = strings.TrimSpace(episode)
	if episode == "" {
		return "未识别集数"
	}
	if number, err := strconv.ParseInt(episode, 10, 64); err == nil && number >= 0 {
		return fmt.Sprintf("第 %d 集", number)
	}
	return "第 " + episode + " 集"
}

// downloadForResource finds the download for this resource by identity:
// the core-resolved info hash first, then the resource the task was created
// from. Titles repeat across releases and never identify a download.
func (s *State) downloadForResource(r Resource) (Download, int, bool) {
	if hash := s.resourceHashes[r.ID]; hash != "" {
		for i, d := range s.Downloads {
			if d.ID == hash {
				return d, i, true
			}
		}
	}
	for i, d := range s.Downloads {
		if d.ResourceID == r.ID {
			return d, i, true
		}
	}
	return Download{}, 0, false
}

func (s *State) resourceRow(c *ui.Context, r Resource) {
	ok, reason := decide(r.Title, s.Rule)
	ui.Column(c.Key("resource-"+r.ID)).Padding(12, 0).Gap(7).BorderWidth(0, 0, 1, 0).BorderColor(colors(c).line).Transition(rowTransition).Children(func() {
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			on := s.Checked[r.ID] && ok
			if !s.Editing {
				ui.Checkbox(c.Key("pick-"+r.ID), &on, "").Label("选择 " + r.ID).Disabled(!ok).OnChange(func() { s.Checked[r.ID] = on })
			}
			ui.Text(c, episodeLabel(r.Episode)).FontSize(textSize(c, textBody)).FontWeight(600)
			if s.isNewRelease(r.Title) {
				badge(c, "新", colors(c).blue, colors(c).blueWash)
			}
			if s.Suggestions && s.SuggestedResource.ID == r.ID {
				badge(c, "正在参考", colors(c).blue, colors(c).blueWash)
			}
			if count := s.duplicates(r.Episode); count > 1 && ok {
				ep := r.Episode
				fg, bg := colors(c).amber, colors(c).amberWash
				if s.EpisodeFilter == r.Episode {
					fg, bg = colors(c).blue, colors(c).blueWash
				}
				quiet(c, "filter-episode-"+r.Episode, fmt.Sprintf("本集匹配 %d 条", count), func() { s.toggleEpisodeFilter(ep) }).FontSize(textSize(c, textCaption)).FontWeight(600).TextColor(fg).Background(bg).Radius(4).Padding(3, 7)
			}
			if d, idx, found := s.downloadForResource(r); found {
				switch {
				case d.Failed:
					badge(c, "失败", colors(c).amber, colors(c).amberWash)
				case d.Progress < 1 && d.Stopped:
					badge(c, "已停止", colors(c).muted, colors(c).canvas)
				case d.Progress < 1 && d.Paused:
					badge(c, "已暂停", colors(c).muted, colors(c).canvas)
				case d.Progress < 1:
					badge(c, fmt.Sprintf("下载中 %d%%", int(d.Progress*100)), colors(c).blue, colors(c).blueWash)
				default:
					badge(c, "已有任务", colors(c).green, colors(c).greenWash)
				}
				if d.Progress >= 1 && !d.Failed {
					quiet(c, "play-resource-"+r.ID, "播放", func() { s.playDownload(idx) }).Label("播放 "+r.ID).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Padding(3, 0).Disabled(s.Busy)
					quiet(c, "open-resource-folder-"+r.ID, "打开文件夹", func() {
						if s.OpenFolder != nil {
							s.OpenFolder(d.Folder)
						}
					}).Label("打开文件夹 "+r.ID).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Padding(3, 0).Disabled(s.Busy || s.OpenFolder == nil)
				}
			}
			ui.Spacer(c)
			if bytes := sizeBytes(r.Bytes, r.MB); bytes > 0 {
				caption(c, byteSizeText(bytes))
			} else {
				caption(c, "大小未知")
			}
		})
		ui.Text(c, r.Title).FontSize(textSize(c, textBody)).LineHeight(1.6).Selectable().TextColor(colors(c).ink)
		published := r.Published
		if published == "" {
			published = "发布时间未知"
		}
		caption(c, published)
		ui.Row(c).Wrap().Gap(4).Children(func() {
			for i, word := range r.Tokens {
				s.tagChip(c, fmt.Sprintf("tag-%s-%d", r.ID, i), word)
			}
			ui.Spacer(c)
			quiet(c, "suggest-"+r.ID, "按此资源筛选", func() {
				s.WordOpen = false
				s.SuggestedResource = r
				s.Suggestions = true
				s.ResourceScroll = ui.ScrollState{}
			}).Label("按此资源筛选 "+r.ID).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Padding(3, 0)
		})
		col := colors(c).green
		if !ok {
			col = colors(c).muted
		}
		ui.Text(c, reason).FontSize(textSize(c, textCaption)).TextColor(col)
	})
}

func (s *State) detailFooter(c *ui.Context, embedded bool) {
	var plan submissionPlan
	if s.detailReady() {
		plan = s.planSubmission()
	}
	resources := plan.New
	_, height := c.Size()
	render := func() {
		ui.Column(c).Padding(14, 28).Gap(7).Background(colors(c).paper).BorderWidth(1, 0, 0, 0).BorderColor(colors(c).line).Children(func() {
			ui.Row(c).Wrap().Gap(controlGap).Children(func() {
				ui.Column(c).FillWidth().MinWidth(0).Gap(5).Children(func() {
					title := fmt.Sprintf("本次下载 %d 个资源，%s", len(resources), resourceSizes(resources))
					if len(resources) == 0 {
						title = "本次不下载，只追后续更新"
					}
					if s.Editing {
						title = "保存后，只下载以后发布的资源"
					}
					if s.Backfill {
						title = fmt.Sprintf("本次下载 %d 个资源，%s", len(resources), resourceSizes(resources))
						if len(resources) == 0 {
							title = "请选择要下载的已有资源"
							if plan.Incomplete+plan.Completed+plan.Failed > 0 {
								title = "所选资源已有下载任务"
							}
						}
					}
					if !s.detailReady() {
						title = "请先加载字幕组和资源"
					}
					ui.Text(c, title).FontSize(textSize(c, textBody)).FontWeight(600)
					if plan.Incomplete+plan.Completed+plan.Failed > 0 {
						caption(c, fmt.Sprintf("已有任务：未完成 %d 个，已完成 %d 个，失败 %d 个", plan.Incomplete, plan.Completed, plan.Failed))
					}
					caption(c, "保存到："+s.targetFolder())
				})
				label := "下载并追番"
				if len(resources) == 0 {
					label = "开启自动追番"
				}
				if s.Editing {
					label = "保存追番条件"
				}
				if s.Backfill {
					label = "下载选中资源"
				}
				if s.Busy {
					label = "正在提交…"
				}
				if plan.Failed > 0 {
					quiet(c, "submission-recovery", "查看失败任务并重试", func() {
						s.WordOpen, s.Suggestions = false, false
						s.DownloadFilter = downloadsFailed
						s.DownloadSearch = ""
						s.enterPage("downloads")
					}).Disabled(s.Busy)
				}
				if s.Editing {
					quiet(c, "begin-backfill", "下载已有资源", s.beginBackfill).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Disabled(!s.detailReady() || s.Busy)
				}
				primary(c, "start-follow", label, s.commit).Disabled(!s.detailReady() || s.Busy || (s.Backfill && len(resources) == 0))
			})
			if s.Backfill {
				if sub, ok := s.subscription(s.EditID); ok && sub.Paused {
					caption(c, "追番已暂停；现在只会排队，恢复追番后开始下载。")
				}
			} else if !s.Editing && !s.ViewResources && textScale(c) <= 1 {
				caption(c, "以后发布的资源会按这些标题条件自动下载。")
			}
		})
	}
	if embedded {
		ui.Column(c).Radius(8).BorderWidth(1).BorderColor(colors(c).line).Children(render)
		return
	}
	ui.Scroll(c.Key("detail-footer")).MaxHeight(height * .32).MinHeight(0).Children(render)
}

func (s *State) cancelDialog(c *ui.Context) {
	sub, ok := s.subscription(s.CancelID)
	if !ok {
		return
	}
	ui.DialogBase(c, &s.CancelPrompt, func(backdrop, panel ui.Element) {
		backdrop.Background(colors(c).scrim).Transition(fadeTransition)
		dialogContent(c, panel, "cancel-content", 460, func() {
			group := sub.GroupName
			if group == "" {
				group = "字幕组名称未知"
			}
			ui.Text(c, "确认取消追番").FontSize(textSize(c, textTitle)).FontWeight(700)
			heading(c, s.anime(sub.AnimeID).Title)
			ui.Column(c).Padding(16).Radius(8).Background(colors(c).canvas).Gap(8).Children(func() {
				ui.Text(c, "字幕组："+group).FontSize(textSize(c, textBody))
				caption(c, "取消后不再检查更新；已下载的文件保留在 "+sub.Folder)
				flight := 0
				for _, d := range s.Downloads {
					if d.AnimeID == sub.AnimeID && !d.Paused && !d.Failed && d.Progress < 1 {
						flight++
					}
				}
				if flight > 0 {
					caption(c, fmt.Sprintf("正在下载的 %d 个任务会继续完成，不受影响。", flight))
				}
			})
			ui.Row(c).Wrap().Justify(ui.End).Gap(controlGap).Children(func() {
				secondary(c, "keep-subscription", "保留追番", func() { s.CancelPrompt = false })
				primary(c, "confirm-cancel-subscription", "确认取消追番", func() {
					s.CancelPrompt, s.CancelID = false, ""
					s.removeSubscription(sub.ID)
				}).Label("确认取消追番 " + sub.GroupName).Disabled(s.Busy)
			})
		})
	})
}

func (s *State) removeDialog(c *ui.Context) {
	var d Download
	found := false
	for _, v := range s.Downloads {
		if v.ID == s.RemoveID {
			d, found = v, true
			break
		}
	}
	if !found {
		return
	}
	ui.DialogBase(c, &s.RemovePrompt, func(backdrop, panel ui.Element) {
		backdrop.Background(colors(c).scrim).Transition(fadeTransition)
		dialogContentWithFooter(c, panel, "remove-content", 460, func() {
			ui.Text(c, "删除任务和文件？").FontSize(textSize(c, textTitle)).FontWeight(700)
			heading(c, s.anime(d.AnimeID).Title)
			ui.Column(c).Padding(16).Radius(8).Background(colors(c).canvas).Gap(8).Children(func() {
				ui.Text(c, d.Title).FontSize(textSize(c, textBody))
				caption(c, "已下载文件会一并删除，无法恢复。")
			})
		}, func() {
			secondary(c, "keep-download", "保留任务", func() { s.RemovePrompt = false })
			secondary(c, "confirm-redownload", "删除并重新下载", func() {
				s.RemovePrompt, s.RemoveID = false, ""
				s.removeDownload(d.ID, true)
			}).Label("确认删除并重新下载").Disabled(s.Busy)
			secondary(c, "confirm-remove-download", "删除任务和文件", func() {
				s.RemovePrompt, s.RemoveID = false, ""
				s.removeDownload(d.ID, false)
			}).Label("确认删除任务和文件").Disabled(s.Busy)
		})
	})
}

func resourceSizes(resources []Resource) string {
	known, unknown := int64(0), 0
	for _, r := range resources {
		if bytes := sizeBytes(r.Bytes, r.MB); bytes > 0 {
			if known > int64(^uint64(0)>>1)-bytes {
				return "总大小超出可显示范围"
			}
			known += bytes
		} else {
			unknown++
		}
	}
	if unknown == 0 {
		return byteSizeText(known)
	}
	if known == 0 {
		return fmt.Sprintf("%d 条大小未知", unknown)
	}
	return fmt.Sprintf("已知大小 %s，另有 %d 条大小未知", byteSizeText(known), unknown)
}
func joinWords(words []string) string {
	if len(words) == 0 {
		return "不限制"
	}
	return strings.Join(words, "、")
}

func (s *State) following(c *ui.Context) {
	ui.Column(c).Grow(1).MinHeight(0).Padding(26, 28).Gap(20).Children(func() {
		ui.Text(c, "我的追番").FontSize(textSize(c, textTitle)).FontWeight(700)
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			if ui.TextInput(c.Key("follow-search"), &s.FollowSearch).Label("搜索追番").Placeholder("搜索已追番剧").Width(240).MinHeight(textSize(c, textBody)*1.5 + 16).Background(colors(c).paper).Changed() {
				s.FollowScroll = ui.ScrollState{}
			}
			for _, sub := range s.Subscriptions {
				if sub.CheckFailed {
					quiet(c, "retry-failed-checks", "重试检查", s.retryFailedChecks).TextColor(colors(c).amber).Disabled(s.Busy)
					break
				}
			}
			secondary(c, "find-more", "添加追番", func() { s.enterPage("browse") })
		})
		if s.Notice != "" {
			ui.Row(c).Gap(12).Padding(12, 16).Radius(6).TextColor(colors(c).green).Background(colors(c).greenWash).Children(func() {
				ui.Text(c, s.Notice).FontSize(textSize(c, textBody)).Grow(1).MinWidth(0)
				if s.UndoSaved != nil {
					quiet(c, "undo-save", "撤销保存", s.undoSave).Disabled(s.Busy)
				}
			})
		}
		if len(s.Subscriptions) == 0 {
			s.empty(c, "还没有追番", "先找到想看的番剧，选好版本，之后会自动下载。", "浏览番剧", func() { s.enterPage("browse") })
			return
		}
		var subs []Subscription
		for _, sub := range s.Subscriptions {
			if s.follows(sub) {
				subs = append(subs, sub)
			}
		}
		// Failed RSS checks float above the saved order so a silent failure
		// cannot hide at the bottom of a long list.
		slices.SortStableFunc(subs, func(a, b Subscription) int {
			switch {
			case a.CheckFailed && !b.CheckFailed:
				return -1
			case !a.CheckFailed && b.CheckFailed:
				return 1
			default:
				return 0
			}
		})
		ui.Row(c).Gap(6).Children(func() {
			caption(c, fmt.Sprintf("%d 部追番", len(subs)))
			ui.Spacer(c)
			if strings.TrimSpace(s.FollowSearch) != "" {
				quiet(c, "clear-follow-search", "清除搜索", func() { s.FollowSearch = "" }).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Padding(3, 0)
			}
		})
		if len(subs) == 0 {
			s.empty(c, "没有找到这部追番", "试试缩短番名，或清除搜索。", "清除搜索", func() { s.FollowSearch = "" })
			return
		}
		width, _ := c.Size()
		cols, cardWidth := gridColumns(width)
		ui.Scroll(c.Key("following-list")).TrackScroll(&s.FollowScroll).Grow(1).MinHeight(0).Padding(0, 0, 24).Children(func() {
			ui.Grid(c).Columns(cols).GapX(18).GapY(24).Children(func() {
				for _, sub := range subs {
					item := sub
					s.followTile(c, item, cardWidth)
				}
			})
		})
	})
}

func gridColumns(width float32) (int, float32) {
	cols := int((width - 260) / 180)
	if cols < 3 {
		cols = 3
	}
	if cols > 6 {
		cols = 6
	}
	return cols, (width - 260 - float32(cols-1)*18) / float32(cols)
}

func (s *State) followTile(c *ui.Context, sub Subscription, width float32) {
	a := s.anime(sub.AnimeID)
	tile := ui.ButtonBase(c.Key("subscription-" + sub.ID)).Label("打开追番 " + sub.GroupName).Width(width).Column().Gap(8).Padding(0).AlignItems(ui.Stretch).Radius(8).Cursor(ui.CursorPointer)
	pose := ui.Pose{}
	if tile.Hovered() {
		pose.Y = -3
		pose.Scale = 1.008
	}
	if tile.Pressed() {
		pose.Y = 0
		pose.Scale = .985
	}
	tile.AnimatePose(pose, contentMotion)
	tile.Children(func() {
		ui.Box(c).Children(func() {
			cover(c, a, width, width*1.36)
			ui.Column(c).Absolute().Top(8).Left(8).Right(8).Gap(4).AlignItems(ui.Start).Children(func() {
				if sub.CheckFailed {
					badge(c, "检查失败", colors(c).amber, colors(c).amberWash)
				}
				if sub.Paused {
					badge(c, "已暂停", colors(c).muted, colors(c).canvas)
				} else if sub.Active {
					label := "下载中"
					if sub.Progress > 0 {
						label = fmt.Sprintf("下载中 %d%%", int(sub.Progress*100))
					}
					badge(c, label, colors(c).blue, colors(c).blueWash)
				} else {
					status := ui.Text(c, "自动追番中").FontSize(textSize(c, textCaption)).FontWeight(600).TextColor(colors(c).green).Background(colors(c).greenWash).Radius(4).Padding(3, 7)
					if !tile.Hovered() {
						status.Opacity(0)
					}
				}
			})
		})
		ui.Text(c, a.Title).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(colors(c).ink)
		ui.Column(c).Gap(5).Children(func() {
			group := sub.GroupName
			if group == "" {
				group = "字幕组名称未知"
			}
			ui.Text(c, group).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
			hint := ui.Text(c, "进入剧集库").FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
			if tile.Hovered() {
				hint.TextColor(colors(c).blue)
			}
		})
		ui.Row(c).Wrap().Gap(controlGap).AlignItems(ui.Center).Children(func() {
			if sub.NewCount > 0 {
				quiet(c, "open-new-"+sub.ID, fmt.Sprintf("新资源 %d 条", sub.NewCount), func() {
					s.viewSubscriptionResources(sub.ID)
					s.OnlyNew = true
				}).Label(fmt.Sprintf("新资源 %d 条 %s", sub.NewCount, sub.GroupName)).FontSize(textSize(c, textCaption)).FontWeight(600).TextColor(colors(c).amber).Background(colors(c).amberWash).Disabled(s.Busy)
			}
			if sub.Completed > 0 {
				ui.Text(c, fmt.Sprintf("已完成 %d 个下载", sub.Completed)).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
			}
		})
	}).OnClick(func() { s.openLibrary(sub.ID) })
}

// libraryEpisodes lists the indices of one anime's downloads sorted by
// episode number, so a season reads top to bottom; transfers without an
// episode sort last.
func (s *State) libraryEpisodes(animeID string) []int {
	indices := make([]int, 0, len(s.Downloads))
	for i, d := range s.Downloads {
		if d.AnimeID == animeID {
			indices = append(indices, i)
		}
	}
	slices.SortStableFunc(indices, func(a, b int) int {
		ea, oka := episodeNumber(s.Downloads[a].Episode)
		eb, okb := episodeNumber(s.Downloads[b].Episode)
		switch {
		case oka && okb:
			return cmp.Compare(ea, eb)
		case oka:
			return -1
		case okb:
			return 1
		default:
			return 0
		}
	})
	return indices
}

// library is the watch view of one follow: its downloaded episodes, ready to
// play. Picking releases and changing the rule stay on the resource view.
func (s *State) library(c *ui.Context) {
	sub, ok := s.subscription(s.LibraryID)
	if !ok {
		s.empty(c, "找不到这条追番", "它可能已被取消追番。", "回到我的追番", func() { s.enterPage("following") })
		return
	}
	indices := s.libraryEpisodes(sub.AnimeID)
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		ui.Row(c).Padding(18, 28, 0).Gap(6).Children(func() {
			iconButton(c, "back-following", "返回我的追番", "back", func() { s.enterPage("following") })
			quiet(c, "back-following-list", "返回我的追番", func() { s.enterPage("following") }).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
		})
		s.libraryHero(c, sub)
		ui.Divider(c).Background(colors(c).line)
		if len(indices) == 0 {
			s.empty(c, "还没有下载完成的剧集", "正在下载的剧集也会显示；完成后就能播放。", "查看资源", func() { s.viewSubscriptionResources(sub.ID) })
			return
		}
		ui.Scroll(c.Key("episode-list")).Grow(1).MinHeight(0).Padding(14, 28, 24).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				for _, i := range indices {
					idx := i
					d := s.Downloads[idx]
					ui.Row(c.Key("episode-"+d.ID)).Wrap().Gap(controlGap).Padding(14, 16).Background(colors(c).paper).Border(1, colors(c).line).Radius(8).Children(func() {
						ui.Text(c, episodeLabel(d.Episode)).FontSize(textSize(c, textBody)).FontWeight(600)
						switch {
						case d.Failed:
							badge(c, "失败", colors(c).amber, colors(c).amberWash)
						case d.Progress < 1 && d.Stopped:
							badge(c, "已停止", colors(c).muted, colors(c).canvas)
						case d.Progress < 1 && d.Paused:
							badge(c, "已暂停", colors(c).muted, colors(c).canvas)
						case d.Progress < 1:
							badge(c, fmt.Sprintf("下载中 %d%%", int(d.Progress*100)), colors(c).blue, colors(c).blueWash)
						default:
							badge(c, "已完成", colors(c).green, colors(c).greenWash)
						}
						ui.Column(c).FillWidth().MinWidth(0).Gap(4).Children(func() {
							ui.Text(c, d.Title).FontSize(textSize(c, textBody)).TextColor(colors(c).muted)
							if d.Error != "" {
								ui.Text(c, d.Error).FontSize(textSize(c, textCaption)).TextColor(colors(c).amber)
							}
						})
						switch {
						case d.Failed:
							quiet(c, "retry-episode-"+d.ID, "重试", func() { s.retryDownload(idx) }).Label("重试 " + d.ID).FontSize(textSize(c, textBody)).TextColor(colors(c).blue).Disabled(s.Busy)
						case d.Progress < 1 && (d.Paused || d.Stopped):
							quiet(c, "resume-episode-"+d.ID, "继续", func() { s.pauseDownload(idx) }).Label("继续 " + d.ID).FontSize(textSize(c, textBody)).TextColor(colors(c).blue).Disabled(s.Busy)
						case d.Progress >= 1:
							quiet(c, "play-episode-"+d.ID, "播放", func() { s.playDownload(idx) }).Label("播放 " + d.ID).FontSize(textSize(c, textBody)).TextColor(colors(c).blue).Disabled(s.Busy)
							quiet(c, "open-episode-folder-"+d.ID, "打开文件夹", func() {
								if s.OpenFolder != nil {
									s.OpenFolder(d.Folder)
								}
							}).Label("打开文件夹 " + d.ID).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Disabled(s.Busy || s.OpenFolder == nil || d.Folder == "")
						}
					})
				}
			})
		})
	})
}

func (s *State) libraryMenuItem(c *ui.Context, key, label, accessLabel string, disabled bool, action func()) {
	b := ui.ButtonBase(c.Key(key)).Label(accessLabel).FillWidth().Padding(8, 12).Radius(6).Cursor(ui.CursorPointer).Disabled(disabled)
	if b.Hovered() {
		b.Background(colors(c).chrome)
	}
	tactile(b).Children(func() { ui.Text(c, label).FontSize(textSize(c, textBody)) }).OnClick(func() {
		s.LibraryMenuOpen = false
		action()
	})
}

// seedStopRule describes both seeding stop conditions in one clause.
func (s *State) seedStopRule() string {
	timeCopy := ""
	switch s.Seed {
	case "不限时长":
	default:
		timeCopy = "分享满 " + s.Seed
	}
	ratioCopy := ""
	switch s.SeedRatio {
	case "不限":
	case "2:1", "3:1":
		ratioCopy = "分享率达到 " + s.SeedRatio
	default:
		ratioCopy = "分享率达到 1:1"
	}
	switch {
	case timeCopy == "" && ratioCopy == "":
		return "不会自动停止分享"
	case timeCopy == "":
		return ratioCopy + " 后停止分享"
	case ratioCopy == "":
		return timeCopy + "后停止分享"
	}
	return ratioCopy + " 或" + timeCopy + "后停止分享"
}

func downloadState(d Download) DownloadFilter {
	switch {
	case d.Failed:
		return downloadsFailed
	case d.Progress >= 1:
		return downloadsCompleted
	case d.Stopped:
		return downloadsStopped
	case d.Paused:
		return downloadsPaused
	default:
		return downloadsDownloading
	}
}

// toggleDownloadGroup flips one anime's collapsed state on the downloads
// page. The map is per-session, so a fresh start shows every group open.
func (s *State) toggleDownloadGroup(animeID string) {
	if s.CollapsedDownloads == nil {
		s.CollapsedDownloads = make(map[string]bool)
	}
	s.CollapsedDownloads[animeID] = !s.CollapsedDownloads[animeID]
}

// hasNewRelease reports whether the followed resource view has any release
// still waiting in the subscription's pending set, so 只看新资源 only shows
// when it can hide something.
func (s *State) hasNewRelease() bool {
	for _, res := range s.resources() {
		if s.isNewRelease(res.Title) {
			return true
		}
	}
	return false
}

// completedDownloads counts an anime's finished transfers with the follow
// card's 已下载 semantics: progress complete and not failed.
func (s *State) completedDownloads(animeID string) int {
	count := 0
	for _, d := range s.Downloads {
		if d.AnimeID == animeID && d.Progress >= 1 && !d.Failed {
			count++
		}
	}
	return count
}

// clearPreviewFilters resets the view-only preview filters — 只看匹配, the
// episode chip, 只看已有任务 and 只看新资源 — without touching the saved rule.
func (s *State) clearPreviewFilters() {
	s.OnlyMatches, s.OnlyDownloaded, s.OnlyNew = false, false, false
	s.EpisodeFilter = ""
	s.ResourceScroll = ui.ScrollState{}
}

// downloadSearches reports whether the download survives the downloads-page
// search: an empty query matches everything, otherwise the anime title or the
// release title must contain the query.
func (s *State) downloadSearches(d Download) bool {
	query := normalize(strings.TrimSpace(s.DownloadSearch))
	if query == "" {
		return true
	}
	if strings.Contains(normalize(d.Title), query) {
		return true
	}
	return strings.Contains(normalize(s.anime(d.AnimeID).Title), query)
}

func (s *State) clearDownloadSearch() {
	s.DownloadSearch = ""
	s.DownloadScroll = ui.ScrollState{}
}

func (s *State) downloadMatches(d Download) bool {
	return s.DownloadFilter == downloadsAll || downloadState(d) == s.DownloadFilter
}

func (s *State) downloads(c *ui.Context) {
	active := 0
	for _, d := range s.Downloads {
		if !d.Paused && !d.Stopped && !d.Failed && d.Progress < 1 {
			active++
		}
	}
	if s.DownloadFilterLast != s.DownloadFilter {
		s.DownloadFilterLast = s.DownloadFilter
		s.DownloadScroll = ui.ScrollState{}
	}
	ui.Column(c).Grow(1).MinHeight(0).Padding(26, 28).Gap(18).Children(func() {
		ui.Row(c).Gap(24).Children(func() {
			ui.Column(c).Grow(1).Gap(6).Children(func() {
				ui.Text(c, "下载").FontSize(textSize(c, textTitle)).FontWeight(700)
			})
			ui.Column(c).Gap(5).Children(func() {
				caption(c, "总下载速度")
				ui.Text(c, speedText(s.downloadSpeed(), false)).FontSize(textSize(c, textSection)).FontWeight(600).TextColor(colors(c).blue)
			})
			ui.Column(c).Gap(5).Children(func() {
				caption(c, "总上传速度")
				ui.Text(c, speedText(s.uploadSpeed(), true)).FontSize(textSize(c, textSection)).FontWeight(600)
			})
		})
		ui.Row(c).Wrap().Gap(12).Children(func() {
			if ui.TextInput(c.Key("download-search"), &s.DownloadSearch).Label("搜索下载").Placeholder("搜索番剧或任务").Width(260).MinHeight(inputHeight(c)).Background(colors(c).paper).Changed() {
				s.DownloadScroll = ui.ScrollState{}
			}
			if strings.TrimSpace(s.DownloadSearch) != "" {
				quiet(c, "clear-download-search", "清除搜索", func() { s.clearDownloadSearch() }).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue)
			}
		})
		visible := make([]int, 0, len(s.Downloads))
		stateCounts := map[DownloadFilter]int{}
		for i, d := range s.Downloads {
			stateCounts[downloadState(d)]++
			if s.downloadMatches(d) && s.downloadSearches(d) {
				visible = append(visible, i)
			}
		}
		if len(s.Downloads) > 0 {
			ui.Row(c).Children(func() {
				caption(c, fmt.Sprintf("%d 个下载任务，按番剧分组", len(visible)))
				ui.Spacer(c)
				pauseLabel := "暂停全部下载"
				if active == 0 {
					pauseLabel = "继续全部下载"
				}
				quiet(c, "pause-all", pauseLabel, func() { s.pauseAllDownloads(active > 0) }).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
			})
			ui.Row(c.Key("download-filter")).Wrap().Gap(controlGap).Children(func() {
				for filter, label := range []string{"全部", "下载中", "已完成", "已暂停", "失败", "已停止"} {
					filter := DownloadFilter(filter)
					count := stateCounts[filter]
					if filter == downloadsAll {
						count = len(s.Downloads)
					}
					button := quiet(c, fmt.Sprintf("download-filter-%d", filter), fmt.Sprintf("%s %d", label, count), func() { s.DownloadFilter = filter }).Label(label).Shrink(0)
					if filter == s.DownloadFilter {
						button.Background(colors(c).blueWash).TextColor(colors(c).blue).FontWeight(600)
					}
				}
			})
		}
		if len(s.Downloads) == 0 {
			s.empty(c, "还没有下载任务", "在番剧详情选择要下载的资源，任务会出现在这里。", "浏览番剧", func() { s.enterPage("browse") })
			return
		}
		if len(visible) == 0 {
			if strings.TrimSpace(s.DownloadSearch) != "" {
				s.empty(c, "没有匹配的下载", "试试缩短关键词，或清除搜索。", "清除搜索", func() { s.clearDownloadSearch() })
				return
			}
			s.empty(c, "这个筛选下没有下载", "换个筛选条件，看看其他下载进度。", "查看全部下载", func() { s.DownloadFilter = downloadsAll })
			return
		}
		type downloadGroup struct {
			animeID  string
			indices  []int
			summary  string
			bytes    int64
			done     int64
			unknown  int
			overflow bool
		}
		var groups []downloadGroup
		byAnime := map[string]int{}
		for _, idx := range visible {
			d := s.Downloads[idx]
			gi, ok := byAnime[d.AnimeID]
			if !ok {
				gi = len(groups)
				byAnime[d.AnimeID] = gi
				groups = append(groups, downloadGroup{animeID: d.AnimeID})
			}
			group := &groups[gi]
			group.indices = append(group.indices, idx)
		}
		for gi := range groups {
			group := &groups[gi]
			slices.SortStableFunc(group.indices, func(a, b int) int {
				ea, oka := episodeNumber(s.Downloads[a].Episode)
				eb, okb := episodeNumber(s.Downloads[b].Episode)
				switch {
				case oka && okb:
					return cmp.Compare(ea, eb)
				case oka:
					return -1
				case okb:
					return 1
				default:
					return 0
				}
			})
			counts := map[DownloadFilter]int{}
			group.bytes, group.done, group.unknown, group.overflow = s.downloadGroupSizes(group.indices)
			for _, idx := range group.indices {
				counts[downloadState(s.Downloads[idx])]++
			}
			parts := []string{fmt.Sprintf("%d 个下载任务", len(group.indices))}
			for _, segment := range []struct {
				filter DownloadFilter
				label  string
			}{{downloadsDownloading, "下载中"}, {downloadsCompleted, "已完成"}, {downloadsPaused, "已暂停"}, {downloadsStopped, "已停止"}, {downloadsFailed, "失败"}} {
				if counts[segment.filter] > 0 {
					parts = append(parts, fmt.Sprintf("%s %d", segment.label, counts[segment.filter]))
				}
			}
			if eta := s.downloadGroupETA(group.indices); eta != "" {
				parts = append(parts, eta)
			}
			group.summary = strings.Join(parts, "，")
		}
		ui.Scroll(c.Key("download-list")).TrackScroll(&s.DownloadScroll).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				for _, g := range groups {
					group := g
					ui.Column(c.Key("download-group-" + group.animeID)).Gap(8).Children(func() {
						collapsed := s.CollapsedDownloads[group.animeID]
						toggle := "收起"
						if collapsed {
							toggle = "展开"
						}
						actions := func() {
							if folder := s.groupFolder(group.animeID); folder != "" {
								quiet(c, "open-group-folder-"+group.animeID, "打开番剧文件夹", func() {
									if s.OpenFolder != nil {
										s.OpenFolder(folder)
									}
								}).FontSize(textSize(c, textCaption)).Disabled(s.OpenFolder == nil)
							}
							quiet(c, "group-detail-"+group.animeID, "查看详情", func() { s.openFollowedResources(group.animeID, "downloads") }).FontSize(textSize(c, textCaption))
							quiet(c, "toggle-group-"+group.animeID, toggle, func() { s.toggleDownloadGroup(group.animeID) }).FontSize(textSize(c, textCaption))
						}
						if textScale(c) > 1 {
							ui.Column(c).Gap(4).Children(func() {
								ui.Text(c, s.anime(group.animeID).Title).FontSize(textSize(c, textSection)).FontWeight(700)
								caption(c, group.summary)
							})
							ui.Row(c).Wrap().Gap(controlGap).Children(actions)
						} else {
							ui.Row(c).Gap(12).Children(func() {
								ui.Text(c, s.anime(group.animeID).Title).FontSize(textSize(c, textSection)).FontWeight(700).Grow(1)
								caption(c, group.summary)
								actions()
							})
						}
						if collapsed {
							return
						}
						if group.overflow {
							caption(c, "总大小超出可显示范围")
						} else if group.bytes > 0 {
							share := float64(group.done) / float64(group.bytes)
							box := ui.Box(c.Key("download-group-progress-" + group.animeID)).Height(4)
							value := box.Animate("value", float32(share), 400*time.Millisecond)
							box.Children(func() { ui.Progress(c, float64(value)).Height(4).Radius(2) })
							totals := "已下载 " + byteSizeText(group.done) + " / " + byteSizeText(group.bytes)
							if group.done == 0 {
								totals = "已下载 0 B / " + byteSizeText(group.bytes)
							}
							if group.unknown > 0 {
								totals = fmt.Sprintf("%s（另有 %d 条大小未知）", totals, group.unknown)
							}
							caption(c, totals)
						} else {
							caption(c, fmt.Sprintf("%d 条大小未知", group.unknown))
						}
						ui.Column(c).Gap(12).Children(func() {
							for _, i := range group.indices {
								idx := i
								d := s.Downloads[idx]
								ui.Column(c.Key("download-"+d.ID)).Padding(16, 18).Gap(9).Background(colors(c).paper).Radius(8).Border(1, colors(c).line).Children(func() {
									ui.Row(c).Gap(12).Children(func() {
										ui.Spacer(c)
										if d.Episode != "" {
											badge(c, episodeLabel(d.Episode), colors(c).muted, colors(c).canvas)
										}
										status := "下载中"
										col := colors(c).blue
										switch {
										case d.Failed:
											status, col = "失败", colors(c).amber
										case d.Stopped && d.Progress < 1:
											status, col = "已停止", colors(c).muted
										case d.Progress >= 1 && d.Stopped:
											status, col = "已完成，已停止分享", colors(c).green
										case d.Paused:
											status, col = "已暂停", colors(c).muted
										case d.Progress >= 1:
											status, col = "已完成，分享中", colors(c).green
										}
										badge(c, status, col, colors(c).canvas)
									})
									ui.Text(c, d.Title).FontSize(textSize(c, textBody)).TextColor(colors(c).muted)
									progress := ui.Box(c.Key("transfer-progress")).Height(5)
									value := progress.Animate("value", float32(d.Progress), 400*time.Millisecond)
									progress.Children(func() { ui.Progress(c, float64(value)).Height(5).Radius(3) })
									metrics := func() {
										ui.Textf(c, "%.0f%%", d.Progress*100).FontSize(textSize(c, textCaption)).FontWeight(600)
										if bytes := sizeBytes(d.Bytes, d.MB); bytes > 0 {
											done := byteSizeText(downloadDoneBytes(d))
											if downloadDoneBytes(d) == 0 {
												done = "0 B"
											}
											caption(c, done+" / "+byteSizeText(bytes))
										} else {
											caption(c, "大小未知")
										}
										speeds := speedText(d.DownSpeed, false) + "   " + speedText(d.UpSpeed, true)
										if eta := downloadETA(d); eta != "" {
											speeds += "   " + eta
										}
										caption(c, speeds)
									}
									actions := func() {
										if d.Failed {
											quiet(c, "retry-"+d.ID, "重试", func() { s.retryDownload(idx) }).TextColor(colors(c).blue).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
										} else if d.Paused || d.Stopped && d.Progress < 1 {
											quiet(c, "pause-"+d.ID, "继续", func() { s.pauseDownload(idx) }).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
										} else if d.Progress < 1 {
											quiet(c, "pause-"+d.ID, "暂停", func() { s.pauseDownload(idx) }).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
										} else if !d.Stopped {
											quiet(c, "stop-seed-"+d.ID, "停止分享", func() { s.stopDownload(idx) }).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
										}
										quiet(c, "remove-"+d.ID, "删除", func() { s.RemovePrompt, s.RemoveID = true, d.ID }).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
										quiet(c, "open-folder-"+d.ID, "打开文件夹", func() {
											if s.OpenFolder != nil {
												s.OpenFolder(d.Folder)
											}
										}).FontSize(textSize(c, textCaption)).Disabled(s.OpenFolder == nil || d.Folder == "")
										if d.Progress >= 1 && !d.Failed {
											quiet(c, "files-"+d.ID, "查看文件", func() { s.openFiles(idx) }).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
											quiet(c, "play-"+d.ID, "播放", func() { s.playDownload(idx) }).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue).Disabled(s.Busy)
										}
									}
									if textScale(c) > 1 {
										ui.Row(c).Wrap().Gap(8).Children(metrics)
										ui.Row(c).Wrap().Gap(controlGap).Children(actions)
									} else {
										ui.Row(c).Gap(14).Children(func() {
											metrics()
											ui.Spacer(c)
											actions()
										})
									}
									caption(c, "保存位置："+d.Folder)
									if d.Progress >= 1 && !d.Stopped {
										uploaded := d.UploadedMB * 1e6
										if d.UploadedBytes != 0 {
											uploaded = float64(d.UploadedBytes)
										}
										bytes := d.DownloadedBytes
										if bytes == 0 {
											bytes = sizeBytes(d.Bytes, d.MB)
										}
										if bytes > 0 && uploaded >= 0 {
											caption(c, fmt.Sprintf("分享率 %.2f，%s", uploaded/float64(bytes), seedTimeText(d.SeedSeconds)))
										} else {
											caption(c, "大小未知，分享率未知，"+seedTimeText(d.SeedSeconds))
										}
									}
									if d.Error != "" {
										ui.Text(c, d.Error).FontSize(textSize(c, textCaption)).TextColor(colors(c).amber)
									} else if d.Failed {
										ui.Text(c, "下载失败，请检查网络后重试。").FontSize(textSize(c, textCaption)).TextColor(colors(c).amber)
									}
								})
							}
						})
					})
				}
			})
		})
		caption(c, "全局上传上限 "+s.Upload+"；完成后"+s.seedStopRule()+"。")
	})
}

func (s *State) settings(c *ui.Context) {
	ui.Scroll(c.Key("settings-scroll")).Grow(1).MinHeight(0).Padding(26, 28).Children(func() {
		ui.Column(c).MaxWidth(800).Gap(22).Children(func() {
			ui.Text(c, "设置").FontSize(textSize(c, textTitle)).FontWeight(700)
			ui.Column(c).Gap(12).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
				heading(c, "外观与动效")
				s.appearanceSettings(c)
				ui.Checkbox(c.Key("reduce-effects"), &s.ReduceEffects, "减少动态效果").OnChange(s.save).Disabled(s.Busy)
				caption(c, "减少界面动画和弹层过渡；也会遵循系统设置。")
			})
			ui.Column(c).Gap(14).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
				heading(c, "下载位置")
				caption(c, "每部番剧单独一个文件夹，保留原文件名。")
				ui.Row(c).Gap(12).Children(func() {
					ui.Text(c, s.Root).FontSize(textSize(c, textBody)).Selectable().Grow(1).MinWidth(0)
					secondary(c, "choose-folder", "选择文件夹", func() {
						if s.PickFolder != nil {
							s.PickFolder()
						}
					}).FontSize(textSize(c, textCaption)).Disabled(s.Busy || s.PickFolder == nil)
				})
				caption(c, "只影响新追番；已有番剧仍保存在原文件夹。")
			})
			ui.Column(c).Gap(14).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
				heading(c, "番剧来源")
				ui.Row(c).Gap(16).Children(func() {
					ui.Column(c).Grow(1).Gap(5).Children(func() {
						ui.Text(c, "蜜柑计划线路").FontSize(textSize(c, textBody))
						caption(c, "两条线路内容相同。切换前会检查；不可用则保留原线路。")
					})
					ui.Select(c.Key("source-route"), &s.SourceRoute, s.routeLabels()).Label("蜜柑计划线路").Width(260).OnChange(func() { s.switchSourceRoute(s.SourceRoute) }).Disabled(s.Busy || s.SourceTesting)
				})
				if s.SourceTesting {
					caption(c, "正在测试新线路…")
				} else if s.SourceRouteError != "" {
					ui.Text(c, s.SourceRouteError).FontSize(textSize(c, textCaption)).TextColor(colors(c).errorInk)
				}
			})
			ui.Column(c).Gap(14).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
				heading(c, "上传与分享")
				ui.Row(c).Wrap().Gap(16).Children(func() {
					ui.Column(c).Grow(1).MinWidth(textSize(c, textBody) * 12).Gap(5).Children(func() {
						ui.Text(c, "全局上传上限").FontSize(textSize(c, textBody))
						caption(c, "所有任务共享此上限。1 MiB = 1,024 KiB。")
					})
					ui.Select(c.Key("upload-limit"), &s.Upload, uploadChoices(s.Upload)).Label("全局上传限速").Width(textSize(c, textBody) * 10).MinHeight(inputHeight(c)).OnChange(s.save).Disabled(s.Busy)
				})
				ui.Row(c).Wrap().Gap(16).Children(func() {
					ui.Column(c).Grow(1).MinWidth(textSize(c, textBody) * 12).Gap(5).Children(func() {
						ui.Text(c, "分享时长").FontSize(textSize(c, textBody))
						caption(c, "下载完成后把文件继续提供给其他人；暂停或离线时不计时。")
					})
					ui.Select(c.Key("seed-duration"), &s.Seed, seedOptions).Label("完成后分享时长").Width(textSize(c, textBody) * 10).MinHeight(inputHeight(c)).OnChange(s.save).Disabled(s.Busy)
				})
				ui.Row(c).Wrap().Gap(16).Children(func() {
					ui.Column(c).Grow(1).MinWidth(textSize(c, textBody) * 12).Gap(5).Children(func() {
						ui.Text(c, "分享率上限").FontSize(textSize(c, textBody))
						caption(c, "分享率是上传量除以下载量；时间或分享率任一达到上限，就停止分享。")
					})
					ui.Select(c.Key("seed-ratio"), &s.SeedRatio, ratioOptions).Label("分享率上限").Width(textSize(c, textBody) * 10).MinHeight(inputHeight(c)).OnChange(s.save).Disabled(s.Busy)
				})
				ui.Column(c).Gap(5).Children(func() {
					ui.Row(c).Wrap().Gap(16).Children(func() {
						ui.Column(c).Grow(1).MinWidth(textSize(c, textBody) * 12).Gap(5).Children(func() {
							ui.Text(c, "追加 Tracker").FontSize(textSize(c, textBody))
							caption(c, "每行一个 udp:// 或 http(s):// 地址，按回车或保存后生效；无效的行会被拒绝。")
						})
						secondary(c, "save-trackers", "保存", s.save).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
					})
					ui.TextInput(c.Key("extra-trackers"), &s.ExtraTrackers).Label("追加 Tracker").Placeholder("udp://tracker.example.com:1337/announce").FillWidth().MinHeight(inputHeight(c)).OnChange(func() { s.TrackerFeedback = "" }).OnSubmit(s.save)
					if s.TrackerFeedback != "" {
						caption(c, s.TrackerFeedback)
					}
					caption(c, "每个任务都会自动追加这些内置 Tracker："+builtinTrackerSummary()+"。")
				})
				caption(c, "下载完成后，"+s.seedStopRule()+"。")
			})
			ui.Column(c).Gap(14).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
				heading(c, "网络与代理")
				ui.Row(c).Gap(16).Children(func() {
					ui.Column(c).Grow(1).Gap(5).Children(func() {
						ui.Text(c, "代理").FontSize(textSize(c, textBody))
						caption(c, s.proxyDescription())
					})
					ui.Segmented(c.Key("proxy-mode"), &s.ProxyMode, "自动", "关闭", "自定义").OnChange(func() {
						s.ProxyFeedback = ""
						s.save()
					}).Disabled(s.Busy)
					secondary(c, "test-proxy", "测试", func() { s.testProxy() }).FontSize(textSize(c, textCaption)).Disabled(s.Busy || s.ProxyTesting || s.Source == nil)
				})
				if s.ProxyMode == 2 {
					ui.TextInput(c.Key("proxy-url"), &s.ProxyURL).Label("代理地址").Placeholder("http://127.0.0.1:7897 或 socks5://…").FillWidth().MinHeight(inputHeight(c)).OnChange(func() {
						s.ProxyFeedback = ""
					}).OnSubmit(func() {
						s.save()
					})
					caption(c, "支持 http、https、socks5；也可只填 host:port。输入地址后按回车保存。")
				}
				if s.ProxyFeedback != "" {
					caption(c, s.ProxyFeedback)
				}
			})
			s.windowSettings(c)
		})
	})
}

func (s *State) filesDialog(c *ui.Context) {
	ui.DialogBase(c, &s.FilesOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(colors(c).scrim).Transition(fadeTransition)
		dialogContentWithFooter(c, panel, "files-content", 560, func() {
			var d Download
			for _, v := range s.Downloads {
				if v.ID == s.FilesID {
					d = v
					break
				}
			}
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, d.Title).FontSize(textSize(c, textSection)).FontWeight(700).Grow(1).MinWidth(0)
				iconButton(c, "close-files", "关闭文件列表", "close", func() { s.FilesOpen = false })
			})
			switch {
			case s.FilesError != "":
				caption(c, s.FilesError)
			case s.FilesLoading:
				caption(c, "正在读取文件…")
			case len(s.Files) == 0:
				caption(c, "没有找到文件，可能已被移动或删除。")
			default:
				ui.Column(c).Gap(8).Children(func() {
					for _, f := range s.Files {
						name := filepath.Base(f.Path)
						dir := filepath.Dir(f.Path)
						ui.Row(c).Wrap().Gap(controlGap).Padding(10, 12).Radius(8).Background(colors(c).canvas).Children(func() {
							ui.Column(c).FillWidth().MinWidth(0).Gap(3).Children(func() {
								ui.Text(c, name).FontSize(textSize(c, textBody)).FontWeight(600)
								caption(c, dir)
								caption(c, exactByteText(f.Size))
							})
							quiet(c, "copy-path-"+name, "复制路径", func() { s.copyPath(f.Path) }).FontSize(textSize(c, textCaption))
							if isPlayableMedia(f.Path) {
								quiet(c, "play-file-"+name, "播放此文件", func() { s.playFile(f.Path) }).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue)
							}
						})
					}
				})
			}
			caption(c, "已保存位置："+d.Folder)
		}, func() {
			secondary(c, "close-files-footer", "关闭", func() { s.FilesOpen = false })
		})
	})
}

func (s *State) playFile(path string) {
	if s.PlayFile == nil {
		return
	}
	if err := s.PlayFile(path); err != nil && s.FilesError == "" {
		s.FilesError = err.Error()
	}
}

func (s *State) copyPath(path string) {
	s.FilesError = ""
	if s.CopyText != nil {
		s.CopyText(path)
	}
}

func (s *State) helpDialog(c *ui.Context) {
	ui.DialogBase(c, &s.Help, func(backdrop, panel ui.Element) {
		backdrop.Background(colors(c).scrim).Transition(fadeTransition)
		dialogContent(c, panel, "help-content", 500, func() {
			ui.Row(c).Children(func() {
				ui.Text(c, "怎么追番").FontSize(textSize(c, textTitle)).FontWeight(700).Grow(1).MinWidth(0)
				iconButton(c, "close-help", "关闭指南", "close", func() { s.Help = false })
			})
			for i, step := range []struct{ title, body string }{{"找到想看的番剧", "按季度和星期浏览，点海报打开番剧。"}, {"选择字幕组和版本", "用标题里的「1080p」「简日内嵌」等词挑选。"}, {"开启追番", "以后发布的资源会自动下载。在「我的追番」点海报，从剧集库播放下载完成的剧集。"}} {
				ui.Row(c).Gap(14).AlignItems(ui.Start).Children(func() {
					ui.Textf(c, "%d", i+1).FontSize(textSize(c, textSection)).FontWeight(600).TextColor(colors(c).blue).Background(colors(c).blueWash).Padding(7, 12).Radius(20)
					ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() { ui.Text(c, step.title).FontSize(textSize(c, textBody)).FontWeight(600); caption(c, step.body) })
				})
			}
			primary(c, "got-it", "关闭指南", func() { s.Help = false }).AlignSelf(ui.End)
		})
	})
}

func (s *State) titlePreferences(c *ui.Context) {
	ui.Column(c.Key("title-preferences")).FillWidth().Gap(12).Transition(rowTransition).Children(func() {
		ui.Text(c, "按此资源筛选").FontSize(textSize(c, textBody)).FontWeight(600)
		caption(c, "为每个标签选择「只要」或「不要」，结果会立即更新。")
		ui.Column(c).Gap(14).Children(func() {
			for i, word := range s.SuggestedResource.Tokens {
				w := word
				ui.Column(c).Gap(6).Children(func() {
					ui.Text(c, w).FontSize(textSize(c, textBody)).FontWeight(600)
					ui.Row(c).Gap(4).Children(func() {
						for _, option := range []struct {
							label     string
							choice    wordChoice
							color, bg ui.Color
						}{{"不限", wordUnrestricted, colors(c).ink, colors(c).canvas}, {"＋ 只要", wordWant, colors(c).blue, colors(c).blueWash}, {"− 不要", wordAvoid, colors(c).avoidInk, colors(c).avoidWash}} {
							o := option
							b := quiet(c, fmt.Sprintf("suggested-%d-%d", i, o.choice), o.label, func() { s.setWord(w, o.choice) }).Label(o.label+" "+w).Grow(1).Justify(ui.Center).FontSize(textSize(c, textCaption)).Padding(controlPaddingY, controlPaddingX).Radius(controlRadius).Border(1, colors(c).line)
							if s.wordChoice(w) == o.choice {
								b.Background(o.bg).TextColor(o.color).BorderColor(o.color)
							}
						}
					})
				})
			}
		})
		secondary(c, "close-suggestions", "完成筛选", func() { s.Suggestions = false }).FillWidth()
	})
}

// follows keeps the subscription when the follow search matches its anime
// title, subtitle group, or a word the saved rule requires.
func (s *State) follows(sub Subscription) bool {
	query := normalize(strings.TrimSpace(s.FollowSearch))
	if query == "" {
		return true
	}
	a := s.anime(sub.AnimeID)
	if strings.Contains(normalize(a.Title), query) {
		return true
	}
	if strings.Contains(normalize(sub.GroupName), query) {
		return true
	}
	for _, w := range sub.Rule.All {
		if strings.Contains(normalize(w), query) {
			return true
		}
	}
	return false
}
