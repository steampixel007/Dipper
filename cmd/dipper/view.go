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

var (
	coversMu sync.Mutex
	covers   = map[string]*ui.Bitmap{}
)

var (
	brandLogo *ui.SVG
	brandApp  *ui.Bitmap
)

func loadImages() {
	loadFonts()
	loadIcons()
	brandLogo = ui.MustParseSVG(brandSVG)
	var err error
	if brandApp, err = ui.DecodeBitmap(brandIconPNG); err != nil {
		panic("decode app icon: " + err.Error())
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

func quiet(c *ui.Context, key, label string, action func()) ui.Element {
	pal := colors(c)
	b := ui.ButtonBase(c.Key(key)).
		MinHeight(buttonHeight(c)).
		Padding(controlPaddingY, controlPaddingX).
		Radius(radiusControl).
		Gap(controlGap).
		AlignItems(ui.Center).
		FontSize(textSize(c, textControl)).
		FontWeight(500).
		TextColor(pal.ink).
		Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(pal.chrome)
	}
	return tactile(b).Children(func() { ui.Text(c, label) }).OnClick(action)
}

func compactQuiet(c *ui.Context, key, label string, action func()) ui.Element {
	pal := colors(c)
	b := ui.ButtonBase(c.Key(key)).
		Padding(compactPaddingY, compactPaddingX).
		Radius(radiusSmall).
		Gap(controlGap).
		AlignItems(ui.Center).
		FontSize(textSize(c, textControl)).
		FontWeight(500).
		TextColor(pal.ink).
		Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(pal.chrome)
	}
	return tactile(b).Children(func() { ui.Text(c, label) }).OnClick(action)
}

func secondary(c *ui.Context, key, label string, action func()) ui.Element {
	pal := colors(c)
	return tactile(ui.Button(c.Key(key), label)).
		MinHeight(buttonHeight(c)).
		Radius(radiusControl).
		Padding(controlPaddingY, controlPaddingX).
		Gap(controlGap).
		FontSize(textSize(c, textControl)).
		FontWeight(500).
		Background(pal.surface).
		Border(1, pal.outline).
		TextColor(pal.ink).
		OnClick(action)
}

func compactSecondary(c *ui.Context, key, label string, action func()) ui.Element {
	pal := colors(c)
	return tactile(ui.Button(c.Key(key), label)).
		Radius(radiusSmall).
		Padding(compactPaddingY, compactPaddingX).
		Gap(controlGap).
		FontSize(textSize(c, textControl)).
		FontWeight(500).
		Background(pal.surface).
		Border(1, pal.outline).
		TextColor(pal.ink).
		OnClick(action)
}

func primary(c *ui.Context, key, label string, action func()) ui.Element {
	return tactile(ui.PrimaryButton(c.Key(key), label)).
		MinHeight(buttonHeight(c)).
		Radius(radiusControl).
		Padding(controlPaddingY, controlPaddingX).
		Gap(controlGap).
		FontSize(textSize(c, textControl)).
		FontWeight(500).
		OnClick(action)
}

func iconButton(c *ui.Context, key, label, name string, action func()) ui.Element {
	pal := colors(c)
	b := ui.ButtonBase(c.Key(key)).
		Label(label).Tooltip(label).
		Size(40, 40).Shrink(0).Center().
		Radius(radiusControl).
		TextColor(pal.ink).
		Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(pal.chrome)
	}
	return tactile(b).Children(func() { icon(c, name) }).OnClick(action)
}

func windowControl(c *ui.Context, key, label, name string, danger bool, action func()) ui.Element {
	pal := colors(c)
	b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Size(48, 32).Shrink(0).Center().Cursor(ui.CursorPointer).TextColor(pal.ink)
	if b.Hovered() {
		b.Background(pal.chrome)
		if danger {
			b.Background(ui.Hex("#C42B1C")).TextColor(ui.Hex("#FFFFFF"))
		}
	}
	return tactile(b).Children(func() { icon(c, name) }).OnClick(action)
}

func pageTitle(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(textSize(c, textTitle)).FontWeight(700).LineHeight(1.35).TextColor(colors(c).ink)
}

func heading(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(textSize(c, textSection)).FontWeight(600).LineHeight(1.35).TextColor(colors(c).ink)
}

func caption(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(textSize(c, textMeta)).FontWeight(400).LineHeight(1.5).TextColor(colors(c).muted)
}

func statusPill(c *ui.Context, text string, col, bg ui.Color) ui.Element {
	return ui.Text(c, text).FontSize(textSize(c, textControl)).FontWeight(500).TextColor(col).Background(bg).Radius(radiusSmall).Padding(4, 8)
}

func (s *State) featuredCover(c *ui.Context, a Anime, w, h float32) {
	key := coverURL(a.Cover)
	if key != "" {
		s.coverLoader.featured = append(s.coverLoader.featured, key)
	}
	drawCover(c, cachedCover(key), key != "", coverCanLoad(s.coverLoader.stages[key]), w, h, coverRadius)
}

func (s *State) posterCover(c *ui.Context, tile ui.Element, a Anime, width float32) {
	key := coverURL(a.Cover)
	visible := tile.InView()
	if key != "" && visible {
		s.coverLoader.visible = append(s.coverLoader.visible, key)
	} else if key != "" && tile.InViewMargin(coverLookbehind, 0, coverLookahead, 0) {
		s.coverLoader.nearby = append(s.coverLoader.nearby, key)
	}
	height := width * 1.36
	bitmap := cachedCover(key)
	if ratio := s.coverLoader.ratios[key]; ratio > 0 {
		height = width * ratio
	} else if bitmap != nil {
		imageWidth, imageHeight := bitmap.Size()
		if imageWidth > 0 && imageHeight > 0 {
			height = width * float32(imageHeight) / float32(imageWidth)
		}
	}
	showLoading := (visible || s.Source == nil) && coverCanLoad(s.coverLoader.stages[key])
	drawCover(c, bitmap, key != "", showLoading, width, height, coverRadius)
}

func coverCanLoad(stage coverStage) bool {
	return stage != coverMiss && stage != coverFailed
}

func drawCover(c *ui.Context, b *ui.Bitmap, hasURL, showLoading bool, w, h, radius float32) {
	pal := colors(c)
	if b != nil {
		ui.Image(c, b).Size(w, h).Fit(ui.Contain).Background(pal.sunken).Radius(radius)
	} else if hasURL {
		ui.Box(c).Size(w, h).Radius(radius).Clip().Background(pal.sunken).Center().Children(func() {
			if showLoading {
				loading.Blocks(c, loading.Options{Size: 24, Color: pal.muted}).Label("正在加载封面")
			}
		})
	} else {
		size := min(w, h) * 0.62
		ui.Box(c).Size(w, h).Radius(radius).Clip().Background(pal.sunken).Center().Children(func() {
			ui.Image(c, brandLogo).Size(size, size).Background(pal.brandWash).Radius(radiusSmall)
		})
	}
}

