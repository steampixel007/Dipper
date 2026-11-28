package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/source"
)

func TestUIBrowseDetailRoundTrip(t *testing.T) {
	state := uiFixture(t, "browse")
	first, other := state.Catalog[0], &state.Catalog[1]
	other.Summary = strings.Repeat("远行的人们在星空下相遇，开始了新的旅程。", 100)
	tester := ui.NewTester(state.view, 1320, 900)
	firstLabel := "查看 " + first.Title
	full := requireLayoutBound(t, tester, firstLabel, 1320, 900)
	tester.Move(full.X+full.W/2, full.Y+60)
	uiRequireText(t, tester, "查看详情")
	originalGroup := state.Group
	tester.ClickAt(full.X+full.W/2, full.Y+60)
	if state.Page != "browse" || state.browseDetail == nil || state.browseDetail.anime.ID != first.ID || state.Group != originalGroup {
		t.Fatalf("poster click did not open the synopsis in browse without changing groups")
	}
	uiRequireText(t, tester, first.Summary, "查看资源")
	third := requireLayoutBound(t, tester, "查看 "+state.Catalog[2].Title, 1320, 900)
	fourth, _ := tester.Find("查看 " + state.Catalog[3].Title)
	closeButton := requireLayoutBound(t, tester, "关闭番剧详情", 1320, 900)
	if fourth.Y <= third.Y || closeButton.X < third.X+third.W {
		t.Fatalf("expected three posters beside detail: third=%+v fourth=%+v close=%+v", third, fourth, closeButton)
	}

	title := requireLayoutBound(t, tester, other.Title, 1320, 900)
	tester.ClickAt(title.X+title.W/2, title.Y+title.H/2)
	if tester.HasText("查看资源") || state.browseDetail.anime.ID != other.ID {
		t.Fatal("selecting a new poster did not replace the synopsis and action")
	}
	tester.Scroll(1110, 480, 0, 500)
	offset := state.browseDetail.summaryScroll.Y
	if offset <= 0 {
		t.Fatal("long synopsis did not scroll")
	}
	uiClick(t, tester, "选择观看版本")
	if state.Page != "detail" || state.AnimeID != other.ID || state.Group != -1 || state.ViewResources {
		t.Fatal("detail action did not open the existing group selection flow")
	}
	uiClick(t, tester, "返回番剧详情")
	if state.Page != "browse" || state.browseDetail == nil || state.browseDetail.anime.ID != other.ID || state.browseDetail.summaryScroll.Y != offset {
		t.Fatalf("return did not restore the selected synopsis at offset %.0f", offset)
	}
	uiClick(t, tester, "关闭番剧详情")
	if state.browseDetail != nil || requireLayoutBound(t, tester, firstLabel, 1320, 900) != full {
		t.Fatal("closing did not restore the original poster layout")
	}
}

func TestUIBrowseDetailPreservesGridScroll(t *testing.T) {
	state := uiFixture(t, "browse")
	for index := 0; index < 18; index++ {
		entry := state.Catalog[0]
		entry.ID, entry.Title = fmt.Sprintf("scroll-%d", index), fmt.Sprintf("海报 %02d", index)
		state.Catalog = append(state.Catalog, entry)
	}
	tester := ui.NewTester(state.view, 1320, 900)
	tester.Scroll(420, 480, 0, 575)
	offset := state.CatalogScroll.Y
	if offset <= 0 {
		t.Fatal("precondition: catalog did not scroll")
	}
	uiClick(t, tester, "查看 海报 04")
	panel := state.browseDetail
	if panel == nil || panel.catalogScroll.Y <= offset {
		t.Fatal("expanded catalog did not scroll to the selected poster")
	}
	tester.Scroll(455, 480, 0, -230)
	tester.Key(0, ui.KeyEscape)
	if state.browseDetail != nil || state.CatalogScroll.Y != offset {
		t.Fatalf("closed detail left catalog at %.0f, want %.0f", state.CatalogScroll.Y, offset)
	}
}

func TestUIBrowseDetailAtNarrowWidth(t *testing.T) {
	state := uiFixture(t, "browse")
	tester := ui.NewTester(state.view, 1040, 720)
	tester.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
	cardLabel := "查看 " + state.Catalog[0].Title
	uiClick(t, tester, cardLabel)
	if state.browseDetail == nil {
		t.Fatal("narrow poster click did not open details")
	}
	closeButton := requireLayoutBound(t, tester, "关闭番剧详情", 1040, 720)
	action := requireLayoutBound(t, tester, "查看资源", 1040, 720)
	if center := action.X + action.W/2; closeButton.X > 960 || center < 514 || center > 526 {
		t.Fatalf("detail should be centered: close=%+v action=%+v", closeButton, action)
	}
	tester.Key(0, ui.KeyEscape)
	if state.browseDetail != nil {
		t.Fatal("Escape did not dismiss the narrow detail")
	}
}

