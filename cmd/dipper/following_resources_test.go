package main

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"

	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

// followingSourceServer serves the captured 227 detail page and the captured
// 161 history pages and seeds their covers so tests never touch the real host.
func followingSourceServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	detailBody, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "bangumi-227.html"))
	if err != nil {
		t.Fatal(err)
	}
	probe1000, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "probe-b227-g161-take1000.html"))
	if err != nil {
		t.Fatal(err)
	}
	probe1523, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "probe-b227-g161-take1523.html"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Home/Bangumi/227":
			w.Write(detailBody)
		case "/Home/ExpandEpisodeTable":
			take, err := strconv.Atoi(r.URL.Query().Get("take"))
			if err != nil {
				t.Errorf("invalid take: %s", r.URL.Query().Get("take"))
				http.Error(w, "bad take", http.StatusBadRequest)
				return
			}
			if take == 1000 {
				w.Write(probe1000)
			} else if take > 1000 {
				w.Write(probe1523)
			} else {
				t.Errorf("unexpected take %d", take)
				http.Error(w, "unexpected take", http.StatusInternalServerError)
			}
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	coverURLs := []string{}
	for _, match := range regexp.MustCompile(`(?:background-image:\s*url\(|(?:data-src|src)=)["']?([^"'\s>]+)`).FindAllStringSubmatch(string(detailBody), -1) {
		coverURLs = append(coverURLs, server.URL+html.UnescapeString(match[1]))
	}
	coversMu.Lock()
	for _, coverURL := range coverURLs {
		covers[coverURL] = covers["3887.jpg"]
		loadingCovers[coverURL] = true
	}
	coversMu.Unlock()
	cleanup := func() {
		coversMu.Lock()
		defer coversMu.Unlock()
		for _, coverURL := range coverURLs {
			delete(covers, coverURL)
			delete(loadingCovers, coverURL)
		}
		server.Close()
	}
	return server, cleanup
}

func TestUIFollowingViewResourcesFixtureBackReturnsToFollowing(t *testing.T) {
	state := uiFixture(t, "following")
	sub := state.Subscriptions[0]
	sub.GroupName = "回归测试组甲"
	state.Subscriptions[0] = sub
	tester := ui.NewTester(state.view, 1320, 860)
	uiClick(t, tester, "查看资源 "+sub.GroupName)
	uiRequireText(t, tester, "我的追番 / 查看资源", "查看已追番资源", "返回我的追番", "已保存条件，仅用于筛选与确认，不修改。")
	uiClick(t, tester, "返回我的追番")
	if state.Page != "following" {
		t.Fatalf("back did not return to following: page=%q", state.Page)
	}
}

// Following-card resource entry opens the saved subscription's group and rule
// in a view-only backfill: nothing is auto-selected while history is loading,
// a later history result must not auto-select matches, and the saved rule is
// preserved.
func TestUIFollowingViewResourcesManualSelectionAndReturn(t *testing.T) {
	state := uiFixture(t, "following")
	server, cleanup := followingSourceServer(t)
	defer cleanup()
	state.Ctx, state.Source = context.Background(), source.New(server.URL)
	sub := state.Subscriptions[0]
	sub.AnimeID = "227"
	sub.GroupID = "161"
	sub.GroupName = "字幕组161"
	state.Subscriptions[0] = sub
	savedRule := cloneRule(sub.Rule)
	updates := make(chan func(), 4)
	state.Update = func(update func()) { updates <- update }
	tester := ui.NewTester(state.view, 1320, 860)
	uiClick(t, tester, "查看资源 "+sub.GroupName)
	if state.Page != "detail" || !state.ViewResources || !state.Backfill || state.EditID != sub.ID || state.DetailReturn != "following" {
		t.Fatalf("wrong view state: page=%q view=%v backfill=%v id=%q return=%q", state.Page, state.ViewResources, state.Backfill, state.EditID, state.DetailReturn)
	}
	if !reflect.DeepEqual(state.Rule, savedRule) {
		t.Fatal("view entry changed the saved rule")
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for !state.detailReady() {
		select {
		case update := <-updates:
			update()
		case <-deadline.C:
			t.Fatalf("detail did not become ready; detail=%q history=%q", state.DetailError, state.HistoryError)
		}
		tester.Frame()
	}
	for _, checked := range state.Checked {
		if checked {
			t.Fatal("view mode auto-selected a resource")
		}
	}
	uiRequireText(t, tester, "我的追番 / 查看资源", "查看已追番资源", "补下选中资源", "不会修改已保存的追番条件", "返回我的追番")
	uiClick(t, tester, "返回我的追番")
	if state.Page != "following" {
		t.Fatalf("back did not return to following: page=%q", state.Page)
	}
	if !reflect.DeepEqual(state.Subscriptions[0].Rule, savedRule) {
		t.Fatal("view entry changed the saved rule")
	}
}

func TestUIFollowingViewResourcesAllowsExplicitBackfillSelection(t *testing.T) {
	state := uiFixture(t, "following")
	server, cleanup := followingSourceServer(t)
	defer cleanup()
	state.Ctx, state.Source = context.Background(), source.New(server.URL)
	sub := state.Subscriptions[0]
	sub.AnimeID = "227"
	sub.GroupID = "161"
	sub.GroupName = "字幕组161"
	state.Subscriptions[0] = sub
	updates := make(chan func(), 4)
	state.Update = func(update func()) { updates <- update }
	tester := ui.NewTester(state.view, 1320, 860)
	uiClick(t, tester, "查看资源 "+sub.GroupName)
	if !state.ViewResources || !state.Backfill {
		t.Fatalf("wrong view state: view=%v backfill=%v", state.ViewResources, state.Backfill)
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for !state.detailReady() {
		select {
		case update := <-updates:
			update()
		case <-deadline.C:
			t.Fatalf("detail did not become ready; detail=%q history=%q", state.DetailError, state.HistoryError)
		}
		tester.Frame()
	}
	if len(state.ResourceList) == 0 {
		t.Fatal("view mode has no resources")
	}
	first := state.ResourceList[0]
	uiClick(t, tester, "选择 "+first.ID)
	if !state.Checked[first.ID] {
		t.Fatal("manual selection did not apply")
	}
	uiClick(t, tester, "补下选中资源")
	if !state.Confirm {
		t.Fatal("selected backfill did not open confirmation")
	}
	uiRequireText(t, tester, "确认补下已有资源", "已保存条件，仅用于筛选与确认，不修改。")
}