func (s *State) view(c *ui.Context) {
	loader := &s.coverLoader
	loader.featured, loader.visible, loader.nearby = loader.featured[:0], loader.visible[:0], loader.nearby[:0]
	defer s.syncCovers()
	installTheme(c, s.darkAppearance(c.Theme().Dark))
	if s.ReduceEffects || c.Preferences().ReduceMotion {
		c.DisableAnimations()
	}
	pal := colors(c)
	ui.Overlay(c, func() {})
	ui.Row(c).Fill().Gap(0).Background(pal.canvas).Children(func() {
		s.sidebar(c)
		ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
			s.topbar(c)
			if s.SaveError != "" {
				ui.Text(c, s.SaveError).FontSize(textSize(c, textMeta)).TextColor(pal.errorInk).Background(pal.errorWash).Padding(12, pageInsetX(c)).LineHeight(1.5)
			}
			s.softwareNotice(c)
			ui.Box(c.Key("page-host")).Grow(1).MinHeight(0).FillWidth().Children(func() {
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
	if s.Page == "browse" && s.browseDetail != nil {
		if browseDetailWidth(c) == 0 {
			s.browseDetailDialog(c)
		} else {
			c.OnShortcut(0, ui.KeyEscape, s.closeBrowseDetail)
		}
	}
	s.cancelDialog(c)
	s.removeDialog(c)
	s.helpDialog(c)
	s.closeDialog(c)
	s.filesDialog(c)
	s.trackerDialog(c)
	s.softwareUpdateDialog(c)
	if s.Page == "detail" && !s.Help && !s.ClosePrompt {
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
	pal := colors(c)
	ui.Scroll(c.Key("sidebar-scroll")).Width(sidebarWidth).FillHeight().Background(pal.chrome).Children(func() {
		ui.Column(c).MinHeight(height).Padding(24, 16).Gap(4).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Padding(0, 4, 24).Children(func() {
				ui.Image(c, brandApp).Size(40, 40)
				ui.Text(c, "Dipper").FontSize(textSize(c, textSection)).FontWeight(600).TextColor(pal.ink).LineHeight(1.35).Grow(1).MinWidth(0)
			})
			for _, nav := range []struct{ id, label string }{{"browse", "浏览番剧"}, {"following", "我的追番"}, {"downloads", "下载"}} {
				current := nav
				active := s.Page == nav.id || (s.Page == "detail" && nav.id == "browse") || (s.Page == "episodes" && nav.id == "following")
				b := ui.ButtonBase(c.Key("nav-"+nav.id)).Label(current.label).FillWidth().MinHeight(44).Padding(12, 16).Gap(12).AlignItems(ui.Center).Radius(radiusControl).Cursor(ui.CursorPointer)
				tactile(b)
				if active {
					b.Background(pal.accentWash).TextColor(pal.accentInk).FontWeight(500)
				} else {
					b.TextColor(pal.ink).FontWeight(500)
					if b.Hovered() {
						b.Background(pal.sunken)
					}
				}
				b.Children(func() {
					if active {
						ui.Box(c).Absolute().Left(4).Top(20).Size(4, 4).Radius(2).Background(pal.accent)
					}
					icon(c, current.id)
					labels := ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(8)
					if textScale(c) > 1 {
						labels.Column().AlignItems(ui.Start).Gap(4)
					}
					labels.Children(func() {
						ui.Text(c, current.label).FontSize(textSize(c, textControl)).FontWeight(500)
						if textScale(c) <= 1 {
							ui.Spacer(c)
						}
						if current.id == "following" && len(s.Subscriptions) > 0 {
							failed := 0
							for _, sub := range s.Subscriptions {
								if sub.CheckFailed {
									failed++
								}
							}
							if failed > 0 {
								ui.Textf(c, "%d 失败", failed).FontSize(textSize(c, textMeta)).FontWeight(500).TextColor(pal.warningInk)
							} else {
								ui.Textf(c, "%d", len(s.Subscriptions)).FontSize(textSize(c, textMeta)).TextColor(pal.muted)
							}
						}
						if current.id == "downloads" {
							count, failed := 0, 0
							for _, d := range s.Downloads {
								if !downloadInQueue(d) {
									continue
								}
								count++
								if d.Failed {
									failed++
								}
							}
							if failed > 0 {
								ui.Textf(c, "%d 失败", failed).FontSize(textSize(c, textMeta)).FontWeight(500).TextColor(pal.warningInk)
							} else if count > 0 {
								ui.Textf(c, "%d", count).FontSize(textSize(c, textMeta)).TextColor(pal.muted)
							}
						}
					})
				}).OnClick(func() { s.enterPage(current.id); s.Notice = "" })
			}
			ui.Spacer(c)
			settingsActive := s.Page == "settings"
			b := ui.ButtonBase(c.Key("nav-settings")).Label("设置").FillWidth().MinHeight(44).Padding(12, 16).Gap(12).Justify(ui.Start).AlignItems(ui.Center).Radius(radiusControl).Cursor(ui.CursorPointer)
			tactile(b)
			if settingsActive {
				b.Background(pal.accentWash).TextColor(pal.accentInk).FontWeight(500)
			} else {
				b.TextColor(pal.ink).FontWeight(500)
				if b.Hovered() {
					b.Background(pal.sunken)
				}
			}
			b.Children(func() {
				if settingsActive {
					ui.Box(c).Absolute().Left(4).Top(20).Size(4, 4).Radius(2).Background(pal.accent)
				}
				icon(c, "settings")
				ui.Text(c, "设置").FontSize(textSize(c, textControl)).FontWeight(500)
			}).OnClick(func() { s.enterPage("settings"); s.Notice = "" })
			compactQuiet(c, "help", "追番小指南", func() { s.Help = true }).TextColor(pal.muted).AlignSelf(ui.Start)
		})
	})
}

func (s *State) topbar(c *ui.Context) {
	pal := colors(c)
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
	rightPadding := bar.Right + 24
	if s.CustomWindowControls {
		rightPadding = 0
	}
	ui.Row(c).MinHeight(topbarHeight).Shrink(0).AlignItems(ui.Center).Padding(12, rightPadding, 12, pageInsetX(c)).Gap(12).
		Background(pal.canvas).
		DragWindow().Children(func() {
		ui.Text(c, names[s.Page]).FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.muted).MinWidth(0).Grow(1)
		iconButton(c, "top-help", "打开追番指南", "help", func() { s.Help = true })
		if s.CustomWindowControls {
			maximized := s.WindowIsMaximized != nil && s.WindowIsMaximized()
			ui.Row(c).Shrink(0).Gap(0).Children(func() {
				windowControl(c, "window-minimize", "最小化窗口", "minimize", false, func() {
					if s.WindowMinimize != nil {
						s.WindowMinimize()
					}
				})
				maximizeLabel, maximizeIcon := "最大化窗口", "maximize"
				if maximized {
					maximizeLabel, maximizeIcon = "还原窗口", "restore"
				}
				maximize := windowControl(c, "window-maximize", maximizeLabel, maximizeIcon, false, func() {
					if s.WindowMaximize != nil {
						s.WindowMaximize()
					}
				})
				if !maximized {
					maximize.MaximizeButton()
				}
				windowControl(c, "window-close", "关闭窗口", "close", true, func() {
					if s.WindowClose != nil {
						s.WindowClose()
					}
				})
			})
		}
	})
}

func (s *State) browse(c *ui.Context) {
	windowWidth, _ := c.Size()
	detailWidth := float32(0)
	if s.browseDetail != nil {
		detailWidth = browseDetailWidth(c)
	}
	pal := colors(c)
	ui.Row(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
		s.browseCatalog(c, windowWidth-sidebarWidth-detailWidth, detailWidth > 0)
		if detailWidth > 0 {
			ui.Column(c).Width(detailWidth).Shrink(0).FillHeight().MinHeight(0).
				Background(pal.surface).BorderWidth(0, 0, 0, 1).BorderColor(pal.outline).
				Children(func() { s.browseDetailContent(c, s.browseDetail) })
		}
	})
}

func (s *State) browseCatalog(c *ui.Context, width float32, split bool) {
	pal := colors(c)
	insetX := pageInsetX(c)
	topPad := float32(36)
	if textScale(c) > 1.5 {
		topPad = 24
	}
	ui.Column(c).Width(width).Shrink(0).FillHeight().MinHeight(0).Padding(topPad, insetX, 0).Children(func() {
		pageTitle(c, s.Year+" 年"+s.Season+"季番剧").Margin(0, 0, 16, 0).ContentTransition(ui.ContentTransition{Animation: &contentMotion})
		if s.SourceError != "" && !s.catalogFromDisk && len(s.Catalog) == 0 {
			ui.Row(c).Wrap().Gap(controlGap).Margin(0, 0, 16, 0).Children(func() {
				compactQuiet(c, "catalog-error-previous", "上一季", func() { s.changeSeason(-1) }).Disabled(!s.canChangeSeason(-1))
				compactQuiet(c, "catalog-error-next", "下一季", func() { s.changeSeason(1) }).Disabled(!s.canChangeSeason(1))
				compactQuiet(c, "catalog-error-current", "回到当季", s.currentSeason)
			})
			s.empty(c, "来源暂时无法连接", s.SourceError, "重试加载", s.refreshCatalog)
			return
		}
		s.browseToolbar(c, split)
		s.browseDays(c)
		if s.SourceError != "" && len(s.Catalog) > 0 {
			ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Margin(0, 0, 16, 0).Children(func() {
				ui.Text(c, "正在显示上次加载的番剧").FontSize(textSize(c, textMeta)).TextColor(pal.warningInk)
				compactQuiet(c, "catalog-cache-retry", "重试加载", s.refreshCatalog).TextColor(pal.accentInk)
			})
		}
		if s.SourceLoading && !s.catalogFromDisk && len(s.Catalog) == 0 {
			pageLoading(c, "正在加载番剧目录…")
		} else {
			var filtered []Anime
			for _, a := range s.Catalog {
				if (s.Day < 0 || a.Day == s.Day) && strings.Contains(normalize(a.Title), normalize(strings.TrimSpace(s.Search))) {
					filtered = append(filtered, a)
				}
			}
			if len(filtered) == 0 {
				title, body := "没有找到这部番剧", "缩短番名或切换全部星期。"
				if strings.TrimSpace(s.Search) == "" {
					title = "暂无番剧"
					body = "当前季度或星期暂无番剧。"
				}
				s.empty(c, title, body, "清除筛选", func() { s.Search = ""; s.Day = -1; s.resetBrowseScroll() })
			} else {
				cols, cardWidth := gridColumnsForInner(width - insetX*2)
				key, scroll := "catalog", &s.CatalogScroll
				if split {
					key, scroll = "catalog-split", &s.browseDetail.catalogScroll
				}
				ui.Scroll(c.Key(key)).TrackScroll(scroll).Grow(1).MinHeight(0).Padding(0, 0, 32).Children(func() {
					ui.Grid(c).Columns(cols).GapX(posterGapX).GapY(posterGapY).Children(func() {
						for _, a := range filtered {
							card := s.animeCard(c, a, cardWidth)
							if split && s.browseDetail.anchorID == a.ID {
								card.ScrollIntoView()
								s.browseDetail.anchorID = ""
							}
						}
					})
				})
			}
		}
	})
}

func (s *State) browseToolbar(c *ui.Context, split bool) {
	pal := colors(c)
	windowWidth, _ := c.Size()
	year, _ := strconv.Atoi(s.Year)
	firstSeason := firstCatalogSeasonIndex(year)
	searchWidth := max(float32(224), textSize(c, textControl)*16)
	if split || windowWidth <= 1080 {
		searchWidth = max(180, textSize(c, textControl)*13)
	}
	ui.Box(c).FillWidth().Margin(0, 0, 16, 0).Children(func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(func() {
			iconButton(c, "prev-season", "上一季", "back", func() { s.changeSeason(-1) }).Disabled(!s.canChangeSeason(-1))
			ui.Select(c.Key("year"), &s.Year, catalogYears()).Label("年份").Width(max(88, textSize(c, textControl)*6)).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).OnChange(s.changeCatalogYear)
			ui.Row(c).Gap(4).Padding(4).Radius(radiusControl).Background(pal.sunken).Children(func() {
				for index, season := range seasons {
					q := season
					active := s.Season == q
					b := ui.ButtonBase(c.Key("season-"+q)).Label(q+"季").Padding(8, 12).Radius(radiusSmall).Center().FontSize(textSize(c, textControl)).FontWeight(500).Cursor(ui.CursorPointer).TextColor(pal.muted)
					if active {
						b.Background(pal.surface).TextColor(pal.ink)
					} else if b.Hovered() {
						b.TextColor(pal.ink)
					}
					tactile(b).Children(func() { ui.Text(c, q+"季") }).OnClick(func() { s.Season = q; s.refreshCatalog() }).Disabled(index < firstSeason)
				}
			})
			iconButton(c, "next-season", "下一季", "next", func() { s.changeSeason(1) }).Disabled(!s.canChangeSeason(1))
			ui.Spacer(c)
			if ui.TextInput(c.Key("search"), &s.Search).Label("搜索番剧").Placeholder("搜索本季番剧").Width(searchWidth).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).Changed() {
				s.resetBrowseScroll()
			}
			if strings.TrimSpace(s.Search) != "" {
				compactQuiet(c, "clear-browse-search", "清除搜索", func() { s.Search = ""; s.resetBrowseScroll() }).TextColor(pal.accentInk)
			}
		})
	})
}

func (s *State) browseDays(c *ui.Context) {
	pal := colors(c)
	bottomMargin := float32(24)
	if textScale(c) > 1.5 {
		bottomMargin = 16
	}
	ui.Row(c).Wrap().AlignItems(ui.Center).Gap(4).Margin(0, 0, bottomMargin, 0).Children(func() {
		for i, label := range []string{"全部", "周一", "周二", "周三", "周四", "周五", "周六", "周日"} {
			day := i
			if i == 0 {
				day = -1
			}
			if i == 7 {
				day = 0
			}
			active := s.Day == day
			b := ui.ButtonBase(c.Key(fmt.Sprintf("day-%d", i))).Label(label).Padding(8, 12).Radius(radiusSmall).FontSize(textSize(c, textControl)).FontWeight(500).Cursor(ui.CursorPointer).TextColor(pal.muted)
			if active {
				b.TextColor(pal.ink)
			} else if b.Hovered() {
				b.TextColor(pal.ink)
			}
			b.Draw(func(painter *ui.Painter, r ui.Rect) {
				if active {
					painter.Fill(ui.Rect{X: r.X + 12, Y: r.Y + r.H - 2, W: r.W - 24, H: 2}, pal.accent, 1)
				}
			})
			tactile(b).Children(func() { ui.Text(c, label) }).OnClick(func() { s.Day = day; s.resetBrowseScroll() })
		}
		ui.Spacer(c)
		compactQuiet(c, "current-season", "回到当季", s.currentSeason).TextColor(pal.muted)
	})
}