func TestBrowseSummaryCachesOnlyActiveResponse(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "bangumi-4071.html"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/Home/Bangumi/4071" {
			t.Errorf("unexpected request: %s", request.URL)
			http.NotFound(writer, request)
			return
		}
		writer.Write(body)
	}))
	defer server.Close()
	state := uiFixture(t, "browse")
	state.Ctx, state.Source = context.Background(), source.New(server.URL)
	updates := make(chan func(), 1)
	state.Update = func(update func()) { updates <- update }
	result := func() func() {
		t.Helper()
		select {
		case update := <-updates:
			return update
		case <-time.After(5 * time.Second):
			t.Fatal("synopsis request did not complete")
			return nil
		}
	}
	anime := Anime{ID: "4071", Title: "异步测试番剧"}
	state.openBrowseDetail(anime)
	outdated := result()
	if !state.browseDetail.loading {
		t.Fatal("the first synopsis should still be loading")
	}
	state.openBrowseDetail(state.Catalog[0])
	outdated()
	if state.browseDetail.anime != state.Catalog[0] || state.browseDetail.loading {
		t.Fatal("old source response overwrote the replacement synopsis")
	}
	if state.anime(anime.ID).Summary != "" {
		t.Fatal("switching posters cached an obsolete synopsis")
	}
	state.openBrowseDetail(anime)
	outdated = result()
	state.closeBrowseDetail()
	outdated()
	if state.browseDetail != nil {
		t.Fatal("a late source response reopened closed details")
	}
	if state.anime(anime.ID).Summary != "" {
		t.Fatal("closing the panel cached an obsolete synopsis")
	}

	state.openBrowseDetail(anime)
	updated := anime
	updated.Title, updated.Day = "更新后的番剧标题", 3
	state.KnownAnime[anime.ID] = updated
	result()()
	summary := state.browseDetail.anime.Summary
	if state.browseDetail.loading || state.browseDetail.failed || summary == "" {
		t.Fatal("active request did not load the synopsis from the captured detail page")
	}
	state.openBrowseDetail(state.Catalog[0])
	state.openBrowseDetail(anime)
	if state.browseDetail.loading || state.browseDetail.anime.Summary != summary {
		state.closeBrowseDetail()
		t.Fatal("switching back did not reuse the loaded synopsis")
	}
	state.closeBrowseDetail()
	state.openBrowseDetail(anime)
	if state.browseDetail.loading || state.browseDetail.anime.Summary != summary {
		state.closeBrowseDetail()
		t.Fatal("reopening the panel did not reuse the loaded synopsis")
	}
	updated.Summary = summary
	updated.BroadcastStart = "10/3/2026"
	updated.OfficialURL = "https://vertexforce.jp/"
	updated.BangumiURL = "https://bgm.tv/subject/636512"
	if got := state.anime(anime.ID); got != updated {
		t.Fatalf("caching a synopsis changed newer anime metadata: got %+v, want %+v", got, updated)
	}
}

func TestUIBrowseDetailShowsGroupsMetadataAndReusesCachedDetail(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "bangumi-4071.html"))
	if err != nil {
		t.Fatal(err)
	}
	historyBody, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "probe-b4063-g370-take65.html"))
	if err != nil {
		t.Fatal(err)
	}
	var detailCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/Home/Bangumi/4071":
			detailCalls.Add(1)
			writer.Write(body)
		case request.URL.Path == "/Home/ExpandEpisodeTable":
			writer.Write(historyBody)
		case strings.HasPrefix(request.URL.Path, "/images/"):
			http.NotFound(writer, request)
		default:
			t.Errorf("unexpected request: %s", request.URL)
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	state := uiFixture(t, "browse")
	state.Subscriptions = nil
	state.Ctx, state.Source = context.Background(), source.New(server.URL)
	var openedURLs []string
	state.OpenURL = func(raw string) { openedURLs = append(openedURLs, raw) }
	updates := make(chan func(), 16)
	state.Update = func(update func()) { updates <- update }
	drainOne := func() {
		t.Helper()
		select {
		case update := <-updates:
			update()
		case <-time.After(5 * time.Second):
			t.Fatal("async update did not arrive")
		}
	}

	anime := Anime{ID: "4071", Title: "至高之力", Day: 0}
	state.Catalog = append([]Anime{anime}, state.Catalog...)
	state.KnownAnime[anime.ID] = anime
	state.openBrowseDetail(anime)
	for state.browseDetail.loading {
		drainOne()
	}
	if got := detailCalls.Load(); got != 1 {
		t.Fatalf("browse detail calls = %d, want 1", got)
	}

	tester := ui.NewTester(state.view, 1320, 900)
	uiRequireText(t, tester, "2026 年 10 月 3 日开播", "字幕组（5）", "LoliHouse", "ANi", "Bangumi 番组计划", "官方网站")
	tester.Scroll(1110, 480, 0, 500)
	uiClick(t, tester, "Bangumi 番组计划")
	uiClick(t, tester, "官方网站")
	state.openExternalURL("javascript:alert(1)")
	if len(openedURLs) != 2 || openedURLs[0] != "https://bgm.tv/subject/636512" || openedURLs[1] != "https://vertexforce.jp/" {
		t.Fatalf("opened external URLs = %v, want bangumi and official URLs only", openedURLs)
	}

	uiClick(t, tester, "选择观看版本")
	if state.Page != "detail" || state.DetailLoading || len(state.Groups) != 5 || state.Groups[0] != "LoliHouse" {
		t.Fatalf("detail page did not reuse cached groups immediately: page=%q loading=%v groups=%v", state.Page, state.DetailLoading, state.Groups)
	}
	if got := detailCalls.Load(); got != 1 {
		t.Fatalf("opening version selection re-requested Detail: detailCalls = %d, want 1", got)
	}
	// Explicit refresh bypasses the cache.
	state.refreshDetail()
	for state.DetailLoading {
		drainOne()
	}
	if got := detailCalls.Load(); got != 2 {
		t.Fatalf("explicit refreshDetail did not bypass cache: detailCalls = %d, want 2", got)
	}
	// Switching source origin invalidates the old origin's cached detail.
	if err := state.Source.Retarget("https://mikanime.tv"); err != nil {
		t.Fatal(err)
	}
	if _, ok := state.cachedDetail(anime.ID); ok {
		t.Fatal("cached detail survived a source origin change")
	}
	state.closeBrowseDetail()
	if state.detailCancel != nil {
		state.detailCancel()
	}
	if state.historyCancel != nil {
		state.historyCancel()
	}
}

