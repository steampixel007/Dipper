package main

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/downloader"
	"github.com/steampixel007/Dipper/internal/source"
)

func TestUIBackfillStatusPillDoesNotStretchFullWidth(t *testing.T) {
	s := uiFixture(t, "detail")
	s.Editing, s.EditID = true, s.Subscriptions[0].ID
	s.Rule = cloneRule(s.Subscriptions[0].Rule)
	s.beginBackfill()

	tt := ui.NewTester(s.view, 1320, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	pill := requireLayoutBound(t, tt, "下载已有资源", 1320, 860)
	if pill.W > 260 {
		t.Fatalf("backfill status pill stretched across the header: %+v", pill)
	}
}

func TestUISidebarSelectsReturnDestinationOnDetailPage(t *testing.T) {
	cases := []struct {
		name          string
		detailReturn  string
		viewResources bool
		wantSelected  string
	}{
		{"browse", "", false, "浏览番剧"},
		{"browse_detail", browseDetailReturn, true, "浏览番剧"},
		{"following", "following", true, "我的追番"},
		{"downloads", "downloads", true, "下载"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := uiFixture(t, "detail")
			s.DetailReturn = tc.detailReturn
			s.ViewResources = tc.viewResources
			tt := ui.NewTester(s.view, 1040, 720)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			item, ok := tt.AccessByRole(ui.RoleTreeItem, tc.wantSelected)
			if !ok || !item.Selected {
				t.Fatalf("sidebar did not select %q for detailReturn=%q viewResources=%v: %+v", tc.wantSelected, tc.detailReturn, tc.viewResources, item)
			}
		})
	}
}

func TestUIDownloadGroupTitleClickCollapsesAndExpands(t *testing.T) {
	s := uiFixture(t, "downloads")
	tt := ui.NewTester(s.view, 1320, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})

	uiRequireText(t, tt, "分享率 0.36")
	uiClick(t, tt, "收起 星海旅记")
	if tt.HasText("分享率 0.36") || !s.CollapsedDownloads["render-1"] {
		t.Fatalf("clicking group title did not collapse group: collapsed=%v", s.CollapsedDownloads)
	}
	uiClick(t, tt, "展开 星海旅记")
	uiRequireText(t, tt, "分享率 0.36")
	if s.CollapsedDownloads["render-1"] {
		t.Fatal("clicking collapsed group title did not expand group")
	}
}