func (s *State) animeCard(c *ui.Context, a Anime, width float32) ui.Element {
	pal := colors(c)
	b := ui.ButtonBase(c.Key("anime-" + a.ID)).Label("查看 " + a.Title).Width(width).Column().Justify(ui.Start).Gap(12).Padding(0).AlignItems(ui.Stretch).Radius(coverRadius).Cursor(ui.CursorPointer)
	selected := s.browseDetail != nil && s.browseDetail.anime.ID == a.ID
	b.Expanded(selected)
	return b.Children(func() {
		ui.Box(c).Radius(coverRadius).Children(func() {
			s.posterCover(c, b, a, width)
			if s.isFollowingAnime(a.ID) {
				statusPill(c, "已追番", pal.ink, pal.surface).Border(1, pal.line).Absolute().Top(8).Left(8)
			}
			if selected {
				ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).Radius(coverRadius).Border(2, pal.accent)
			}
			if b.Hovered() || b.FocusVisible() {
				ui.Row(c).Absolute().Bottom(8).Left(8).Right(8).Center().Gap(4).
					Padding(8, 12).Radius(radiusSmall).Background(pal.surface).Border(1, pal.line).TextColor(pal.ink).
					Transition(revealTransition).Children(func() {
					ui.Text(c, "查看详情").FontSize(textSize(c, textControl)).FontWeight(500)
					icon(c, "next")
				})
			}
		})
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.Title).FontSize(textSize(c, textBody)).FontWeight(500).LineHeight(1.5).TextColor(pal.ink)
			if s.Day == -1 {
				caption(c, dayName(a.Day))
			}
		})
	}).OnClick(func() { s.openBrowseDetail(a) })
}

func dayName(d int) string {
	if d < 0 || d > 6 {
		return "更新时间未知"
	}
	return []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[d%7]
}

func catalogYears() []string {
	var years []string
	for year := time.Now().Year(); year >= earliestCatalogYear; year-- {
		years = append(years, strconv.Itoa(year))
	}
	return years
}

func pageLoading(c *ui.Context, text string) {
	ui.Column(c).Grow(1).Center().Gap(16).Children(func() { caption(c, text) })
}

func (s *State) enterPage(page string) {
	if page == "downloads" {
		s.DownloadHistory = false
	}
	if s.Page == page {
		return
	}
	if s.Page != "detail" || s.DetailReturn != browseDetailReturn || page != "browse" {
		s.closeBrowseDetail()
	}
	s.Page = page
	s.LibraryMenuOpen = false
}

func (s *State) empty(c *ui.Context, title, body, button string, action func()) {
	ui.Column(c).Grow(1).Center().Gap(16).Children(func() {
		ui.Box(c).TextColor(colors(c).muted).Children(func() { icon(c, "browse") })
		heading(c, title)
		if body != "" {
			caption(c, body)
		}
		secondary(c, "empty-action", button, action)
	})
}

func (s *State) detail(c *ui.Context) {
	a := s.anime(s.AnimeID)
	pal := colors(c)
	insetX := pageInsetX(c)
	_, height := c.Size()
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		if detailUsesPageScroll(c, a) {
			ui.Scroll(c.Key("detail-content")).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Padding(16, insetX, 32).Gap(24).Children(func() {
					s.detailHeader(c, a)
					s.detailOptions(c)
					s.detailPreview(c, true)
					s.detailFooter(c, true)
				})
			})
		} else {
			ui.Scroll(c.Key("detail-header")).MaxHeight(height * .25).MinHeight(0).Children(func() {
				s.detailHeader(c, a)
			})
			ui.Row(c).Grow(1).MinHeight(0).Gap(0).Children(func() {
				ui.Scroll(c.Key("detail-options")).Width(304).FillHeight().Padding(24, 24, 24, insetX).BorderWidth(0, 1, 0, 0).BorderColor(pal.line).Children(func() {
					s.detailOptions(c)
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
	pal := colors(c)
	ui.Column(c).Padding(20, pageInsetX(c), 16).Gap(16).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			backLabel := "返回番剧列表"
			backPage := "browse"
			if s.DetailReturn == "following" {
				backLabel = "返回我的追番"
				backPage = "following"
			} else if s.DetailReturn == "downloads" {
				backLabel = "返回下载"
				backPage = "downloads"
			} else if s.DetailReturn == browseDetailReturn {
				backLabel = "返回番剧详情"
			}
			back := ui.ButtonBase(c.Key("back-browse")).Label(backLabel).
				Padding(8, 12, 8, 4).Gap(8).AlignItems(ui.Center).Radius(radiusSmall).
				FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.muted).Cursor(ui.CursorPointer)
			if back.Hovered() {
				back.Background(pal.chrome).TextColor(pal.ink)
			}
			tactile(back).Children(func() {
				icon(c, "back")
				ui.Text(c, backLabel)
			}).OnClick(func() { s.enterPage(backPage) })
		})
		ui.Row(c).Wrap().Gap(16).AlignItems(ui.Start).Children(func() {
			s.featuredCover(c, a, 56, 76)
			ui.Column(c).Grow(1).MinWidth(0).Gap(8).Children(func() {
				pageTitle(c, a.Title)
				caption(c, dayName(a.Day))
				switch {
				case s.ViewResources:
					ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
						statusPill(c, "已追番资源", pal.accentInk, pal.accentWash)
						compactQuiet(c, "fresh-selection", "重新选字幕组", func() {
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
						}).TextColor(pal.accentInk).Disabled(s.Busy)
					})
				case s.Backfill:
					statusPill(c, "下载已有资源", pal.accentInk, pal.accentWash)
				}
			})
		})
	})
}

func (s *State) detailGroups(c *ui.Context) {
	pal := colors(c)
	heading(c, "字幕组")
	if s.DetailLoading {
		caption(c, "正在加载字幕组…")
	} else if s.DetailError != "" {
		ui.Text(c, s.DetailError).FontSize(textSize(c, textMeta)).TextColor(pal.warningInk).LineHeight(1.5)
		compactSecondary(c, "retry-detail", "重试加载字幕组", s.refreshDetail).Disabled(s.Busy)
	} else if !s.hasGroup() || s.ChoosingGroup {
		s.groupPicker(c)
	} else {
		ui.Row(c).AlignItems(ui.Center).Padding(12, 16).Radius(radiusControl).Background(pal.accentWash).Gap(8).Children(func() {
			ui.Text(c, s.Groups[s.Group]).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.accentInk).LineHeight(1.5).Grow(1).MinWidth(0)
			if !s.ViewResources {
				compactQuiet(c, "change-group", "更换", func() { s.ChoosingGroup = true }).TextColor(pal.accentInk).Disabled(s.Busy)
			}
		})
	}
}

func (s *State) detailOptions(c *ui.Context) {
	ui.Column(c).Gap(20).Children(func() {
		s.detailGroups(c)
		if s.detailReady() {
			if s.VersionSetup {
				s.versionOptions(c)
			} else if s.Suggestions {
				heading(c, "你想要什么版本")
				s.titlePreferences(c)
			} else {
				heading(c, "你想要什么版本")
				s.ruleEditor(c)
			}
		}
	})
}

func (s *State) detailPreview(c *ui.Context, embedded bool) {
	pal := colors(c)
	column := ui.Column(c).MinWidth(0).Gap(16)
	if embedded {
		column.Padding(0)
	} else {
		column.Grow(1).FillHeight().Padding(24, 0, 0, 0)
	}
	insetX := pageInsetX(c)
	headerRow := func() {
		row := ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12)
		if !embedded {
			row.Padding(0, insetX, 0, 24)
		}
		row.Children(func() {
			heading(c, "可下载资源")
			ui.Spacer(c)
			if s.EpisodeFilter != "" {
				statusPill(c, "只看"+episodeLabel(s.EpisodeFilter), pal.accentInk, pal.accentWash)
				compactQuiet(c, "clear-episode-filter", "清除", func() { s.toggleEpisodeFilter(s.EpisodeFilter) }).TextColor(pal.accentInk)
			}
			if s.detailReady() && (!s.VersionSetup || !s.TitleSetup) {
				ui.Checkbox(c.Key("only-matches"), &s.OnlyMatches, "只看匹配")
				ui.Checkbox(c.Key("only-downloaded"), &s.OnlyDownloaded, "只看已有任务")
				if s.ViewResources && s.EditID != "" && (s.OnlyNew || s.hasNewRelease()) {
					ui.Checkbox(c.Key("only-new"), &s.OnlyNew, "只看新资源")
				}
			}
		})
	}
	column.Children(func() {
		headerRow()
		if s.DetailLoading {
			pageLoading(c, "正在加载字幕组…")
		} else if s.DetailError != "" {
			s.empty(c, "字幕组加载失败", s.DetailError, "重试加载", s.refreshDetail)
		} else if len(s.Groups) == 0 {
			s.empty(c, "暂无字幕组", "来源尚未提供这部番剧的字幕组信息。", "重新加载", s.refreshDetail)
		} else if !s.hasGroup() && s.GroupCountLoading {
			pageLoading(c, "正在加载各字幕组资源数量…")
		} else if !s.hasGroup() {
			pageLoading(c, "先选择一个字幕组。")
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
	pal := colors(c)
	ui.Column(c).Gap(16).Children(func() {
		for _, kind := range []struct {
			title string
			words []string
			color ui.Color
		}{{"只要这些词", s.Rule.All, pal.accentInk}, {"不要这些词", s.Rule.None, pal.muted}} {
			k := kind
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, k.title).FontSize(textSize(c, textControl)).FontWeight(500).TextColor(k.color)
				if len(k.words) == 0 {
					caption(c, "暂不限制")
				}
				ui.Row(c).Wrap().Gap(8).Children(func() {
					for i, word := range k.words {
						w := word
						key := fmt.Sprintf("rule-%s-%d", k.title, i)
						ui.Row(c.Key(key)).AlignItems(ui.Center).Gap(4).Transition(rowTransition).Children(func() {
							s.wordChip(c, key, w)
							iconButton(c, key+"-remove", "移除条件「"+w+"」", "close", func() { s.setWord(w, wordUnrestricted) }).Size(28, 28).Radius(radiusSmall)
						})
					}
				})
			})
		}
		for idx, g := range s.Rule.Any {
			ui.Column(c).Padding(12, 16).Radius(radiusControl).Background(pal.accentWash).Gap(4).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "任意一个写法都可以").FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.accentInk).Grow(1)
					iconButton(c, fmt.Sprintf("remove-alias-%d", idx), fmt.Sprintf("移除第%d组写法", idx+1), "close", func() {
						rule := cloneRule(s.Rule)
						rule.Any = slices.Delete(rule.Any, idx, idx+1)
						s.applyRule(rule, "移除任意词组")
					}).Size(28, 28).Radius(radiusSmall)
				})
				ui.Text(c, strings.Join(g, " / ")).FontSize(textSize(c, textMeta)).TextColor(pal.ink)
			})
		}
		ui.Text(c, "添加标题词").FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.ink)
		ui.TextInput(c.Key("keyword"), &s.Keyword).Label("标题关键词").Placeholder("输入标题中的词").FillWidth().MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			compactSecondary(c, "want-custom", "只要这个词", func() { s.setWord(s.Keyword, wordWant); s.Keyword = "" }).TextColor(pal.accentInk).Disabled(strings.TrimSpace(s.Keyword) == "")
			compactSecondary(c, "avoid-custom", "不要这个词", func() { s.setWord(s.Keyword, wordAvoid); s.Keyword = "" }).TextColor(pal.muted).Disabled(strings.TrimSpace(s.Keyword) == "")
		})
		compactQuiet(c, "toggle-aliases", "添加同义写法", func() { s.ShowAliases = !s.ShowAliases }).TextColor(pal.accentInk).AlignSelf(ui.Start)
		if s.ShowAliases {
			ui.TextInput(c.Key("aliases"), &s.AnyDraft).Label("同义写法").Placeholder("简日内嵌 / CHS").FillWidth().MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface)
			compactSecondary(c, "add-alias-group", "添加这组写法", s.addAliases).Disabled(strings.TrimSpace(s.AnyDraft) == "")
		}
		compactQuiet(c, "clear-rules", "重置全部条件", func() { s.applyRule(Rule{}, "重置全部条件"); s.WordOpen = false }).TextColor(pal.muted).AlignSelf(ui.Start)
	})
}