func TestUIBrowseDetailFollowedShowsGroupsAndOpensLibrary(t *testing.T) {
	state := uiFixture(t, "browse")
	first := state.Catalog[0]
	tester := ui.NewTester(state.view, 1320, 900)
	uiClick(t, tester, "查看 "+first.Title)
	uiRequireText(t, tester, "已追番", "正在追：回归测试组甲、回归测试组乙", "查看资源", "进入剧集库")

	uiClick(t, tester, "进入剧集库")
	if state.Page != "episodes" || state.LibraryID != state.Subscriptions[0].ID || state.LibraryReturn != browseDetailReturn {
		t.Fatalf("entering library from browse detail failed: page=%q id=%q return=%q", state.Page, state.LibraryID, state.LibraryReturn)
	}
	uiRequireText(t, tester, "浏览番剧 / 剧集库", "返回番剧详情")

	uiClick(t, tester, "返回番剧详情")
	if state.Page != "browse" || state.browseDetail == nil || state.browseDetail.anime.ID != first.ID {
		t.Fatalf("returning from library did not restore browse detail: page=%q detail=%+v", state.Page, state.browseDetail)
	}
}

func TestUIBrowseDetailEmptySummaryReusesCacheAndEmptyGroupsState(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "bangumi-4063.html"))
	if err != nil {
		t.Fatal(err)
	}
	var detailCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/Home/Bangumi/4063":
			detailCalls.Add(1)
			writer.Write(body)
		case strings.HasPrefix(request.URL.Path, "/images/"):
			http.NotFound(writer, request)
		default:
			t.Errorf("unexpected request: %s", request.URL)
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	state := uiFixture(t, "browse")
	state.Ctx, state.Source = context.Background(), source.New(server.URL)
	updates := make(chan func(), 8)
	state.Update = func(update func()) { updates <- update }

	anime := Anime{ID: "4063", Title: "无简介番剧", Day: 1}
	state.openBrowseDetail(anime)
	for state.browseDetail.loading {
		select {
		case update := <-updates:
			update()
		case <-time.After(5 * time.Second):
			t.Fatal("detail request did not complete")
		}
	}
	if detailCalls.Load() != 1 || state.browseDetail.anime.Summary != "" || len(state.browseDetail.groups) == 0 {
		t.Fatalf("unexpected detail load result: calls=%d summary=%q groups=%d", detailCalls.Load(), state.browseDetail.anime.Summary, len(state.browseDetail.groups))
	}
	state.openBrowseDetail(state.Catalog[0])
	state.openBrowseDetail(anime)
	if detailCalls.Load() != 1 || state.browseDetail.loading || len(state.browseDetail.groups) == 0 {
		t.Fatalf("reopening empty-summary anime re-fetched or lost cached groups: calls=%d loading=%v groups=%d", detailCalls.Load(), state.browseDetail.loading, len(state.browseDetail.groups))
	}

	// Unloaded detail must not falsely claim there are no subtitle groups;
	// a loaded detail with zero groups should state it explicitly.
	state.browseDetail.loaded, state.browseDetail.groups = false, nil
	tester := ui.NewTester(state.view, 1320, 900)
	if tester.HasText("暂无字幕组发布资源") {
		t.Fatal("unloaded browse detail falsely reported zero subtitle groups")
	}
	state.browseDetail.loaded = true
	tester = ui.NewTester(state.view, 1320, 900)
	uiRequireText(t, tester, "暂无字幕组发布资源")
}