func TestUIBrowseSplitToolbarStaysSingleRow(t *testing.T) {
	s := uiFixture(t, "browse")
	s.cacheDetail(s.Catalog[0].ID, "", source.Detail{
		Anime: source.Anime{ID: s.Catalog[0].ID, Title: s.Catalog[0].Title, Summary: s.Catalog[0].Summary},
		Groups: []source.Group{
			{ID: s.GroupIDs[0], Name: s.Groups[0], Count: 3},
			{ID: s.GroupIDs[1], Name: s.Groups[1], Count: 2},
		},
	})
	s.openBrowseDetail(s.Catalog[0])

	tt := ui.NewTester(s.view, 1320, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	search := requireLayoutBound(t, tt, "搜索番剧", 1320, 860)
	season := requireLayoutBound(t, tt, "春季", 1320, 860)
	if delta := (search.Y + search.H/2) - (season.Y + season.H/2); delta > 8 || delta < -8 {
		t.Fatalf("opening browse detail wrapped the search bar: search=%+v season=%+v", search, season)
	}
}

func TestUIBrowseDetailGroupChipOpensSelectedGroup(t *testing.T) {
	s := uiFixture(t, "browse")
	s.Subscriptions = nil
	s.cacheDetail(s.Catalog[0].ID, "", source.Detail{
		Anime: source.Anime{ID: s.Catalog[0].ID, Title: s.Catalog[0].Title, Summary: s.Catalog[0].Summary},
		Groups: []source.Group{
			{ID: s.GroupIDs[0], Name: s.Groups[0], Count: 1, Preview: []source.Resource{{ID: "r1", Title: "[甲] 星海旅记 - 01 [1080p].mkv"}}},
			{ID: s.GroupIDs[1], Name: s.Groups[1], Count: 1, Preview: []source.Resource{{ID: "r2", Title: "[乙] 星海旅记 - 01 [1080p].mkv"}}},
		},
	})
	s.openBrowseDetail(s.Catalog[0])

	tt := ui.NewTester(s.view, 1320, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	uiClick(t, tt, "选择字幕组 "+s.Groups[1])
	if s.Page != "detail" || s.Group != 1 || s.GroupIDs[s.Group] != s.GroupIDs[1] {
		t.Fatalf("clicking group chip did not open selected group: page=%q group=%d ids=%v", s.Page, s.Group, s.GroupIDs)
	}
}

func TestUIDownloadsDeduplicatesSharedFolderAndPolicyLinksToSettings(t *testing.T) {
	s := uiFixture(t, "downloads")
	tt := ui.NewTester(s.view, 1320, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})

	folderLabel := "保存位置：" + s.Downloads[0].Folder
	occurrences := 0
	for _, text := range tt.Texts() {
		if text == folderLabel {
			occurrences++
		}
	}
	if occurrences != 1 {
		t.Fatalf("shared folder appeared %d times, want 1 in group summary", occurrences)
	}

	policy := "全局上传上限 " + s.Upload + "；完成后" + s.seedStopRule() + "。"
	uiClick(t, tt, policy)
	if s.Page != "settings" {
		t.Fatalf("clicking policy footer opened %q, want settings", s.Page)
	}
}

func TestUIFilesDialogCopyShowsFeedbackAndShortcutsNavigate(t *testing.T) {
	s := uiFixture(t, "downloads")
	var copied string
	s.CopyText = func(v string) { copied = v }
	s.FilesOpen = true
	s.FilesID = s.Downloads[2].ID
	s.Files = []downloader.DownloadedFile{{Path: `C:\Downloads\Dipper\星海旅记\ep02.mkv`, Size: 236_000_000}}

	tt := ui.NewTester(s.view, 1320, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	uiClick(t, tt, "复制路径 "+s.Files[0].Path)
	if copied != s.Files[0].Path {
		t.Fatalf("copied path = %q, want %q", copied, s.Files[0].Path)
	}
	uiRequireText(t, tt, "已复制")
	tt.Advance(1600 * time.Millisecond)
	if tt.HasText("已复制") {
		t.Fatal("copy confirmation did not clear after timeout")
	}

	// Shortcuts: Ctrl+F focuses active search field, Alt+Left returns from library.
	s.FilesOpen = false
	s.Page = "browse"
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyF)
	tt.Frame()
	if !tt.Focused("搜索番剧") {
		t.Fatal("Ctrl+F did not focus browse search field")
	}

	s.openLibrary(s.Subscriptions[0].ID)
	tt.Frame()
	tt.Key(ui.Alt, ui.KeyLeft)
	tt.Frame()
	if s.Page != "following" {
		t.Fatalf("Alt+Left from library went to %q, want following", s.Page)
	}
}

func TestUITopbarOmitsDuplicateHelpAndTitleScalesAboveSection(t *testing.T) {
	if textTitle <= textSection {
		t.Fatalf("textTitle (%v) should scale above textSection (%v)", textTitle, textSection)
	}
	s := uiFixture(t, "browse")
	tt := ui.NewTester(s.view, 1320, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if _, found := tt.Find("打开追番指南"); found {
		t.Fatal("topbar still renders duplicate help button")
	}
	if _, found := tt.Find("追番小指南"); !found {
		t.Fatal("sidebar help entry is missing")
	}
	// Verify current-season hiding when already on current season with no filters.
	s.setCurrentSeason()
	s.Day = -1
	s.Search = ""
	tt.Frame()
	if tt.HasText("回到当季") {
		t.Fatalf("current-season button still rendered while already viewing current season: %s", strings.Join(tt.Texts(), ", "))
	}
}

func TestUISidebarFocusedSelectionKeepsHighTextContrast(t *testing.T) {
	for _, appearance := range []string{appearanceLight, appearanceDark} {
		t.Run(appearance, func(t *testing.T) {
			s := uiFixture(t, "browse")
			s.Appearance = appearance
			tt := ui.NewTester(s.view, 1040, 720)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})

			// Clicking a sidebar item gives the sidebar focus (chosen && focused).
			uiClick(t, tt, "下载")
			if s.Page != "downloads" {
				t.Fatalf("clicking downloads opened %q", s.Page)
			}
			item, ok := tt.AccessByRole(ui.RoleTreeItem, "下载")
			if !ok || !item.Selected {
				t.Fatalf("downloads sidebar item not selected: %+v", item)
			}
			pal := lightPalette
			if appearance == appearanceDark {
				pal = darkPalette
			}
			frame := tt.Image()
			bgSample := color.NRGBAModel.Convert(frame.At(int(item.Bounds.X+item.Bounds.W*0.75), int(item.Bounds.Y+item.Bounds.H/2))).(color.NRGBA)
			wantBg := color.NRGBA{R: pal.accentWash.R, G: pal.accentWash.G, B: pal.accentWash.B, A: 255}
			if bgSample != wantBg {
				t.Fatalf("%s focused sidebar item fill=%v, want accentWash %v", appearance, bgSample, wantBg)
			}
			labelRect := ui.Rect{X: item.Bounds.X + 8, Y: item.Bounds.Y + 4, W: item.Bounds.W * 0.65, H: item.Bounds.H - 8}
			if inkPixels := countNonBackgroundPixels(frame, labelRect, pal.accentWash); inkPixels < 40 {
				t.Fatalf("%s focused sidebar item label/icon has too few readable pixels (%d) on fill %v", appearance, inkPixels, bgSample)
			}
		})
	}
}