func (s *State) tagChip(c *ui.Context, key, word string) {
	pal := colors(c)
	col, bg := pal.ink, pal.canvas
	if choice := s.wordChoice(word); choice == wordWant {
		col, bg = pal.accentInk, pal.accentWash
	} else if choice == wordAvoid {
		col, bg = pal.muted, pal.sunken
	}
	b := compactQuiet(c, key, word, func() { s.WordAnchor = key; s.WordOpen = true }).
		Label("筛选 "+word).Padding(4, 12).Radius(radiusSmall).Border(1, pal.line).Background(bg).TextColor(col).Tooltip("选择只要、不要，或移除条件")
	if s.WordAnchor == key {
		s.wordPopover(c, b, key, word)
	}
}

func (s *State) wordChip(c *ui.Context, key, word string) {
	pal := colors(c)
	choice := s.wordChoice(word)
	label := word
	col, bg := pal.ink, pal.surface
	if choice == wordWant {
		label = "＋ " + word
		col, bg = pal.accentInk, pal.accentWash
	}
	if choice == wordAvoid {
		label = "− " + word
		col, bg = pal.muted, pal.sunken
	}
	b := compactQuiet(c, key, label, func() { s.WordAnchor = key; s.WordOpen = true }).
		Label("筛选 "+word).Padding(4, 12).Radius(radiusSmall).Border(1, pal.line).Background(bg).TextColor(col).Tooltip("选择只要、不要，或移除条件")
	if s.WordAnchor == key {
		s.wordPopover(c, b, key, word)
	}
}

func (s *State) wordPopover(c *ui.Context, anchor ui.Element, key, word string) {
	pal := colors(c)
	choice := s.wordChoice(word)
	ui.PopoverBase(c.Key("word-popup-"+key), anchor, &s.WordOpen, func(panel ui.Element) {
		panel.Width(320).Margin(4, 0, 0).Padding(12).Gap(4).Radius(radiusPanel).Background(pal.surface).Border(1, pal.outline).Transition(popupTransition)
		ui.Text(c, "筛选「"+word+"」").FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.ink).Padding(8, 12)
		for _, option := range []struct {
			label     string
			choice    wordChoice
			color, bg ui.Color
		}{{"只要包含", wordWant, pal.accentInk, pal.accentWash}, {"不要包含", wordAvoid, pal.muted, pal.sunken}} {
			o := option
			count := s.countRuleMatches(ruleWithWord(s.Rule, word, o.choice))
			button := ui.ButtonBase(c.Key(fmt.Sprintf("word-choice-%d", o.choice))).
				Label(map[wordChoice]string{wordWant: "只要含「" + word + "」的", wordAvoid: "不要含「" + word + "」的"}[o.choice]).
				Row().FillWidth().AlignItems(ui.Center).Gap(8).Padding(8, 12).Radius(radiusSmall).Cursor(ui.CursorPointer)
			tactile(button)
			if choice == o.choice {
				button.Background(o.bg)
			} else if button.Hovered() {
				button.Background(pal.chrome)
			}
			button.Children(func() {
				ui.Text(c, o.label).FontSize(textSize(c, textControl)).FontWeight(500).TextColor(o.color)
				ui.Spacer(c)
				ui.Textf(c, "剩 %d 条", count).FontSize(textSize(c, textMeta)).TextColor(o.color)
			}).OnClick(func() { s.setWord(word, o.choice) })
		}
		if choice != wordUnrestricted {
			compactQuiet(c, "unrestrict-word", "不再限制这个词", func() { s.setWord(word, wordUnrestricted) }).Label("不再限制「" + word + "」").FillWidth()
		}
	})
}

func (s *State) quickFilter(c *ui.Context) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, "点词筛选版本").FontSize(textSize(c, textControl)).FontWeight(500).TextColor(colors(c).ink)
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for i, word := range s.quickWords() {
				s.wordChip(c, fmt.Sprintf("quick-word-%d", i), word)
			}
		})
	})
}

func (s *State) preview(c *ui.Context, embedded bool) {
	pal := colors(c)
	manual := s.VersionSetup && s.TitleSetup
	insetX := pageInsetX(c)
	previewBlock := func(fn func()) {
		if embedded {
			fn()
			return
		}
		ui.Column(c).Padding(0, insetX, 0, 24).Gap(12).Children(fn)
	}
	previewBlock(func() {
		if s.VersionSetup {
			if !manual {
				s.versionSummary(c)
				if s.Rule.Version == nil {
					return
				}
			}
		} else {
			s.quickFilter(c)
		}
	})
	if s.VersionSetup && !manual && s.Rule.Version == nil {
		return
	}
	n, eps, _ := s.matchSummary()
	matches := s.matchingResources()
	choices, byEpisode := s.episodeChoices(matches)
	previewBlock(func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Textf(c, "匹配 %d 条", n).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.accentInk).ContentTransition(ui.ContentTransition{Numeric: true, Animation: &contentMotion})
			caption(c, fmt.Sprintf("覆盖 %d 集，%s", eps, resourceSizes(matches)))
			if len(s.UndoFilters) > 0 {
				compactQuiet(c, "undo-filter", "撤销上一步", s.undoFilter).Tooltip("撤销" + s.UndoFilters[len(s.UndoFilters)-1].Action + " (Ctrl+Z)").TextColor(pal.accentInk)
			}
			if len(s.RedoFilters) > 0 {
				compactQuiet(c, "redo-filter", "重做筛选", s.redoFilter).Tooltip("重做" + s.RedoFilters[len(s.RedoFilters)-1].Action + " (Ctrl+Y)").TextColor(pal.accentInk)
			}
		})
		if s.FilterFeedback != "" {
			ui.Text(c, s.FilterFeedback).FontSize(textSize(c, textMeta)).TextColor(pal.ink).ContentTransition(ui.ContentTransition{Animation: &contentMotion})
		}
		if n == 0 {
			message := "没有资源符合这些条件。"
			if len(s.resources()) == 0 {
				message = "这组暂无已发布资源，开启追番后将按条件检查后续更新。"
			}
			ui.Text(c, message).FontSize(textSize(c, textMeta)).TextColor(pal.warningInk).Padding(12, 16).Background(pal.warningWash).Radius(radiusControl).LineHeight(1.5)
		}
		if !s.Editing && !manual {
			selected, _ := s.selected()
			all := len(choices) > 0 && len(selected) == len(choices)
			label := "全选匹配资源"
			if s.selectsOnePerEpisode() {
				label = "全选符合条件的集数"
			}
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Checkbox(c.Key("select-all"), &all, label).Disabled(n == 0).OnChange(func() {
					for _, resource := range choices {
						s.setResourceChecked(resource, all)
					}
				})
				ui.Spacer(c)
				compactQuiet(c, "select-none", "清空选择", func() {
					for _, r := range s.resources() {
						s.Checked[r.ID] = false
					}
				}).TextColor(pal.muted)
			})
		}
	})
	var resourceViewport ui.Rect
	renderRows := func() {
		ui.Column(c).Gap(0).Children(func() {
			visible := 0
			for _, r := range s.previewResources() {
				res := r
				ok, _ := matchResource(res, s.Rule)
				if !manual && s.OnlyMatches && !ok {
					continue
				}
				if s.EpisodeFilter != "" && res.Episode != s.EpisodeFilter {
					continue
				}
				if choice, exists := byEpisode[res.Episode]; exists && s.EpisodeFilter == "" &&
					!s.OnlyDownloaded && choice.ID != res.ID {
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
				s.resourceRow(c, res, byEpisode[res.Episode].ID == res.ID, resourceViewport)
			}
			if visible == 0 && len(s.previewResources()) > 0 {
				s.empty(c, "没有符合筛选的资源", "清除筛选查看全部资源。", "清除筛选", s.clearPreviewFilters)
			}
		})
	}
	if embedded {
		ui.Column(c).Padding(0, 0, 24).Children(renderRows)
	} else {
		sc := ui.Scroll(c.Key("resources")).TrackScroll(&s.ResourceScroll).Grow(1).MinHeight(0).Padding(0, insetX, 24, 24)
		resourceViewport = sc.Bounds()
		sc.Children(renderRows)
	}
}

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

