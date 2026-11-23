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
	"close":     `<path d="m6 6 12 12M6 18 18 6"/>`,
	"minimize":  `<path d="M5 12h14"/>`,
	"maximize":  `<rect x="5" y="5" width="14" height="14" rx="1"/>`,
	"restore":   `<path d="M8 8h11v11H8zM5 16V5h11"/>`,
	"pause":     `<path d="M8 5v14M16 5v14"/>`,
	"play":      `<path d="m8 4 12 8-12 8z"/>`,
	"clock":     `<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`,
	"refresh":   `<path d="M20 10a8 8 0 0 0-14-4L3 9m0-6v6h6M4 14a8 8 0 0 0 14 4l3-3m0 6v-6h-6"/>`,
	"download":  `<path d="M12 3v14m-5-5 5 5 5-5M4 21h16"/>`,
	"upload":    `<path d="M12 20V6m-5 5 5-5 5 5M4 3h16"/>`,
}
var icons = map[string]*ui.SVG{}

func loadImages() {
	brandLogo = ui.MustParseSVG(brandSVG)
	var err error
	if brandApp, err = ui.DecodeBitmap(brandIconPNG); err != nil {
		panic("decode app icon: " + err.Error())
	}
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
func windowControl(c *ui.Context, key, label, name string, danger bool, action func()) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Size(46, 32).Shrink(0).Center().Cursor(ui.CursorPointer).TextColor(colors(c).ink)
	if b.Hovered() {
		b.Background(colors(c).line)
		if danger {
			b.Background(ui.Hex("#c42b1c")).TextColor(ui.Hex("#ffffff"))
		}
	}
	return tactile(b).Children(func() { icon(c, name) }).OnClick(action)
}
func caption(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted).LineHeight(1.5)
}
func badge(c *ui.Context, text string, col ui.Color, bg ui.Color) ui.Element {
	return ui.Badge(c, text).FontWeight(600).TextColor(col).Background(bg).Padding(3, 7)
}
func heading(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(textSize(c, textSection)).FontWeight(600).TextColor(colors(c).ink)
}

const (
	coverRadius       = 8
	browseCoverRadius = coverRadius
)

const topbarHeight float32 = 64
const sidebarWidth float32 = 204

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
	drawCover(c, bitmap, key != "", showLoading, width, height, browseCoverRadius)
}

func coverCanLoad(stage coverStage) bool {
	return stage != coverMiss && stage != coverFailed
}