func TestUIBrowseDetailSurfaceStaysUniformAcrossThemes(t *testing.T) {
	if lightPalette.dialogSurface != lightPalette.canvas {
		t.Fatalf("light dialogSurface=%v differs from canvas=%v", lightPalette.dialogSurface, lightPalette.canvas)
	}
	for _, appearance := range []string{appearanceLight, appearanceDark} {
		t.Run(appearance, func(t *testing.T) {
			s := uiFixture(t, "browse")
			s.Appearance = appearance
			s.openBrowseDetail(s.Catalog[0])
			tt := ui.NewTester(s.view, 1040, 720)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})

			closeBtn := requireLayoutBound(t, tt, "关闭番剧详情", 1040, 720)
			pal := lightPalette
			if appearance == appearanceDark {
				pal = darkPalette
			}
			frame := tt.Image()
			// Sample the dialog header area beside the close button: must match
			// uniform pal.dialogSurface instead of a blurred cover tint band or stark white.
			sample := color.NRGBAModel.Convert(frame.At(int(closeBtn.X-120), int(closeBtn.Y+closeBtn.H/2))).(color.NRGBA)
			want := color.NRGBA{R: pal.dialogSurface.R, G: pal.dialogSurface.G, B: pal.dialogSurface.B, A: 255}
			if sample != want {
				t.Fatalf("%s browse detail top surface=%v, want uniform pal.dialogSurface %v", appearance, sample, want)
			}
		})
	}
}