func (s *State) resourceRow(c *ui.Context, r Resource, episodeChoice bool, viewport ui.Rect) {
	pal := colors(c)
	ok, reason := matchResource(r, s.Rule)
	on := s.Checked[r.ID] && ok
	working := on || s.ActiveTitleRow == r.ID || (s.WordOpen && strings.HasPrefix(s.WordAnchor, "tag-"+r.ID+"-"))
	row := ui.Column(c.Key("resource-"+r.ID)).Padding(16, 12).Gap(8).Radius(radiusControl).BorderWidth(0, 0, 1, 0).BorderColor(pal.line).Transition(rowTransition)
	if row.Hovered() {
		row.Background(pal.chrome)
	}
	row.Children(func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(controlGap).Children(func() {
			if !s.Editing && !(s.VersionSetup && s.TitleSetup) {
				ui.Checkbox(c.Key("pick-"+r.ID), &on, "").Label("选择 " + r.ID).Disabled(!ok).OnChange(func() { s.setResourceChecked(r, on) })
			}
			ui.Text(c, episodeLabel(r.Episode)).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink)
			if s.isNewRelease(r.Title) {
				statusPill(c, "新", pal.accentInk, pal.accentWash)
			}
			if s.Suggestions && s.SuggestedResource.ID == r.ID {
				statusPill(c, "正在参考", pal.accentInk, pal.accentWash)
			}
			if count := s.duplicates(r.Episode); count > 1 && ok {
				ep := r.Episode
				fg, bg := pal.warningInk, pal.warningWash
				label := fmt.Sprintf("本集匹配 %d 条", count)
				if s.EpisodeFilter == r.Episode {
					fg, bg = pal.accentInk, pal.accentWash
				}
				if s.selectsOnePerEpisode() {
					label = fmt.Sprintf("本集匹配 %d 个版本 · 换一个", count)
					if s.EpisodeFilter == ep {
						label = "返回全部集数"
					}
				}
				if !s.selectsOnePerEpisode() || episodeChoice {
					compactQuiet(c, "filter-episode-"+r.ID, label, func() { s.toggleEpisodeFilter(ep) }).TextColor(fg).Background(bg).Padding(4, 8)
				}
			}
			if d, idx, found := s.downloadForResource(r); found {
				switch {
				case d.Failed:
					statusPill(c, "失败", pal.warningInk, pal.warningWash)
				case d.Progress < 1 && d.Stopped:
					caption(c, "已停止")
				case d.Progress < 1 && d.Paused:
					caption(c, "已暂停")
				case d.Queued:
					caption(c, "排队中")
				case d.Progress < 1:
					statusPill(c, fmt.Sprintf("下载中 %d%%", int(d.Progress*100)), pal.accentInk, pal.accentWash)
				default:
					caption(c, "已有任务")
				}
				if d.Progress >= 1 && !d.Failed {
					intentGroupIn(c, row, viewport, working, func() {
						compactQuiet(c, "play-resource-"+r.ID, "播放", func() { s.playDownload(idx) }).Label("播放 " + r.ID).TextColor(pal.accentInk).Disabled(s.Busy)
						compactQuiet(c, "open-resource-folder-"+r.ID, "打开文件夹", func() {
							if s.OpenFolder != nil {
								s.OpenFolder(d.Folder)
							}
						}).Label("打开文件夹 " + r.ID).TextColor(pal.accentInk).Disabled(s.Busy || s.OpenFolder == nil)
					})
				}
			}
			ui.Spacer(c)
			if bytes := sizeBytes(r.Bytes, r.MB); bytes > 0 {
				caption(c, byteSizeText(bytes))
			} else {
				caption(c, "大小未知")
			}
		})
		if s.VersionSetup && s.TitleSetup {
			s.originalTitle(c, r)
		} else {
			ui.Text(c, r.Title).FontSize(textSize(c, textBody)).FontWeight(400).LineHeight(1.5).Selectable().TextColor(pal.ink)
		}
		published := r.Published
		if published == "" {
			published = "发布时间未知"
		}
		caption(c, published)
		if !s.VersionSetup {
			ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(func() {
				for i, word := range r.Tokens {
					s.tagChip(c, fmt.Sprintf("tag-%s-%d", r.ID, i), word)
				}
				ui.Spacer(c)
				intentGroupIn(c, row, viewport, working, func() {
					compactQuiet(c, "suggest-"+r.ID, "按此资源筛选", func() {
						s.WordOpen = false
						s.SuggestedResource = r
						s.Suggestions = true
						s.ResourceScroll = ui.ScrollState{}
					}).Label("按此资源筛选 " + r.ID).TextColor(pal.accentInk)
				})
			})
		}
		caption(c, reason)
	})
}

func (s *State) detailFooter(c *ui.Context, embedded bool) {
	pal := colors(c)
	var plan submissionPlan
	if s.detailReady() {
		plan = s.planSubmission()
	}
	resources := plan.New
	_, height := c.Size()
	render := func() {
		ui.Column(c).Padding(16, pageInsetX(c), 20).Gap(8).Background(pal.canvas).BorderWidth(1, 0, 0, 0).BorderColor(pal.line).Children(func() {
			ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
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
					} else if !s.canSaveDetailRule() {
						title = "选好版本后即可追番"
						if s.TitleSetup {
							title = "添加标题条件后即可追番"
						}
					}
					ui.Text(c, title).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5)
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
					compactQuiet(c, "begin-backfill", "下载已有资源", s.beginBackfill).TextColor(pal.accentInk).Disabled(!s.detailReady() || s.Busy)
				}
				primary(c, "start-follow", label, s.commit).Disabled(!s.canSaveDetailRule() || s.Busy || (s.Backfill && len(resources) == 0))
			})
			if s.Backfill {
				if sub, ok := s.subscription(s.EditID); ok && sub.Paused {
					caption(c, "追番已暂停；现在只会排队，恢复追番后开始下载。")
				}
			}
		})
	}
	if embedded {
		ui.Column(c).Children(render)
		return
	}
	ui.Scroll(c.Key("detail-footer")).MaxHeight(height * .32).MinHeight(0).Children(render)
}

func (s *State) cancelDialog(c *ui.Context) {
	sub, ok := s.subscription(s.CancelID)
	if !ok {
		return
	}
	pal := colors(c)
	ui.DialogBase(c, &s.CancelPrompt, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.scrim).Transition(fadeTransition)
		dialogContentWithFooter(c, panel, "cancel-content", 480, radiusSheet, func() {
			group := sub.GroupName
			if group == "" {
				group = "字幕组名称未知"
			}
			pageTitle(c, "确认取消追番")
			heading(c, s.anime(sub.AnimeID).Title)
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "字幕组："+group).FontSize(textSize(c, textBody)).TextColor(pal.ink).LineHeight(1.5)
				caption(c, "取消后不再检查更新；已下载的文件保留在 "+sub.Folder)
				flight := 0
				for _, d := range s.Downloads {
					if d.AnimeID == sub.AnimeID && (downloadState(d) == downloadsDownloading || downloadState(d) == downloadsQueued) {
						flight++
					}
				}
				if flight > 0 {
					caption(c, fmt.Sprintf("下载中和排队中的 %d 个任务会继续完成。", flight))
				}
			})
		}, func() {
			secondary(c, "keep-subscription", "保留追番", func() { s.CancelPrompt = false })
			primary(c, "confirm-cancel-subscription", "确认取消追番", func() {
				s.CancelPrompt, s.CancelID = false, ""
				s.removeSubscription(sub.ID)
			}).Label("确认取消追番 " + sub.GroupName).Disabled(s.Busy)
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
	pal := colors(c)
	ui.DialogBase(c, &s.RemovePrompt, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.scrim).Transition(fadeTransition)
		dialogContentWithFooter(c, panel, "remove-content", 480, radiusSheet, func() {
			pageTitle(c, "删除任务和文件？")
			heading(c, s.anime(d.AnimeID).Title)
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, d.Title).FontSize(textSize(c, textBody)).TextColor(pal.ink).LineHeight(1.5)
				ui.Text(c, "已下载文件会一并删除，无法恢复。").FontSize(textSize(c, textMeta)).TextColor(pal.errorInk).LineHeight(1.5)
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
			}).Label("确认删除任务和文件").TextColor(pal.errorInk).Disabled(s.Busy)
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

func (s *State) retryableChecks() []string {
	var pending []string
	for _, sub := range s.Subscriptions {
		if sub.CheckFailed && !sub.Paused {
			pending = append(pending, sub.ID)
		}
	}
	return pending
}

func (s *State) following(c *ui.Context) {
	pal := colors(c)
	insetX := pageInsetX(c)
	topPad := float32(36)
	if textScale(c) > 1.5 {
		topPad = 24
	}
	ui.Column(c).Grow(1).MinHeight(0).Padding(topPad, insetX, 0).Gap(20).Children(func() {
		pageTitle(c, "我的追番")
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
			if ui.TextInput(c.Key("follow-search"), &s.FollowSearch).Label("搜索追番").Placeholder("搜索已追番剧").Width(240).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).Changed() {
				s.FollowScroll = ui.ScrollState{}
			}
			if len(s.retryableChecks()) > 0 {
				quiet(c, "retry-failed-checks", "重试检查", s.retryFailedChecks).TextColor(pal.warningInk).Disabled(s.Busy)
			}
			secondary(c, "find-more", "添加追番", func() { s.enterPage("browse") })
		})
		if s.Notice != "" {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Padding(12, 16).Radius(radiusControl).TextColor(pal.ink).Background(pal.accentWash).Children(func() {
				ui.Text(c, s.Notice).FontSize(textSize(c, textControl)).FontWeight(500).Grow(1).MinWidth(0)
				if s.UndoSaved != nil {
					compactQuiet(c, "undo-save", "撤销保存", s.undoSave).TextColor(pal.accentInk).Disabled(s.Busy)
				}
			})
		}
		if len(s.Subscriptions) == 0 {
			s.empty(c, "还没有追番", "", "浏览番剧", func() { s.enterPage("browse") })
			return
		}
		var subs []Subscription
		for _, sub := range s.Subscriptions {
			if s.follows(sub) {
				subs = append(subs, sub)
			}
		}
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
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			caption(c, fmt.Sprintf("%d 部追番", len(subs)))
			ui.Spacer(c)
			if strings.TrimSpace(s.FollowSearch) != "" {
				compactQuiet(c, "clear-follow-search", "清除搜索", func() { s.FollowSearch = "" }).TextColor(pal.accentInk)
			}
		})
		if len(subs) == 0 {
			s.empty(c, "没有找到这部追番", "", "清除搜索", func() { s.FollowSearch = "" })
			return
		}
		width, _ := c.Size()
		cols, cardWidth := gridColumnsForInner(width - sidebarWidth - insetX*2)
		ui.Scroll(c.Key("following-list")).TrackScroll(&s.FollowScroll).Grow(1).MinHeight(0).Padding(0, 0, 32).Children(func() {
			ui.Grid(c).Columns(cols).GapX(posterGapX).GapY(posterGapY).Children(func() {
				for _, sub := range subs {
					item := sub
					s.followTile(c, item, cardWidth)
				}
			})
		})
	})
}