func drawCover(c *ui.Context, b *ui.Bitmap, hasURL, showLoading bool, w, h, radius float32) {
	if b != nil {
		ui.Image(c, b).Size(w, h).Fit(ui.Contain).Background(colors(c).sunken).Radius(radius)
	} else if hasURL {
		ui.Box(c).Size(w, h).Radius(radius).Clip().Background(colors(c).sunken).Center().Children(func() {
			if showLoading {
				loading.Blocks(c, loading.Options{Size: 26, Color: colors(c).muted}).Label("正在加载封面")
			}
		})
	} else {
		size := min(w, h) * 0.62
		ui.Box(c).Size(w, h).Radius(radius).Clip().Background(colors(c).sunken).Center().Children(func() { ui.Image(c, brandLogo).Size(size, size).Background(colors(c).brandWash).Radius(8) })
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
				ui.Text(c, s.SaveError).FontSize(textSize(c, textCaption)).TextColor(colors(c).errorInk).Background(colors(c).errorWash).Padding(8, 24)
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
	selected := s.Page
	switch selected {
	case "detail":
		selected = "browse"
	case "episodes":
		selected = "following"
	}
	active := selected
	navigate := func(page string) {
		s.enterPage(page)
		s.Notice = ""
	}
	navItem := func(id, label string) ui.Element {
		item := ui.SidebarItem(c, id, icons[id], label).Label(label).MinHeight(44)
		if id == active {
			item.FontWeight(600).OnClick(func() { navigate(id) })
		}
		return item
	}
	theme := *c.Theme()
	theme.SurfacePressed = pal.blueWash
	sidebar := ui.Themed(c, &theme).Width(sidebarWidth).FillHeight().Shrink(0)
	sidebar.Children(func() {
		ui.Sidebar(c.Key("sidebar-scroll"), &selected, func() {
			ui.Column(c).MinHeight(height).Padding(24, 14, 16).Gap(8).Children(func() {
				ui.Row(c).Gap(10).Padding(0, 4, 20).Children(func() {
					ui.Image(c, brandApp).Size(40, 40)
					ui.Text(c, "Dipper").FontSize(textSize(c, textSection)).FontWeight(600).TextColor(pal.ink).Grow(1).MinWidth(0)
				})
				navItem("browse", "浏览番剧")
				following := navItem("following", "我的追番")
				failed := 0
				for _, sub := range s.Subscriptions {
					if sub.CheckFailed {
						failed++
					}
				}
				sidebarCount(c, following, len(s.Subscriptions), failed)
				downloads := navItem("downloads", "下载")
				count, failed := 0, 0
				for _, download := range s.Downloads {
					if downloadInQueue(download) {
						count++
						if download.Failed {
							failed++
						}
					}
				}
				sidebarCount(c, downloads, count, failed)
				ui.Spacer(c)
				ui.Divider(c).Background(pal.line)
				navItem("settings", "设置")
				quiet(c, "help", "追番小指南", func() { s.Help = true }).FontSize(textSize(c, textCaption)).TextColor(pal.muted)
			})
		}).Fill().Padding(0).Background(pal.chrome).
			BorderWidth(0, 1, 0, 0).BorderColor(pal.line).OnChange(func() { navigate(selected) })
	})
}

func sidebarCount(c *ui.Context, item ui.Element, count, failed int) {
	if count == 0 {
		return
	}
	label := strconv.Itoa(count)
	if failed > 0 {
		label = fmt.Sprintf("%d 失败", failed)
	}
	item.Children(func() {
		render := func() {
			badge := ui.Badge(c, label)
			if failed > 0 {
				badge.TextColor(colors(c).amber).Background(colors(c).amberWash)
			}
		}
		if textScale(c) > 1 {
			item.Wrap()
			ui.Row(c).FillWidth().PaddingStart(24).Children(render)
			return
		}
		render()
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
	ui.Row(c).MinHeight(topbarHeight).Shrink(0).Padding(12, rightPadding, 12, 28).Gap(12).
		Background(pal.canvas).
		DragWindow().Children(func() {
		ui.Text(c, names[s.Page]).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(pal.ink).MinWidth(0).Grow(1)
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
	ui.Row(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
		s.browseCatalog(c, windowWidth-sidebarWidth-detailWidth, detailWidth > 0)
		if detailWidth > 0 {
			ui.Column(c).Width(detailWidth).Shrink(0).FillHeight().MinHeight(0).
				Background(colors(c).paper).BorderWidth(0, 0, 0, 1).BorderColor(colors(c).line).
				Children(func() { s.browseDetailContent(c, s.browseDetail) })
		}
	})
}

func (s *State) browseCatalog(c *ui.Context, width float32, split bool) {
	pal := colors(c)
	ui.Column(c).Width(width).Shrink(0).FillHeight().MinHeight(0).Padding(24, 28, 0).Children(func() {
		ui.Text(c, s.Year+" 年"+s.Season+"季番剧").FontSize(textSize(c, textTitle)).FontWeight(600).TextColor(pal.ink).Margin(0, 0, 18, 0).ContentTransition(ui.ContentTransition{Animation: &contentMotion})
		if s.SourceError != "" && !s.catalogFromDisk && len(s.Catalog) == 0 {
			ui.Row(c).Wrap().Gap(controlGap).Margin(0, 0, 14, 0).Children(func() {
				quiet(c, "catalog-error-previous", "上一季", func() { s.changeSeason(-1) }).Disabled(!s.canChangeSeason(-1))
				quiet(c, "catalog-error-next", "下一季", func() { s.changeSeason(1) }).Disabled(!s.canChangeSeason(1))
				quiet(c, "catalog-error-current", "回到当季", s.currentSeason)
			})
			s.empty(c, "来源暂时无法连接", s.SourceError, "重试加载", s.refreshCatalog)
			return
		}
		s.browseToolbar(c, split)
		s.browseDays(c)
		if s.SourceError != "" && len(s.Catalog) > 0 {
			ui.Row(c).Wrap().Gap(controlGap).Margin(0, 0, 14, 0).Children(func() {
				ui.Text(c, "正在显示上次加载的番剧").FontSize(textSize(c, textBody)).TextColor(pal.ink)
				quiet(c, "catalog-cache-retry", "重试加载", s.refreshCatalog)
			})
		}
		if !s.IntroHidden {
			ui.Row(c.Key("beginner-hint")).Wrap().Padding(8, 12).Gap(8).Radius(controlRadius).Background(pal.chrome).Margin(0, 0, 16, 0).Transition(ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Exit: &ui.Motion{Collapse: true}, Size: true}).Children(func() {
				ui.Text(c, "第一次追番？").FontSize(textSize(c, textBody)).FontWeight(600).Grow(1)
				quiet(c, "intro-help", "了解怎么追", func() { s.Help = true }).TextColor(pal.ink).FontSize(textSize(c, textCaption))
				iconButton(c, "dismiss-intro", "收起新手提示", "close", func() { s.IntroHidden = true; s.save() }).Disabled(s.Busy)
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
				title, body := "没有找到这部番剧", "试试缩短番名，或切换到全部星期。"
				if strings.TrimSpace(s.Search) == "" {
					title = "暂无番剧"
					body = "当前季度或星期暂无番剧，试试切换季度或查看全部星期。"
				}
				s.empty(c, title, body, "清除筛选", func() { s.Search = ""; s.Day = -1; s.resetBrowseScroll() })
			} else {
				cols, cardWidth := gridColumns(width + sidebarWidth)
				key, scroll := "catalog", &s.CatalogScroll
				if split {
					key, scroll = "catalog-split", &s.browseDetail.catalogScroll
				}
				ui.Scroll(c.Key(key)).TrackScroll(scroll).Grow(1).MinHeight(0).Padding(0, 0, 24).Children(func() {
					ui.Grid(c).Columns(cols).GapX(18).GapY(24).Children(func() {
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
	year, _ := strconv.Atoi(s.Year)
	firstSeason := firstCatalogSeasonIndex(year)
	searchWidth := max(float32(220), textSize(c, textBody)*16)
	if split {
		searchWidth = max(180, textSize(c, textBody)*13)
	}
	ui.Box(c).FillWidth().Margin(0, 0, 12, 0).Children(func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(10).Children(func() {
			iconButton(c, "prev-season", "上一季", "back", func() { s.changeSeason(-1) }).Disabled(!s.canChangeSeason(-1))
			ui.Select(c.Key("year"), &s.Year, catalogYears()).Label("年份").Width(max(88, textSize(c, textBody)*5)).MinHeight(inputHeight(c)).Radius(controlRadius).Background(pal.paper).OnChange(s.changeCatalogYear)
			selected := slices.Index(seasons, s.Season)
			quarters := ui.SegmentedBase(c.Key("season"), &selected, len(seasons))
			quarters.Track.Label("季度").Gap(2).Padding(3).Radius(controlRadius).Background(pal.sunken).Children(func() {
				for index, season := range seasons {
					b := quarters.Segment(index).Label(season+"季").Padding(6, 12).Radius(controlRadius - 2).Cursor(ui.CursorPointer).TextColor(pal.muted)
					if selected == index {
						b.Background(pal.paper).TextColor(pal.ink).FontWeight(600)
					} else if b.Hovered() {
						b.TextColor(pal.ink)
					}
					tactile(b).Children(func() { ui.Text(c, season+"季").SingleLine() }).Disabled(index < firstSeason)
				}
			}).OnChange(func() {
				s.Season = seasons[selected]
				s.refreshCatalog()
			})
			iconButton(c, "next-season", "下一季", "next", func() { s.changeSeason(1) }).Disabled(!s.canChangeSeason(1))
			ui.Spacer(c)
			ui.SearchField(c.Key("search"), &s.Search).Label("搜索番剧").Placeholder("搜索本季番剧").Width(searchWidth).MinHeight(inputHeight(c)).OnChange(s.resetBrowseScroll)
		})
	})
}

func (s *State) browseDays(c *ui.Context) {
	pal := colors(c)
	ui.Row(c).Wrap().AlignItems(ui.Center).Gap(2).Margin(0, 0, 14, 0).Children(func() {
		days := []int{-1, 1, 2, 3, 4, 5, 6, 0}
		selected := slices.Index(days, s.Day)
		weekdays := ui.SegmentedBase(c.Key("weekday"), &selected, len(days))
		weekdays.Track.Label("星期").Wrap().Shrink(1).MinWidth(0).Gap(2).Children(func() {
			for index, label := range []string{"全部", "周一", "周二", "周三", "周四", "周五", "周六", "周日"} {
				filterSegment(c, weekdays.Segment(index), label, label, selected == index)
			}
		}).OnChange(func() {
			s.Day = days[selected]
			s.resetBrowseScroll()
		})
		ui.Spacer(c)
		caption(c, "来源：蜜柑计划")
		ui.Text(c, "·").FontSize(textSize(c, textCaption)).TextColor(pal.outline).MarginX(2)
		quiet(c, "current-season", "回到当季", s.currentSeason).Padding(4, 8).FontSize(textSize(c, textCaption)).TextColor(pal.ink)
	})
}

func filterSegment(c *ui.Context, segment ui.Element, label, text string, active bool) {
	pal := colors(c)
	segment.Label(label).Padding(6, 10).Radius(controlRadius).Shrink(0).Cursor(ui.CursorPointer).TextColor(pal.muted)
	if active {
		segment.Background(pal.blueWash).TextColor(pal.blue).FontWeight(600)
	} else if segment.Hovered() {
		segment.Background(pal.chrome).TextColor(pal.ink)
	}
	tactile(segment).Children(func() { ui.Text(c, text).SingleLine() })
}

func (s *State) animeCard(c *ui.Context, a Anime, width float32) ui.Element {
	pal := colors(c)
	b := ui.ButtonBase(c.Key("anime-" + a.ID)).Label("查看 " + a.Title).Width(width).Column().Justify(ui.Start).Gap(10).Padding(0).AlignItems(ui.Stretch).Radius(browseCoverRadius).Cursor(ui.CursorPointer)
	selected := s.browseDetail != nil && s.browseDetail.anime.ID == a.ID
	b.Expanded(selected)
	pose := ui.Pose{}
	if b.Hovered() {
		pose.Y = -2
	}
	if b.Pressed() {
		pose.Y = 0
	}
	b.AnimatePose(pose, contentMotion)
	return b.Children(func() {
		ui.Box(c).Radius(browseCoverRadius).
			Shadow(0, 2, 6, -2, pal.cardShadow).Children(func() {
			s.posterCover(c, b, a, width)
			if s.isFollowingAnime(a.ID) {
				badge(c, "已追番", pal.ink, pal.paper).Border(1, pal.line).Absolute().Top(8).Left(8)
			}
			if selected {
				ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).Radius(browseCoverRadius).Border(2, pal.blue)
			}
			if b.Hovered() || b.FocusVisible() {
				ui.Row(c).Absolute().Bottom(6).Left(6).Right(6).Center().Gap(6).
					Padding(8, 4).Radius(6).Background(pal.paper).TextColor(pal.ink).Children(func() {
					ui.Text(c, "查看详情").FontSize(textSize(c, textBody)).FontWeight(600)
					icon(c, "next")
				})
			}
		})
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, a.Title).FontSize(textSize(c, textBody)).FontWeight(600).LineHeight(1.4).TextColor(pal.ink)
			if s.Day == -1 {
				ui.Text(c, dayName(a.Day)).FontSize(textSize(c, textCaption)).TextColor(pal.muted)
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
	ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
		ui.Spinner(c).TextColor(colors(c).muted)
		caption(c, text)
	})
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
					s.detailOptions(c)
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
			} else if s.DetailReturn == browseDetailReturn {
				backLabel = "返回番剧详情"
			}
			iconButton(c, "back-browse", backLabel, "back", func() { s.enterPage(backPage) })
			quiet(c, "back-list", backLabel, func() { s.enterPage(backPage) }).FontSize(textSize(c, textCaption)).TextColor(colors(c).muted)
		})
		ui.Row(c).Wrap().Gap(controlGap).AlignItems(ui.Start).Children(func() {
			s.featuredCover(c, a, 52, 72)
			ui.Column(c).Grow(1).MinWidth(0).Gap(7).Children(func() {
				ui.Text(c, a.Title).FontSize(textSize(c, textTitle)).FontWeight(600)
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

func (s *State) detailOptions(c *ui.Context) {
	ui.Column(c).Gap(14).Children(func() {
		s.detailGroups(c)
		if s.detailReady() {
			ui.Divider(c).Background(colors(c).line).Margin(2, 0)
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
			if s.detailReady() && (!s.VersionSetup || !s.TitleSetup) {
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
		} else if !s.hasGroup() && s.GroupCountLoading {
			pageLoading(c, "正在比较各字幕组的资源数量，也可以直接选择字幕组。")
		} else if !s.hasGroup() {
			ui.Column(c).Grow(1).Center().Children(func() {
				caption(c, "先选择一个字幕组，即可预览发布资源。")
			})
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
		}{{"只要这些词", "标题里必须都有", s.Rule.All, colors(c).blue}, {"不要这些词", "标题里出现任意一个就不下载", s.Rule.None, colors(c).muted}} {
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
		ui.Field(c.Key("keyword-field"), "标题关键词", func() {
			ui.TextInput(c.Key("keyword"), &s.Keyword).Placeholder("输入标题中的词").FillWidth().MinHeight(inputHeight(c))
		})
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			secondary(c, "want-custom", "只要这个词", func() { s.setWord(s.Keyword, wordWant); s.Keyword = "" }).FontSize(textSize(c, textBody)).Padding(6, 10).TextColor(colors(c).blue).Disabled(strings.TrimSpace(s.Keyword) == "")
			secondary(c, "avoid-custom", "不要这个词", func() { s.setWord(s.Keyword, wordAvoid); s.Keyword = "" }).FontSize(textSize(c, textBody)).Padding(6, 10).TextColor(colors(c).muted).Disabled(strings.TrimSpace(s.Keyword) == "")
		})
		aliases := ui.CollapsibleBase(c.Key("toggle-aliases"), &s.ShowAliases)
		trigger := aliases.Trigger.Label("添加同义写法").FontSize(textSize(c, textCaption)).
			TextColor(colors(c).blue).Padding(3, 0).Cursor(ui.CursorPointer)
		tactile(trigger).Children(func() { ui.Text(c, "添加同义写法") })
		if s.ShowAliases {
			aliases.Panel(func() {
				ui.Field(c.Key("alias-field"), "同义写法", func() {
					ui.TextInput(c.Key("aliases"), &s.AnyDraft).Placeholder("简日内嵌 / CHS").FillWidth().MinHeight(inputHeight(c))
				}).Description("用 / 分隔同一类标签的不同写法，例如：简日内嵌 / CHS。")
				secondary(c, "add-alias-group", "添加这组写法", s.addAliases).FontSize(textSize(c, textBody)).Padding(6, 10).Disabled(strings.TrimSpace(s.AnyDraft) == "")
			}).Gap(8)
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
		col, bg = colors(c).muted, colors(c).sunken
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
		col, bg = colors(c).muted, colors(c).sunken
	}
	b := quiet(c, key, label, func() { s.WordAnchor = key; s.WordOpen = true }).Label("筛选 "+word).FontSize(textSize(c, textCaption)).Padding(5, 8).Radius(5).Border(1, colors(c).line).Background(bg).TextColor(col).Tooltip("选择只要、不要，或移除条件")
	if s.WordAnchor == key {
		s.wordPopover(c, b, key, word)
	}
}

func (s *State) wordPopover(c *ui.Context, anchor ui.Element, key, word string) {
	if !s.WordOpen {
		return
	}
	choice := s.wordChoice(word)
	selected := choice
	ui.Popover(c.Key("word-popup-"+key), anchor, &s.WordOpen, func() {
		ui.Text(c, "筛选「"+word+"」").FontSize(textSize(c, textBody)).FontWeight(600).Padding(4, 6)
		ui.RadioGroup(c, func() {
			for _, option := range []struct {
				label, action, description string
				choice                     wordChoice
				color, bg                  ui.Color
			}{
				{"只要包含", "只要含", "标题里必须有这个词", wordWant, colors(c).blue, colors(c).blueWash},
				{"不要包含", "不要含", "标题里有这个词就不下载", wordAvoid, colors(c).muted, colors(c).sunken},
			} {
				count := s.countRuleMatches(ruleWithWord(s.Rule, word, option.choice))
				radio := ui.RadioBase(c.Key(fmt.Sprintf("word-choice-%d", option.choice)), &selected, option.choice).
					Label(option.action+"「"+word+"」的").Column().FillWidth().Gap(5).Padding(12).
					Radius(6).Border(1, colors(c).line).Background(colors(c).paper).Cursor(ui.CursorPointer)
				if radio.Hovered() || choice == option.choice {
					radio.Background(option.bg).BorderColor(option.color)
				}
				tactile(radio).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, option.label).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(option.color)
						ui.Spacer(c)
						ui.Textf(c, "剩 %d 条", count).FontSize(textSize(c, textCaption)).TextColor(option.color)
					})
					caption(c, option.description)
				})
				bindRadio(radio, choice == option.choice, func() { s.setWord(word, option.choice) })
			}
		}).Label("标题词条件").Gap(6)
		if choice != wordUnrestricted {
			quiet(c, "unrestrict-word", "不再限制这个词", func() { s.setWord(word, wordUnrestricted) }).Label("不再限制「" + word + "」").FontSize(textSize(c, textCaption)).FillWidth()
		}
		caption(c, "只匹配原标题，不判断视频内容。")
	}).Width(320).MinWidth(0).Padding(10).Gap(6).Background(colors(c).paper).
		Shadow(0, 6, 20, 0, colors(c).popoverShadow).Transition(popupTransition)
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
	manual := s.VersionSetup && s.TitleSetup
	if s.VersionSetup {
		if manual {
			caption(c, "点原标题里的词，看含义和筛选结果。")
		} else {
			s.versionSummary(c)
			if s.Rule.Version == nil {
				return
			}
		}
	} else {
		s.quickFilter(c)
	}
	n, eps, _ := s.matchSummary()
	matches := s.matchingResources()
	choices, byEpisode := s.episodeChoices(matches)
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
	if (!manual || hasTitleConditions(s.Rule)) && s.Rule.Version == nil && len(s.Rule.All) == 0 && len(s.Rule.Any) == 0 {
		if len(s.Rule.None) == 0 {
			caption(c, "还没限制版本，这组全部资源都会匹配。")
		} else {
			caption(c, "只排除写出这些词的标题。标题没写的，也会匹配。")
		}
	}
	if manual {
		caption(c, "只追后续更新，按标题里的实际写法匹配。")
	} else if s.Backfill || s.ViewResources {
		caption(c, "勾选要下载的资源。筛选不会修改追番条件。")
	} else if s.Editing {
		caption(c, "调整条件只影响以后发布的资源；已经看过或跳过的不会自动下载。")
	} else if s.selectsOnePerEpisode() {
		caption(c, "每集默认选一份。同集有多个资源时，可以换一个。")
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
	if !s.Editing && !manual {
		selected, _ := s.selected()
		all := ui.CheckOff
		switch {
		case len(selected) > 0 && len(selected) == len(choices):
			all = ui.CheckOn
		case len(selected) > 0:
			all = ui.CheckMixed
		}
		label := "全选匹配资源"
		if s.selectsOnePerEpisode() {
			label = "全选符合条件的集数"
		}
		ui.Row(c).Children(func() {
			ui.CheckboxState(c.Key("select-all"), &all, label).FontSize(textSize(c, textBody)).Disabled(n == 0).OnChange(func() {
				for _, resource := range choices {
					s.setResourceChecked(resource, all == ui.CheckOn)
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
				s.resourceRow(c, res, byEpisode[res.Episode].ID == res.ID)
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

func (s *State) resourceRow(c *ui.Context, r Resource, episodeChoice bool) {
	ok, reason := matchResource(r, s.Rule)
	ui.Column(c.Key("resource-"+r.ID)).Padding(12, 0).Gap(7).BorderWidth(0, 0, 1, 0).BorderColor(colors(c).line).Transition(rowTransition).Children(func() {
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			on := s.Checked[r.ID] && ok
			if !s.Editing && !(s.VersionSetup && s.TitleSetup) {
				ui.Checkbox(c.Key("pick-"+r.ID), &on, "").Label("选择 " + r.ID).Disabled(!ok).OnChange(func() { s.setResourceChecked(r, on) })
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
				label := fmt.Sprintf("本集匹配 %d 条", count)
				if s.EpisodeFilter == r.Episode {
					fg, bg = colors(c).blue, colors(c).blueWash
				}
				if s.selectsOnePerEpisode() {
					label = fmt.Sprintf("本集匹配 %d 个版本 · 换一个", count)
					if s.EpisodeFilter == ep {
						label = "返回全部集数"
					}
				}
				if !s.selectsOnePerEpisode() || episodeChoice {
					quiet(c, "filter-episode-"+r.ID, label, func() { s.toggleEpisodeFilter(ep) }).FontSize(textSize(c, textCaption)).FontWeight(600).TextColor(fg).Background(bg).Radius(4).Padding(3, 7)
				}
			}
			if d, idx, found := s.downloadForResource(r); found {
				switch {
				case d.Failed:
					badge(c, "失败", colors(c).amber, colors(c).amberWash)
				case d.Progress < 1 && d.Stopped:
					badge(c, "已停止", colors(c).muted, colors(c).canvas)
				case d.Progress < 1 && d.Paused:
					badge(c, "已暂停", colors(c).muted, colors(c).canvas)
				case d.Queued:
					badge(c, "排队中", colors(c).muted, colors(c).canvas)
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
		if s.VersionSetup && s.TitleSetup {
			s.originalTitle(c, r)
		} else {
			ui.Text(c, r.Title).FontSize(textSize(c, textBody)).LineHeight(1.6).Selectable().TextColor(colors(c).ink)
		}
		published := r.Published
		if published == "" {
			published = "发布时间未知"
		}
		caption(c, published)
		if !s.VersionSetup {
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
		}
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
					} else if !s.canSaveDetailRule() {
						title = "选好版本后即可追番"
						if s.TitleSetup {
							title = "添加标题条件后即可追番"
						}
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
				primary(c, "start-follow", label, s.commit).Disabled(!s.canSaveDetailRule() || s.Busy || (s.Backfill && len(resources) == 0))
			})
			if s.Backfill {
				if sub, ok := s.subscription(s.EditID); ok && sub.Paused {
					caption(c, "追番已暂停；现在只会排队，恢复追番后开始下载。")
				}
			} else if !s.Editing && !s.ViewResources && textScale(c) <= 1 {
				if s.Rule.Version != nil {
					caption(c, "以后发布的同版本资源会自动下载。")
				} else if !s.VersionSetup {
					caption(c, "以后发布的资源会按这些标题条件自动下载。")
				}
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
			ui.Text(c, "确认取消追番").FontSize(textSize(c, textTitle)).FontWeight(600)
			heading(c, s.anime(sub.AnimeID).Title)
			ui.Column(c).Padding(16).Radius(8).Background(colors(c).canvas).Gap(8).Children(func() {
				ui.Text(c, "字幕组："+group).FontSize(textSize(c, textBody))
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
			ui.Text(c, "删除任务和文件？").FontSize(textSize(c, textTitle)).FontWeight(600)
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

// retryableChecks names the follows the batch retry would actually check.
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
	ui.Column(c).Grow(1).MinHeight(0).Padding(24, 28).Gap(20).Children(func() {
		ui.Text(c, "我的追番").FontSize(textSize(c, textTitle)).FontWeight(600)
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			ui.SearchField(c.Key("follow-search"), &s.FollowSearch).Label("搜索追番").Placeholder("搜索已追番剧").Width(240).MinHeight(inputHeight(c)).OnChange(func() {
				s.FollowScroll = ui.ScrollState{}
			})
			if len(s.retryableChecks()) > 0 {
				quiet(c, "retry-failed-checks", "重试检查", s.retryFailedChecks).TextColor(colors(c).amber).Disabled(s.Busy)
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
		caption(c, fmt.Sprintf("%d 部追番", len(subs)))
		if len(subs) == 0 {
			s.empty(c, "没有找到这部追番", "试试缩短番名，或清除搜索。", "清除搜索", func() {
				s.FollowSearch = ""
				s.FollowScroll = ui.ScrollState{}
			})
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
	pal := colors(c)
	tile := ui.ButtonBase(c.Key("subscription-" + sub.ID)).Label("打开追番 " + sub.GroupName).Width(width).Column().Justify(ui.Start).Gap(10).Padding(0).AlignItems(ui.Stretch).Radius(browseCoverRadius).Cursor(ui.CursorPointer)
	pose := ui.Pose{}
	if tile.Hovered() {
		pose.Y = -2
	}
	if tile.Pressed() {
		pose.Y = 0
	}
	tile.AnimatePose(pose, contentMotion)
	tile.Children(func() {
		ui.Box(c).Radius(browseCoverRadius).
			Shadow(0, 2, 6, -2, pal.cardShadow).Children(func() {
			s.posterCover(c, tile, a, width)
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
				} else if sub.Queued {
					badge(c, "排队中", colors(c).muted, colors(c).canvas)
				} else {
					status := badge(c, "自动追番中", pal.green, pal.greenWash)
					if !tile.Hovered() {
						status.Opacity(0)
					}
				}
			})
		})
		ui.Text(c, a.Title).FontSize(textSize(c, textBody)).FontWeight(600).LineHeight(1.4).TextColor(pal.ink)
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
		ui.Row(c).Padding(12, 16, 0).Children(func() {
			back := ui.ButtonBase(c.Key("back-following")).Label("返回我的追番").
				Padding(8, 12).Gap(6).AlignItems(ui.Center).Radius(controlRadius).
				FontSize(textSize(c, textCaption)).TextColor(colors(c).muted).Cursor(ui.CursorPointer)
			if back.Hovered() {
				back.Background(colors(c).chrome)
			}
			tactile(back).Children(func() {
				icon(c, "back")
				ui.Text(c, "返回我的追番")
			}).OnClick(func() { s.enterPage("following") })
		})
		s.libraryHero(c, sub)
		ui.Divider(c).Background(colors(c).line)
		if len(indices) == 0 {
			s.empty(c, "还没有下载完成的剧集", "正在下载的剧集也会显示；完成后就能播放。", "查看资源", func() { s.viewSubscriptionResources(sub.ID) })
			return
		}
		ui.Scroll(c.Key("episode-list")).Grow(1).MinHeight(0).Padding(0, 28, 24).Children(func() {
			for position, index := range indices {
				if position > 0 {
					ui.Divider(c).Background(colors(c).line)
				}
				s.libraryEpisodeRow(c, index)
			}
		})
	})
}

func (s *State) libraryEpisodeRow(c *ui.Context, index int) {
	download := s.Downloads[index]
	palette := colors(c)
	ui.Row(c.Key("episode-"+download.ID)).AlignItems(ui.Center).Gap(16).Padding(14, 0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, episodeLabel(download.Episode)).FontSize(textSize(c, textBody)).FontWeight(600)
				switch {
				case download.Failed:
					badge(c, "失败", palette.amber, palette.amberWash)
				case download.Progress >= 1:
					caption(c, "已完成")
				case download.Stopped:
					caption(c, "已停止")
				case download.Paused:
					caption(c, "已暂停")
				case download.Queued:
					caption(c, "排队中")
				default:
					caption(c, fmt.Sprintf("下载中 %d%%", int(download.Progress*100)))
				}
			})
			caption(c, download.Title)
			if download.Error != "" {
				ui.Text(c, download.Error).FontSize(textSize(c, textCaption)).TextColor(palette.amber)
			}
		})
		switch {
		case download.Failed:
			secondary(c, "retry-episode-"+download.ID, "重试", func() { s.retryDownload(index) }).
				Label("重试 " + download.ID).MinHeight(40).Shrink(0).Disabled(s.Busy)
		case download.Progress < 1 && (download.Paused || download.Stopped):
			secondary(c, "resume-episode-"+download.ID, "继续", func() { s.pauseDownload(index) }).
				Label("继续 " + download.ID).MinHeight(40).Shrink(0).Disabled(s.Busy)
		case download.Progress >= 1:
			ui.Row(c).AlignItems(ui.Center).Gap(8).Shrink(0).Children(func() {
				primary(c, "play-episode-"+download.ID, "播放", func() { s.playDownload(index) }).
					Label("播放 " + download.ID).MinWidth(72).MinHeight(40).Disabled(s.Busy)
				iconButton(c, "open-episode-folder-"+download.ID, "打开文件夹", "folder", func() {
					if s.OpenFolder != nil {
						s.OpenFolder(download.Folder)
					}
				}).Label("打开文件夹 "+download.ID).Size(40, 40).Disabled(s.Busy || s.OpenFolder == nil || download.Folder == "")
				ui.PopoverMenuButton(c.Key("episode-menu-"+download.ID), "更多", func(menu *ui.Menu) {
					if menu.Item("查看文件").Disabled(s.Busy).Chosen() {
						s.openFiles(index)
					}
					if menu.Item("删除或重新下载").Disabled(s.Busy).Chosen() {
						s.RemovePrompt, s.RemoveID = true, download.ID
					}
				}, ui.PopoverMenuButtonOptions{At: ui.AnchorBottomEnd, Self: ui.AnchorTopEnd}).
					Label("剧集操作 " + download.ID).FontSize(textSize(c, textCaption)).MinHeight(40).Disabled(s.Busy)
			})
		}
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
	return downloadInQueue(d) && (s.DownloadFilter == downloadsAll || downloadState(d) == s.DownloadFilter)
}

func (s *State) downloads(c *ui.Context) {
	if s.DownloadHistory {
		s.downloadHistory(c)
		return
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
	ui.Column(c).Grow(1).MinHeight(0).Padding(24, 28).Gap(20).Children(func() {
		ui.Row(c).Gap(24).Children(func() {
			ui.Column(c).Grow(1).Gap(6).Children(func() {
				ui.Text(c, "下载").FontSize(textSize(c, textTitle)).FontWeight(600)
				if finished > 0 {
					ui.Row(c).Wrap().Gap(controlGap).Children(func() {
						quiet(c, "download-history", fmt.Sprintf("下载记录 %d", finished), func() {
							s.DownloadHistory = true
							s.DownloadHistoryScroll = ui.ScrollState{}
						}).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue)
					})
				}
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
		ui.SearchField(c.Key("download-search"), &s.DownloadSearch).Label("搜索下载").Placeholder("搜索番剧或任务").Width(260).MinHeight(inputHeight(c)).OnChange(func() {
			s.DownloadScroll = ui.ScrollState{}
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
			ui.Row(c).Children(func() {
				caption(c, fmt.Sprintf("%d 个下载任务，按番剧分组", len(visible)))
				ui.Spacer(c)
				pauseLabel := "暂停全部下载"
				if active == 0 {
					pauseLabel = "继续全部下载"
				}
				if active > 0 || resumable > 0 {
					quiet(c, "pause-all", pauseLabel, func() { s.pauseAllDownloads(active > 0) }).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
				}
			})
			type filterOption struct {
				filter DownloadFilter
				label  string
			}
			options := []filterOption{{downloadsAll, "全部"}, {downloadsDownloading, "下载中"}, {downloadsQueued, "排队中"}, {downloadsSeeding, "分享中"}, {downloadsPaused, "已暂停"}, {downloadsFailed, "失败"}, {downloadsStopped, "已停止"}}
			selected := slices.IndexFunc(options, func(option filterOption) bool {
				return option.filter == s.DownloadFilter
			})
			filters := ui.SegmentedBase(c.Key("download-filter"), &selected, len(options))
			filters.Track.Label("下载状态").Wrap().Gap(controlGap).Children(func() {
				for index, option := range options {
					filter, label := option.filter, option.label
					count := stateCounts[filter]
					if filter == downloadsAll {
						count = queued
					}
					filterSegment(c, filters.Segment(index), label, fmt.Sprintf("%s %d", label, count), selected == index)
				}
			}).OnChange(func() {
				s.DownloadFilter = options[selected].filter
				s.DownloadScroll = ui.ScrollState{}
			})
		}
		if len(s.Downloads) == 0 {
			s.empty(c, "还没有下载任务", "在番剧详情选择要下载的资源，任务会出现在这里。", "浏览番剧", func() { s.enterPage("browse") })
			return
		}
		if queued == 0 {
			s.empty(c, "当前没有进行中的任务", "下载和分享都已结束。已结束的任务可在上方「下载记录」中管理文件。", "我的追番", func() { s.enterPage("following") })
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
		ui.Scroll(c.Key("download-list")).TrackScroll(&s.DownloadScroll).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				for _, g := range groups {
					group := g
					ui.Column(c.Key("download-group-" + group.animeID)).Gap(8).Children(func() {
						open := !s.CollapsedDownloads[group.animeID]
						toggle := "收起"
						if !open {
							toggle = "展开"
						}
						var disclosure ui.CollapsibleParts
						actions := func() {
							if folder := s.groupFolder(group.animeID); folder != "" {
								quiet(c, "open-group-folder-"+group.animeID, "打开番剧文件夹", func() {
									if s.OpenFolder != nil {
										s.OpenFolder(folder)
									}
								}).FontSize(textSize(c, textCaption)).Disabled(s.OpenFolder == nil)
							}
							quiet(c, "group-detail-"+group.animeID, "查看详情", func() { s.openFollowedResources(group.animeID, "downloads") }).FontSize(textSize(c, textCaption))
							disclosure = ui.CollapsibleBase(c.Key("toggle-group-"+group.animeID), &open)
							button := disclosure.Trigger.Label(toggle).Padding(controlPaddingY, controlPaddingX).Radius(controlRadius).Cursor(ui.CursorPointer)
							if button.Hovered() {
								button.Background(colors(c).chrome)
							}
							tactile(button).Children(func() {
								ui.Text(c, toggle).FontSize(textSize(c, textCaption))
							}).OnChange(func() {
								if s.CollapsedDownloads == nil {
									s.CollapsedDownloads = make(map[string]bool)
								}
								s.CollapsedDownloads[group.animeID] = !open
							})
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
						if !open {
							return
						}
						renderTasks := func() {
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
											col, wash := colors(c).blue, colors(c).blueWash
											switch {
											case d.Failed:
												status, col, wash = "失败", colors(c).amber, colors(c).amberWash
											case d.Stopped && d.Progress < 1:
												status, col, wash = "已停止", colors(c).muted, colors(c).canvas
											case d.Paused:
												status, col, wash = "已暂停", colors(c).muted, colors(c).canvas
											case d.Queued:
												status, col, wash = "排队中", colors(c).muted, colors(c).canvas
											case d.Progress >= 1:
												status, col, wash = "已完成，分享中", colors(c).green, colors(c).greenWash
											}
											badge(c, status, col, wash)
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
						}
						disclosure.Panel(renderTasks).Gap(8)
					})
				}
			})
		})
		caption(c, "全局上传上限 "+s.Upload+"；完成后"+s.seedStopRule()+"。")
	})
}

func (s *State) downloadHistory(c *ui.Context) {
	var hidden []int
	for i, d := range s.Downloads {
		if !downloadInQueue(d) {
			hidden = append(hidden, i)
		}
	}
	ui.Column(c).Grow(1).MinHeight(0).Padding(24, 28).Gap(20).Children(func() {
		ui.Row(c).Gap(20).Children(func() {
			ui.Column(c).Grow(1).Gap(6).Children(func() {
				ui.Text(c, "下载记录").FontSize(textSize(c, textTitle)).FontWeight(600)
				caption(c, "分享已结束的任务，仍可查看文件、播放或删除。")
			})
			quiet(c, "download-history-back", "返回当前下载", func() {
				s.DownloadHistory = false
			}).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue)
		})
		if len(hidden) == 0 {
			s.empty(c, "还没有下载记录", "分享结束的任务会出现在这里，不依赖是否仍在追番。", "返回当前下载", func() {
				s.DownloadHistory = false
			})
			return
		}
		ui.Scroll(c.Key("download-history-list")).TrackScroll(&s.DownloadHistoryScroll).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				for _, i := range hidden {
					idx := i
					d := s.Downloads[idx]
					title := s.anime(d.AnimeID).Title
					if title == "" {
						title = d.Title
					}
					ui.Column(c.Key("download-history-"+d.ID)).Padding(16, 18).Gap(9).Background(colors(c).paper).Radius(8).Border(1, colors(c).line).Children(func() {
						ui.Row(c).Wrap().Gap(12).Children(func() {
							ui.Text(c, title).FontSize(textSize(c, textSection)).FontWeight(700)
							if d.Episode != "" {
								badge(c, episodeLabel(d.Episode), colors(c).muted, colors(c).canvas)
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
							}).Label("下载记录操作 " + d.ID).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
						})
						ui.Text(c, d.Title).FontSize(textSize(c, textBody)).TextColor(colors(c).muted)
						caption(c, fmt.Sprintf("已完成 %s，分享已结束", byteSizeText(sizeBytes(d.Bytes, d.MB))))
						caption(c, "保存位置："+d.Folder)
					})
				}
			})
		})
	})
}

func (s *State) settings(c *ui.Context) {
	ui.Scroll(c.Key("settings-scroll")).Grow(1).MinHeight(0).Padding(24, 28).Children(func() {
		ui.Column(c).MaxWidth(800).Gap(22).Children(func() {
			ui.Text(c, "设置").FontSize(textSize(c, textTitle)).FontWeight(600)
			ui.Column(c).Gap(12).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
				heading(c, "外观与动效")
				s.appearanceSettings(c)
				ui.Checkbox(c.Key("reduce-effects"), &s.ReduceEffects, "减少动态效果").OnChange(s.save).Disabled(s.Busy)
				caption(c, "减少界面动画和弹层过渡；也会遵循系统设置。")
			})
			ui.Column(c).Gap(14).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
				heading(c, "下载")
				ui.Row(c).Wrap().AlignItems(ui.Center).Gap(12).Children(func() {
					ui.Column(c).Grow(1).MinWidth(textSize(c, textBody) * 12).Gap(5).Children(func() {
						ui.Text(c, "同时下载任务数").FontSize(textSize(c, textBody))
						caption(c, "超出的任务会排队，有空位时自动开始。分享中的任务不占名额。")
					})
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.TextInput(c.Key("download-limit"), &s.downloadLimitDraft).Label("填写同时下载任务数").
							Width(textSize(c, textBody) * 4).MinHeight(inputHeight(c)).
							OnChange(func() { s.downloadLimitNotice = "" }).OnSubmit(s.saveDownloadLimit).Disabled(s.Busy)
						caption(c, "个")
						secondary(c, "save-download-limit", "保存", s.saveDownloadLimit).Label("保存同时下载任务数").
							Disabled(s.Busy || s.downloadLimitDraft == downloadLimitText(s.DownloadLimit))
					})
				})
				if s.downloadLimitNotice != "" {
					if s.downloadLimitError {
						ui.Text(c, s.downloadLimitNotice).FontSize(textSize(c, textCaption)).TextColor(colors(c).errorInk)
					} else {
						caption(c, s.downloadLimitNotice)
					}
				}
				ui.Text(c, "下载位置").FontSize(textSize(c, textBody))
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
				ui.Divider(c).Background(colors(c).line)
				s.trackerSettings(c)
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
				s.transferSettings(c)
			})
			ui.Column(c).Gap(14).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
				heading(c, "网络与代理")
				s.proxySettings(c)
			})
			s.windowSettings(c)
			s.softwareUpdateSettings(c)
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
								quiet(c, "play-file-"+name, "播放此文件", func() { s.playFile(f.Path) }).Label("播放此文件 " + f.Path).FontSize(textSize(c, textCaption)).TextColor(colors(c).blue)
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
				ui.Text(c, "怎么追番").FontSize(textSize(c, textTitle)).FontWeight(600).Grow(1).MinWidth(0)
				iconButton(c, "close-help", "关闭指南", "close", func() { s.Help = false })
			})
			for i, step := range []struct{ title, body string }{{"找到想看的番剧", "按季度和星期浏览，点海报打开番剧。"}, {"选择观看版本", "按画质、字幕和文件大小挑选，也可以按原标题追番。"}, {"开启追番", "以后发布的资源会自动下载。在「我的追番」点海报，从剧集库播放下载完成的剧集。"}} {
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
				ui.Column(c).Gap(6).Children(func() {
					ui.Text(c, word).FontSize(textSize(c, textBody)).FontWeight(600)
					options := []struct {
						label     string
						color, bg ui.Color
					}{
						wordUnrestricted: {"不限", colors(c).ink, colors(c).canvas},
						wordWant:         {"＋ 只要", colors(c).blue, colors(c).blueWash},
						wordAvoid:        {"− 不要", colors(c).muted, colors(c).sunken},
					}
					selected := int(s.wordChoice(word))
					control := ui.SegmentedBase(c.Key(fmt.Sprintf("suggested-%d", i)), &selected, len(options))
					control.Track.Label(word).Wrap().Gap(4).Children(func() {
						for index, option := range options {
							segment := control.Segment(index).Grow(1).Border(1, colors(c).line).FontSize(textSize(c, textCaption))
							filterSegment(c, segment, option.label+" "+word, option.label, index == selected)
							if index == selected {
								segment.Background(option.bg).TextColor(option.color).BorderColor(option.color)
							}
						}
					}).OnChange(func() { s.setWord(word, wordChoice(selected)) })
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