func TestUIViewResourcesBreadcrumbSingleColumnAndLibraryPrimaryCTA(t *testing.T) {
	// 1. In Browse Detail for a followed anime, 进入剧集库 is primary and 查看资源 is secondary.
	s := uiFixture(t, "browse")
	s.cacheDetail(s.Catalog[0].ID, "", source.Detail{
		Anime: source.Anime{ID: s.Catalog[0].ID, Title: s.Catalog[0].Title, Summary: s.Catalog[0].Summary},
		Groups: []source.Group{
			{
				ID: s.GroupIDs[0], Name: s.Groups[0], Count: 1,
				Preview: []source.Resource{{ID: "r1", Title: "[回归测试组甲] 星海旅记 - 01 [1080p][HEVC].mkv", Size: 486_000_000}},
			},
		},
	})
	s.openBrowseDetail(s.Catalog[0])
	tt := ui.NewTester(s.view, 1320, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	libBounds := requireLayoutBound(t, tt, "进入剧集库", 1320, 860)
	resBounds := requireLayoutBound(t, tt, "查看资源", 1320, 860)
	frame := tt.Image()
	libFill := color.NRGBAModel.Convert(frame.At(int(libBounds.X-12), int(libBounds.Y+libBounds.H/2))).(color.NRGBA)
	wantPrimary := color.NRGBA{R: lightPalette.accent.R, G: lightPalette.accent.G, B: lightPalette.accent.B, A: 255}
	if libFill != wantPrimary {
		t.Fatalf("进入剧集库 fill=%v, want primary accent %v", libFill, wantPrimary)
	}
	resFill := color.NRGBAModel.Convert(frame.At(int(resBounds.X-12), int(resBounds.Y+resBounds.H/2))).(color.NRGBA)
	wantSecondary := color.NRGBA{R: lightPalette.surface.R, G: lightPalette.surface.G, B: lightPalette.surface.B, A: 255}
	if resFill != wantSecondary {
		t.Fatalf("查看资源 fill=%v, want secondary surface %v", resFill, wantSecondary)
	}

	// 2. Clicking 查看资源 from Browse Detail uses 浏览番剧 / 查看资源 breadcrumb and
	// collapses the temporary keyword rule editor by default.
	uiClick(t, tt, "查看资源")
	uiRequireText(t, tt, "浏览番剧 / 查看资源", "返回番剧详情", "已追番资源")
	if tt.HasText("标题关键词") {
		t.Fatal("view resources still renders the full keyword rule editor expanded by default")
	}
	uiClick(t, tt, "▸ 临时筛选条件")
	uiRequireText(t, tt, "标题关键词", "重置全部条件")

	// 3. Entering from Downloads shows 下载 / 查看资源.
	s.DetailReturn = "downloads"
	tt.Frame()
	uiRequireText(t, tt, "下载 / 查看资源", "返回下载")
}

func TestUISidebarSelectionPillGlidesAcrossItems(t *testing.T) {
	s := uiFixture(t, "browse")
	s.Appearance = appearanceDark
	s.ReduceEffects = false
	tt := ui.NewTester(s.view, 1040, 720)
	tt.Advance(200 * time.Millisecond)

	following, ok := tt.AccessByRole(ui.RoleTreeItem, "我的追番")
	if !ok {
		t.Fatal("missing following sidebar item")
	}
	downloads, ok := tt.AccessByRole(ui.RoleTreeItem, "下载")
	if !ok {
		t.Fatal("missing downloads sidebar item")
	}

	// Click "下载" without fast-forwarding so we can inspect mid-flight spring frames.
	if err := tt.Click("下载"); err != nil {
		t.Fatalf("click downloads: %v", err)
	}
	// Sample mid-flight frames: the gliding pill must pass through the middle item ("我的追番")
	// before settling on "下载".
	wantWash := color.NRGBA{R: darkPalette.accentWash.R, G: darkPalette.accentWash.G, B: darkPalette.accentWash.B, A: 255}
	sawMidFlight := false
	for step := 0; step < 12; step++ {
		tt.Advance(16 * time.Millisecond)
		frame := tt.Image()
		midPx := color.NRGBAModel.Convert(frame.At(int(following.Bounds.X+following.Bounds.W*0.75), int(following.Bounds.Y+following.Bounds.H/2))).(color.NRGBA)
		if midPx == wantWash {
			sawMidFlight = true
			break
		}
	}
	if !sawMidFlight {
		t.Fatal("sidebar selection pill jumped directly without gliding across intermediate items")
	}
	tt.Advance(400 * time.Millisecond)
	settled := color.NRGBAModel.Convert(tt.Image().At(int(downloads.Bounds.X+downloads.Bounds.W*0.75), int(downloads.Bounds.Y+downloads.Bounds.H/2))).(color.NRGBA)
	if settled != wantWash {
		t.Fatalf("settled downloads pill fill=%v, want accentWash %v", settled, wantWash)
	}
}