func gridColumns(windowWidth float32) (int, float32) {
	inset := float32(40)
	if windowWidth <= 1080 {
		inset = 32
	}
	return gridColumnsForInner(windowWidth - sidebarWidth - inset*2)
}

func gridColumnsForInner(innerWidth float32) (int, float32) {
	cols := int((innerWidth + posterGapX) / 212)
	if cols < 3 {
		cols = 3
	}
	if cols > 6 {
		cols = 6
	}
	return cols, (innerWidth - float32(cols-1)*posterGapX) / float32(cols)
}

func (s *State) followTile(c *ui.Context, sub Subscription, width float32) {
	a := s.anime(sub.AnimeID)
	pal := colors(c)
	tile := ui.ButtonBase(c.Key("subscription-" + sub.ID)).Label("打开追番 " + sub.GroupName).Width(width).Column().Justify(ui.Start).Gap(12).Padding(0).AlignItems(ui.Stretch).Radius(coverRadius).Cursor(ui.CursorPointer)
	tile.Children(func() {
		ui.Box(c).Radius(coverRadius).Children(func() {
			s.posterCover(c, tile, a, width)
			ui.Column(c).Absolute().Top(8).Left(8).Right(8).Gap(4).AlignItems(ui.Start).Children(func() {
				if sub.CheckFailed {
					statusPill(c, "检查失败", pal.warningInk, pal.warningWash)
				}
				if sub.Paused {
					statusPill(c, "已暂停", pal.ink, pal.surface).Border(1, pal.line)
				} else if sub.Active {
					label := "下载中"
					if sub.Progress > 0 {
						label = fmt.Sprintf("下载中 %d%%", int(sub.Progress*100))
					}
					statusPill(c, label, pal.accentInk, pal.accentWash)
				} else if sub.Queued {
					statusPill(c, "排队中", pal.ink, pal.surface).Border(1, pal.line)
				}
			})
		})
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.Title).FontSize(textSize(c, textBody)).FontWeight(500).LineHeight(1.5).TextColor(pal.ink)
			group := sub.GroupName
			if group == "" {
				group = "字幕组名称未知"
			}
			caption(c, group)
		})
		if sub.NewCount > 0 || sub.Completed > 0 {
			ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				if sub.NewCount > 0 {
					compactQuiet(c, "open-new-"+sub.ID, fmt.Sprintf("新资源 %d 条", sub.NewCount), func() {
						s.viewSubscriptionResources(sub.ID)
						s.OnlyNew = true
					}).Label(fmt.Sprintf("新资源 %d 条 %s", sub.NewCount, sub.GroupName)).TextColor(pal.warningInk).Background(pal.warningWash).Padding(4, 8).Disabled(s.Busy)
				}
				if sub.Completed > 0 {
					caption(c, fmt.Sprintf("已完成 %d 个下载", sub.Completed))
				}
			})
		}
	}).OnClick(func() { s.openLibrary(sub.ID) })
}

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

func (s *State) library(c *ui.Context) {
	sub, ok := s.subscription(s.LibraryID)
	if !ok {
		s.empty(c, "找不到这条追番", "它可能已被取消追番。", "回到我的追番", func() { s.enterPage("following") })
		return
	}
	pal := colors(c)
	insetX := pageInsetX(c)
	indices := s.libraryEpisodes(sub.AnimeID)
	renderBack := func() {
		ui.Row(c).Padding(16, insetX-16, 0).Children(func() {
			back := ui.ButtonBase(c.Key("back-following")).Label("返回我的追番").
				Padding(8, 16).Gap(8).AlignItems(ui.Center).Radius(radiusSmall).
				FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.muted).Cursor(ui.CursorPointer)
			if back.Hovered() {
				back.Background(pal.chrome).TextColor(pal.ink)
			}
			tactile(back).Children(func() {
				icon(c, "back")
				ui.Text(c, "返回我的追番")
			}).OnClick(func() { s.enterPage("following") })
		})
	}
	renderEpisodes := func(epViewport ui.Rect) {
		if len(indices) == 0 {
			s.empty(c, "还没有下载完成的剧集", "", "查看资源", func() { s.viewSubscriptionResources(sub.ID) })
			return
		}
		for position, index := range indices {
			if position > 0 {
				ui.Divider(c).Background(pal.line)
			}
			s.libraryEpisodeRow(c, index, epViewport)
		}
	}
	if textScale(c) > 1 {
		epScroll := ui.Scroll(c.Key("episode-list")).Grow(1).MinHeight(0)
		epViewport := epScroll.Bounds()
		epScroll.Children(func() {
			renderBack()
			s.libraryHero(c, sub, true)
			ui.Column(c).Padding(0, insetX, 32).Children(func() {
				renderEpisodes(epViewport)
			})
		})
		return
	}
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		renderBack()
		s.libraryHero(c, sub, false)
		if len(indices) == 0 {
			s.empty(c, "还没有下载完成的剧集", "", "查看资源", func() { s.viewSubscriptionResources(sub.ID) })
			return
		}
		epScroll := ui.Scroll(c.Key("episode-list")).Grow(1).MinHeight(0).Padding(0, insetX, 32)
		epViewport := epScroll.Bounds()
		epScroll.Children(func() {
			renderEpisodes(epViewport)
		})
	})
}

type episodeMenuOpenKey struct{}

func (s *State) libraryEpisodeRow(c *ui.Context, index int, viewport ui.Rect) {
	download := s.Downloads[index]
	pal := colors(c)
	row := ui.Row(c.Key("episode-"+download.ID)).AlignItems(ui.Center).Gap(16).Padding(16, 12).Radius(radiusControl).Transition(rowTransition)
	if row.Hovered() {
		row.Background(pal.chrome)
	}
	menuOpen := ui.Local(row, episodeMenuOpenKey{}, func() bool { return false })
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, episodeLabel(download.Episode)).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink)
				switch {
				case download.Failed:
					statusPill(c, "失败", pal.warningInk, pal.warningWash)
				case download.Progress >= 1:
					caption(c, "已完成")
				case download.Stopped:
					caption(c, "已停止")
				case download.Paused:
					caption(c, "已暂停")
				case download.Queued:
					caption(c, "排队中")
				default:
					statusPill(c, fmt.Sprintf("下载中 %d%%", int(download.Progress*100)), pal.accentInk, pal.accentWash)
				}
			})
			caption(c, download.Title)
			if download.Error != "" {
				ui.Text(c, download.Error).FontSize(textSize(c, textMeta)).TextColor(pal.warningInk).LineHeight(1.5)
			}
		})
		switch {
		case download.Failed:
			secondary(c, "retry-episode-"+download.ID, "重试", func() { s.retryDownload(index) }).
				Label("重试 " + download.ID).Shrink(0).Disabled(s.Busy)
		case download.Progress < 1 && (download.Paused || download.Stopped):
			secondary(c, "resume-episode-"+download.ID, "继续", func() { s.pauseDownload(index) }).
				Label("继续 " + download.ID).Shrink(0).Disabled(s.Busy)
		case download.Progress >= 1:
			ui.Row(c).AlignItems(ui.Center).Gap(8).Shrink(0).Children(func() {
				intentGroupIn(c, row, viewport, *menuOpen, func() {
					iconButton(c, "open-episode-folder-"+download.ID, "打开文件夹", "folder", func() {
						if s.OpenFolder != nil {
							s.OpenFolder(download.Folder)
						}
					}).Label("打开文件夹 " + download.ID).Disabled(s.Busy || s.OpenFolder == nil || download.Folder == "")
					ui.PopoverMenuButton(c.Key("episode-menu-"+download.ID), "更多", func(menu *ui.Menu) {
						if menu.Item("查看文件").Disabled(s.Busy).Chosen() {
							s.openFiles(index)
						}
						if menu.Item("删除或重新下载").Disabled(s.Busy).Chosen() {
							s.RemovePrompt, s.RemoveID = true, download.ID
						}
					}, ui.PopoverMenuButtonOptions{
						At:    ui.AnchorBottomEnd,
						Self:  ui.AnchorTopEnd,
						Open:  menuOpen,
						Panel: menuPanelStyle(c),
					}).Label("剧集操作 " + download.ID).FontSize(textSize(c, textControl)).MinHeight(40).Disabled(s.Busy)
				})
				primary(c, "play-episode-"+download.ID, "播放", func() { s.playDownload(index) }).
					Label("播放 " + download.ID).MinWidth(76).Disabled(s.Busy)
			})
		}
	})
}

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
	case "":
		ratioCopy = "分享率达到 1:1"
	default:
		ratioCopy = "分享率达到 " + s.SeedRatio
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
	case d.Stopped:
		return downloadsStopped
	case d.Paused:
		return downloadsPaused
	case d.Progress >= 1:
		return downloadsSeeding
	case d.Queued:
		return downloadsQueued
	default:
		return downloadsDownloading
	}
}

func downloadInQueue(d Download) bool {
	return d.Failed || !(d.Progress >= 1 && d.Stopped)
}

func (s *State) toggleDownloadGroup(animeID string) {
	if s.CollapsedDownloads == nil {
		s.CollapsedDownloads = make(map[string]bool)
	}
	s.CollapsedDownloads[animeID] = !s.CollapsedDownloads[animeID]
}

func (s *State) hasNewRelease() bool {
	for _, res := range s.resources() {
		if s.isNewRelease(res.Title) {
			return true
		}
	}
	return false
}

func (s *State) completedDownloads(animeID string) int {
	count := 0
	for _, d := range s.Downloads {
		if d.AnimeID == animeID && d.Progress >= 1 && !d.Failed {
			count++
		}
	}
	return count
}

func (s *State) clearPreviewFilters() {
	s.OnlyMatches, s.OnlyDownloaded, s.OnlyNew = false, false, false
	s.EpisodeFilter = ""
	s.ResourceScroll = ui.ScrollState{}
}

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
	return downloadInQueue(d) && (s.DownloadFilter == downloadsAll || downloadState(d) == s.DownloadFilter)
}

