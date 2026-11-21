package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	firstLabel, otherLabel := "查看 "+first.Title, "查看 "+other.Title
	full := requireLayoutBound(t, tester, firstLabel, 1320, 900)
	tester.Move(full.X+full.W/2, full.Y+60)
	uiRequireText(t, tester, "查看详情")
	originalGroup := state.Group
	uiClick(t, tester, firstLabel)
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

	uiClick(t, tester, otherLabel)
	if tester.HasText("查看资源") || state.browseDetail.anime.ID != other.ID {
		t.Fatal("selecting a new poster did not replace the synopsis and action")
	}
	tester.Scroll(1110, 480, 0, 500)
	offset := state.browseDetail.summaryScroll.Y
	if offset <= 0 {
		t.Fatal("long synopsis did not scroll")
	}
	uiClick(t, tester, "选择字幕组")
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

func TestBrowseSummaryIgnoresObsoleteResponses(t *testing.T) {
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
	state.openBrowseDetail(anime)
	result()()
	if state.browseDetail.loading || state.browseDetail.failed || state.browseDetail.anime.Summary == "" {
		t.Fatal("active request did not load the synopsis from the captured detail page")
	}
	state.openBrowseDetail(state.Catalog[0])
	state.openBrowseDetail(anime)
	outdated = result()
	state.closeBrowseDetail()
	outdated()
	if state.browseDetail != nil {
		t.Fatal("a late source response reopened closed details")
	}
}