func (s *State) downloads(c *ui.Context) {
	if s.DownloadHistory {
		s.downloadHistory(c)
		return
	}
	pal := colors(c)
	insetX := pageInsetX(c)
	topPad, outerGap, groupGap := float32(36), float32(20), float32(40)
	if textScale(c) > 1.5 {
		topPad, outerGap, groupGap = 16, 12, 24
	}
	active, resumable, queued := 0, 0, 0
	finished := 0
	for _, d := range s.Downloads {
		if !downloadInQueue(d) {
			finished++
			continue
		}
		queued++
		if !d.Paused && !d.Stopped && !d.Failed && d.Progress < 1 {
			active++
		}
		if !d.Failed && (d.Paused || d.Stopped && d.Progress < 1) {
			resumable++
		}
	}
	if s.DownloadFilterLast != s.DownloadFilter {
		s.DownloadFilterLast = s.DownloadFilter
		s.DownloadScroll = ui.ScrollState{}
	}
	ui.Column(c).Grow(1).MinHeight(0).Padding(topPad, insetX, 20).Gap(outerGap).Children(func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(24).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Grow(1).Children(func() {
				pageTitle(c, "下载")
				if finished > 0 {
					compactQuiet(c, "download-history", fmt.Sprintf("下载记录 %d", finished), func() {
						s.DownloadHistory = true
						s.DownloadHistoryScroll = ui.ScrollState{}
					}).TextColor(pal.accentInk)
				}
			})
			ui.Column(c).Gap(4).Children(func() {
				caption(c, "总下载速度")
				ui.Text(c, speedText(s.downloadSpeed(), false)).FontSize(textSize(c, textSection)).FontWeight(600).TextColor(pal.accentInk)
			})
			ui.Column(c).Gap(4).Children(func() {
				caption(c, "总上传速度")
				ui.Text(c, speedText(s.uploadSpeed(), true)).FontSize(textSize(c, textSection)).FontWeight(600).TextColor(pal.ink)
			})
		})
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
			if ui.TextInput(c.Key("download-search"), &s.DownloadSearch).Label("搜索下载").Placeholder("搜索番剧或任务").Width(260).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).Changed() {
				s.DownloadScroll = ui.ScrollState{}
			}
			if strings.TrimSpace(s.DownloadSearch) != "" {
				compactQuiet(c, "clear-download-search", "清除搜索", func() { s.clearDownloadSearch() }).TextColor(pal.accentInk)
			}
		})
		visible := make([]int, 0, len(s.Downloads))
		stateCounts := map[DownloadFilter]int{}
		for i, d := range s.Downloads {
			if !downloadInQueue(d) {
				continue
			}
			stateCounts[downloadState(d)]++
			if s.downloadMatches(d) && s.downloadSearches(d) {
				visible = append(visible, i)
			}
		}
		if queued > 0 {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				caption(c, fmt.Sprintf("%d 个下载任务，按番剧分组", len(visible)))
				ui.Spacer(c)
				pauseLabel := "暂停全部下载"
				if active == 0 {
					pauseLabel = "继续全部下载"
				}
				if active > 0 || resumable > 0 {
					compactQuiet(c, "pause-all", pauseLabel, func() { s.pauseAllDownloads(active > 0) }).Disabled(s.Busy)
				}
			})
			ui.Row(c.Key("download-filter")).Wrap().Gap(8).Children(func() {
				for _, option := range []struct {
					filter DownloadFilter
					label  string
				}{{downloadsAll, "全部"}, {downloadsDownloading, "下载中"}, {downloadsQueued, "排队中"}, {downloadsSeeding, "分享中"}, {downloadsPaused, "已暂停"}, {downloadsFailed, "失败"}, {downloadsStopped, "已停止"}} {
					filter, label := option.filter, option.label
					count := stateCounts[filter]
					if filter == downloadsAll {
						count = queued
					}
					button := compactQuiet(c, fmt.Sprintf("download-filter-%d", filter), fmt.Sprintf("%s %d", label, count), func() { s.DownloadFilter = filter }).Label(label).Shrink(0)
					if filter == s.DownloadFilter {
						button.Background(pal.accentWash).TextColor(pal.accentInk)
					}
				}
			})
		}
		if len(s.Downloads) == 0 {
			s.empty(c, "还没有下载任务", "", "浏览番剧", func() { s.enterPage("browse") })
			return
		}
		if queued == 0 {
			s.empty(c, "当前没有进行中的任务", "已结束的任务可在上方「下载记录」中查看。", "我的追番", func() { s.enterPage("following") })
			return
		}
		if len(visible) == 0 {
			if strings.TrimSpace(s.DownloadSearch) != "" {
				s.empty(c, "没有匹配的下载", "", "清除搜索", func() { s.clearDownloadSearch() })
				return
			}
			s.empty(c, "这个筛选下没有下载", "", "查看全部下载", func() { s.DownloadFilter = downloadsAll })
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
			}{{downloadsDownloading, "下载中"}, {downloadsQueued, "排队中"}, {downloadsSeeding, "分享中"}, {downloadsPaused, "已暂停"}, {downloadsStopped, "已停止"}, {downloadsFailed, "失败"}} {
				if counts[segment.filter] > 0 {
					parts = append(parts, fmt.Sprintf("%s %d", segment.label, counts[segment.filter]))
				}
			}
			if eta := s.downloadGroupETA(group.indices); eta != "" {
				parts = append(parts, eta)
			}
			group.summary = strings.Join(parts, "，")
		}
		dlScroll := ui.Scroll(c.Key("download-list")).TrackScroll(&s.DownloadScroll).Grow(1).MinHeight(0)
		dlViewport := dlScroll.Bounds()
		dlScroll.Children(func() {
			ui.Column(c).Gap(groupGap).Children(func() {
				for _, g := range groups {
					group := g
					ui.Column(c.Key("download-group-" + group.animeID)).Gap(12).Children(func() {
						collapsed := s.CollapsedDownloads[group.animeID]
						toggle := "收起"
						if collapsed {
							toggle = "展开"
						}
						header := ui.Column(c.Key("download-group-header-" + group.animeID)).Gap(4)
						header.Children(func() {
							actions := func() {
								if folder := s.groupFolder(group.animeID); folder != "" {
									compactQuiet(c, "open-group-folder-"+group.animeID, "打开番剧文件夹", func() {
										if s.OpenFolder != nil {
											s.OpenFolder(folder)
										}
									}).Disabled(s.OpenFolder == nil)
								}
								compactQuiet(c, "group-detail-"+group.animeID, "查看详情", func() { s.openFollowedResources(group.animeID, "downloads") })
								compactQuiet(c, "toggle-group-"+group.animeID, toggle, func() { s.toggleDownloadGroup(group.animeID) })
							}
							if textScale(c) > 1 {
								heading(c, s.anime(group.animeID).Title)
								caption(c, group.summary)
								intentGroupIn(c, header, dlViewport, collapsed, actions).Wrap()
							} else {
								ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
									heading(c, s.anime(group.animeID).Title)
									caption(c, group.summary)
									ui.Spacer(c)
									intentGroupIn(c, header, dlViewport, collapsed, actions)
								})
							}
						})
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
						ui.Column(c).Gap(0).Children(func() {
							for pos, i := range group.indices {
								if pos > 0 {
									ui.Divider(c).Background(pal.line)
								}
								idx := i
								d := s.Downloads[idx]
								row := ui.Column(c.Key("download-"+d.ID)).Padding(16, 12).Gap(8).Radius(radiusControl).Transition(rowTransition)
								if row.Hovered() {
									row.Background(pal.chrome)
								}
								row.Children(func() {
									ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(func() {
										ui.Text(c, d.Title).FontSize(textSize(c, textBody)).FontWeight(400).LineHeight(1.5).TextColor(pal.ink).Grow(1).MinWidth(0)
										if d.Episode != "" {
											caption(c, episodeLabel(d.Episode))
										}
										switch {
										case d.Failed:
											statusPill(c, "失败", pal.warningInk, pal.warningWash)
										case d.Stopped && d.Progress < 1:
											caption(c, "已停止")
										case d.Paused:
											caption(c, "已暂停")
										case d.Queued:
											caption(c, "排队中")
										case d.Progress >= 1:
											ui.Text(c, "已完成，分享中").FontSize(textSize(c, textMeta)).TextColor(pal.ink)
										default:
											statusPill(c, "下载中", pal.accentInk, pal.accentWash)
										}
									})
									progress := ui.Box(c.Key("transfer-progress")).Height(4)
									value := progress.Animate("value", float32(d.Progress), 400*time.Millisecond)
									progress.Children(func() { ui.Progress(c, float64(value)).Height(4).Radius(2) })
									metrics := func() {
										ui.Textf(c, "%.0f%%", d.Progress*100).FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.ink)
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
									secondaryActions := func() {
										if !d.Failed {
											if d.Paused || d.Stopped && d.Progress < 1 {
												compactQuiet(c, "pause-"+d.ID, "继续", func() { s.pauseDownload(idx) }).Disabled(s.Busy)
											} else if d.Progress < 1 {
												compactQuiet(c, "pause-"+d.ID, "暂停", func() { s.pauseDownload(idx) }).Disabled(s.Busy)
											} else if !d.Stopped {
												compactQuiet(c, "stop-seed-"+d.ID, "停止分享", func() { s.stopDownload(idx) }).Disabled(s.Busy)
											}
										}
										compactQuiet(c, "remove-"+d.ID, "删除", func() { s.RemovePrompt, s.RemoveID = true, d.ID }).Disabled(s.Busy)
										compactQuiet(c, "open-folder-"+d.ID, "打开文件夹", func() {
											if s.OpenFolder != nil {
												s.OpenFolder(d.Folder)
											}
										}).Disabled(s.OpenFolder == nil || d.Folder == "")
										if d.Progress >= 1 && !d.Failed {
											compactQuiet(c, "files-"+d.ID, "查看文件", func() { s.openFiles(idx) }).Disabled(s.Busy)
											compactQuiet(c, "play-"+d.ID, "播放", func() { s.playDownload(idx) }).TextColor(pal.accentInk).Disabled(s.Busy)
										}
									}
									actionRow := func() {
										if d.Failed {
											compactQuiet(c, "retry-"+d.ID, "重试", func() { s.retryDownload(idx) }).TextColor(pal.accentInk).Disabled(s.Busy)
										}
										intentGroupIn(c, row, dlViewport, false, secondaryActions).Wrap()
									}
									if textScale(c) > 1 {
										ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(metrics)
										ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(actionRow)
									} else {
										ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
											metrics()
											ui.Spacer(c)
											actionRow()
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
										ui.Text(c, d.Error).FontSize(textSize(c, textMeta)).TextColor(pal.warningInk).LineHeight(1.5)
									} else if d.Failed {
										ui.Text(c, "下载失败，请检查网络后重试。").FontSize(textSize(c, textMeta)).TextColor(pal.warningInk).LineHeight(1.5)
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

func (s *State) downloadHistory(c *ui.Context) {
	pal := colors(c)
	insetX := pageInsetX(c)
	topPad := float32(36)
	if textScale(c) > 1.5 {
		topPad = 24
	}
	var hidden []int
	for i, d := range s.Downloads {
		if !downloadInQueue(d) {
			hidden = append(hidden, i)
		}
	}
	ui.Column(c).Grow(1).MinHeight(0).Padding(topPad, insetX, 24).Gap(20).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
			pageTitle(c, "下载记录").Grow(1)
			compactQuiet(c, "download-history-back", "返回当前下载", func() {
				s.DownloadHistory = false
			}).TextColor(pal.accentInk)
		})
		if len(hidden) == 0 {
			s.empty(c, "还没有下载记录", "", "返回当前下载", func() {
				s.DownloadHistory = false
			})
			return
		}
		ui.Scroll(c.Key("download-history-list")).TrackScroll(&s.DownloadHistoryScroll).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Gap(0).Children(func() {
				for pos, i := range hidden {
					if pos > 0 {
						ui.Divider(c).Background(pal.line)
					}
					idx := i
					d := s.Downloads[idx]
					title := s.anime(d.AnimeID).Title
					if title == "" {
						title = d.Title
					}
					row := ui.Column(c.Key("download-history-"+d.ID)).Padding(16, 12).Gap(8).Radius(radiusControl).Transition(rowTransition)
					if row.Hovered() {
						row.Background(pal.chrome)
					}
					row.Children(func() {
						ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
							heading(c, title)
							if d.Episode != "" {
								caption(c, episodeLabel(d.Episode))
							}
							ui.Spacer(c)
							ui.PopoverMenuButton(c.Key("download-history-menu-"+d.ID), "管理", func(menu *ui.Menu) {
								if menu.Item("查看文件").Disabled(s.Busy).Chosen() {
									s.openFiles(idx)
								}
								if menu.Item("播放").Disabled(s.Busy).Chosen() {
									s.playDownload(idx)
								}
								if menu.Item("打开文件夹").Disabled(s.Busy || s.OpenFolder == nil || d.Folder == "").Chosen() {
									s.OpenFolder(d.Folder)
								}
								if menu.Item("删除或重新下载").Disabled(s.Busy).Chosen() {
									s.RemovePrompt, s.RemoveID = true, d.ID
								}
							}, ui.PopoverMenuButtonOptions{
								At:    ui.AnchorBottomEnd,
								Self:  ui.AnchorTopEnd,
								Panel: menuPanelStyle(c),
							}).Label("下载记录操作 " + d.ID).FontSize(textSize(c, textControl)).Disabled(s.Busy)
						})
						ui.Text(c, d.Title).FontSize(textSize(c, textBody)).TextColor(pal.muted).LineHeight(1.5)
						caption(c, fmt.Sprintf("已完成 %s，分享已结束", byteSizeText(sizeBytes(d.Bytes, d.MB))))
						caption(c, "保存位置："+d.Folder)
					})
				}
			})
		})
	})
}

func (s *State) settings(c *ui.Context) {
	pal := colors(c)
	insetX := pageInsetX(c)
	topPad := float32(36)
	if textScale(c) > 1.5 {
		topPad = 24
	}
	ui.Scroll(c.Key("settings-scroll")).Grow(1).MinHeight(0).Padding(topPad, insetX, 48).Children(func() {
		ui.Column(c).MaxWidth(800).Gap(40).Children(func() {
			pageTitle(c, "设置")
			ui.Column(c).Gap(16).Children(func() {
				heading(c, "外观与动效")
				s.appearanceSettings(c)
				ui.Checkbox(c.Key("reduce-effects"), &s.ReduceEffects, "减少动态效果").OnChange(s.save).Disabled(s.Busy)
			})
			ui.Column(c).Gap(16).Children(func() {
				heading(c, "下载")
				ui.Row(c).Wrap().AlignItems(ui.Center).Gap(16).Children(func() {
					ui.Text(c, "同时下载任务数").FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5).Grow(1).MinWidth(textSize(c, textBody) * 10)
					ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
						ui.TextInput(c.Key("download-limit"), &s.downloadLimitDraft).Label("填写同时下载任务数").
							Width(textSize(c, textControl) * 5).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).
							OnChange(func() { s.downloadLimitNotice = "" }).OnSubmit(s.saveDownloadLimit).Disabled(s.Busy)
						caption(c, "个")
						secondary(c, "save-download-limit", "保存", s.saveDownloadLimit).Label("保存同时下载任务数").
							Disabled(s.Busy || s.downloadLimitDraft == downloadLimitText(s.DownloadLimit))
					})
				})
				if s.downloadLimitNotice != "" {
					if s.downloadLimitError {
						ui.Text(c, s.downloadLimitNotice).FontSize(textSize(c, textMeta)).TextColor(pal.errorInk).LineHeight(1.5)
					} else {
						caption(c, s.downloadLimitNotice)
					}
				}
				ui.Column(c).Gap(8).Children(func() {
					ui.Text(c, "下载位置").FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5)
					ui.Row(c).Wrap().AlignItems(ui.Center).Gap(16).Children(func() {
						ui.Text(c, s.Root).FontSize(textSize(c, textMeta)).TextColor(pal.muted).LineHeight(1.5).Selectable().Grow(1).MinWidth(0)
						secondary(c, "choose-folder", "选择文件夹", func() {
							if s.PickFolder != nil {
								s.PickFolder()
							}
						}).Disabled(s.Busy || s.PickFolder == nil)
					})
				})
				s.trackerSettings(c)
			})
			ui.Column(c).Gap(16).Children(func() {
				heading(c, "番剧来源")
				ui.Row(c).Wrap().AlignItems(ui.Center).Gap(16).Children(func() {
					ui.Text(c, "蜜柑计划线路").FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5).Grow(1)
					ui.Select(c.Key("source-route"), &s.SourceRoute, s.routeLabels()).Label("蜜柑计划线路").Width(260).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).OnChange(func() { s.switchSourceRoute(s.SourceRoute) }).Disabled(s.Busy || s.SourceTesting)
				})
				if s.SourceTesting {
					caption(c, "正在测试新线路…")
				} else if s.SourceRouteError != "" {
					ui.Text(c, s.SourceRouteError).FontSize(textSize(c, textMeta)).TextColor(pal.errorInk).LineHeight(1.5)
				}
			})
			ui.Column(c).Gap(16).Children(func() {
				heading(c, "上传与分享")
				s.transferSettings(c)
			})
			ui.Column(c).Gap(16).Children(func() {
				heading(c, "网络与代理")
				s.proxySettings(c)
			})
			s.windowSettings(c)
			s.softwareUpdateSettings(c)
		})
	})
}

func (s *State) filesDialog(c *ui.Context) {
	pal := colors(c)
	ui.DialogBase(c, &s.FilesOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.scrim).Transition(fadeTransition)
		dialogContentWithFooter(c, panel, "files-content", 580, radiusDialog, func() {
			var d Download
			for _, v := range s.Downloads {
				if v.ID == s.FilesID {
					d = v
					break
				}
			}
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				pageTitle(c, d.Title).Grow(1).MinWidth(0)
				iconButton(c, "close-files", "关闭文件列表", "close", func() { s.FilesOpen = false })
			})
			switch {
			case s.FilesError != "":
				ui.Text(c, s.FilesError).FontSize(textSize(c, textMeta)).TextColor(pal.errorInk).LineHeight(1.5)
			case s.FilesLoading:
				caption(c, "正在读取文件…")
			case len(s.Files) == 0:
				caption(c, "没有找到文件，可能已被移动或删除。")
			default:
				ui.Column(c).Gap(0).Children(func() {
					for pos, f := range s.Files {
						if pos > 0 {
							ui.Divider(c).Background(pal.line)
						}
						name := filepath.Base(f.Path)
						dir := filepath.Dir(f.Path)
						row := ui.Row(c.Key("file-"+name)).Wrap().AlignItems(ui.Center).Gap(12).Padding(12, 8).Radius(radiusSmall)
						if row.Hovered() {
							row.Background(pal.chrome)
						}
						row.Children(func() {
							ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
								ui.Text(c, name).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5)
								caption(c, dir)
								caption(c, exactByteText(f.Size))
							})
							compactQuiet(c, "copy-path-"+name, "复制路径", func() { s.copyPath(f.Path) })
							if isPlayableMedia(f.Path) {
								compactQuiet(c, "play-file-"+name, "播放此文件", func() { s.playFile(f.Path) }).Label("播放此文件 " + f.Path).TextColor(pal.accentInk)
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
	pal := colors(c)
	ui.DialogBase(c, &s.Help, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.scrim).Transition(fadeTransition)
		dialogContentWithFooter(c, panel, "help-content", 520, radiusDialog, func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				pageTitle(c, "怎么追番").Grow(1).MinWidth(0)
				iconButton(c, "close-help", "关闭指南", "close", func() { s.Help = false })
			})
			for i, step := range []struct{ title, body string }{
				{"找到想看的番剧", "按季度和星期浏览，点海报打开番剧。"},
				{"选择观看版本", "按画质、字幕和文件大小挑选，也可以按原标题追番。"},
				{"开启追番", "以后发布的资源会自动下载。在「我的追番」点海报，从剧集库播放下载完成的剧集。"},
			} {
				ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
					ui.Textf(c, "%d", i+1).FontSize(textSize(c, textControl)).FontWeight(600).TextColor(pal.accentInk).Background(pal.accentWash).Padding(4, 12).Radius(radiusControl)
					ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
						ui.Text(c, step.title).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5)
						caption(c, step.body)
					})
				})
			}
		}, func() {
			primary(c, "got-it", "关闭指南", func() { s.Help = false })
		})
	})
}

func (s *State) titlePreferences(c *ui.Context) {
	pal := colors(c)
	ui.Column(c.Key("title-preferences")).FillWidth().Gap(16).Transition(rowTransition).Children(func() {
		ui.Text(c, "按此资源筛选").FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink)
		ui.Column(c).Gap(16).Children(func() {
			for i, word := range s.SuggestedResource.Tokens {
				w := word
				ui.Column(c).Gap(8).Children(func() {
					ui.Text(c, w).FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.ink)
					ui.Row(c).Gap(4).Children(func() {
						for _, option := range []struct {
							label     string
							choice    wordChoice
							color, bg ui.Color
						}{{"不限", wordUnrestricted, pal.ink, pal.canvas}, {"＋ 只要", wordWant, pal.accentInk, pal.accentWash}, {"− 不要", wordAvoid, pal.muted, pal.sunken}} {
							o := option
							b := compactSecondary(c, fmt.Sprintf("suggested-%d-%d", i, o.choice), o.label, func() { s.setWord(w, o.choice) }).Label(o.label + " " + w).Grow(1).Justify(ui.Center)
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
