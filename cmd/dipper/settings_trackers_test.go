package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/source"
	"github.com/steampixel007/Dipper/internal/trackers"
)

func TestUITrackerManagerFindsListsAndAddressesAcrossPages(t *testing.T) {
	s := uiFixture(t, "settings")
	addresses := make([]string, core.MaxTrackersPerList)
	for index := range addresses {
		addresses[index] = fmt.Sprintf("https://tracker-%03d.example/announce", index+1)
	}
	for index := 1; index <= core.MaxTrackerLists; index++ {
		s.TrackerLists = append(s.TrackerLists, core.TrackerList{
			URL: fmt.Sprintf("https://lists.example/list-%02d.txt", index), Trackers: addresses, FetchedAt: time.Now(),
		})
	}
	s.openTrackerManager()
	tt := ui.NewTester(s.view, 1040, 720)
	tt.Advance(300 * time.Millisecond)
	uiRequireText(t, tt, "在线清单", "Dipper 推荐清单", "第 1 / 4 页 · 16 份清单")
	if tt.HasText("list-16.txt") || tt.HasText(addresses[0]) {
		t.Fatal("overview exposed entries beyond the current page")
	}
	uiScrollTo(t, tt, "清单下一页")
	uiClick(t, tt, "清单下一页")
	uiRequireText(t, tt, "第 2 / 4 页 · 16 份清单", "list-05.txt")
	uiClick(t, tt, "搜索清单")
	tt.Type("LIST-16")
	uiRequireText(t, tt, "list-16.txt")
	uiClick(t, tt, "查看 https://lists.example/list-16.txt")
	uiRequireText(t, tt, "第 1 / 25 页 · 200 个地址", addresses[0])
	if tt.HasText(addresses[8]) {
		t.Fatal("addresses beyond the first page were rendered")
	}
	uiScrollTo(t, tt, "地址下一页")
	uiClick(t, tt, "地址下一页")
	uiRequireText(t, tt, "第 2 / 25 页 · 200 个地址", addresses[8])
	uiClick(t, tt, "搜索地址")
	tt.Type("TRACKER-200")
	uiRequireText(t, tt, addresses[199])
	uiClick(t, tt, "删除 "+addresses[199])
	if tt.HasText(addresses[199]) {
		t.Fatal("deleted address remained in the filtered list")
	}
	uiClick(t, tt, "已删除的地址（1）")
	uiClick(t, tt, "添加回来 "+addresses[199])
	uiClick(t, tt, "查看现有地址")
	uiClick(t, tt, "搜索地址")
	tt.Type("TRACKER-200")
	uiRequireText(t, tt, addresses[199])
	uiClick(t, tt, "返回下载来源")
	uiRequireText(t, tt, "list-16.txt")
	uiClick(t, tt, "搜索清单")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("missing-list")
	uiRequireText(t, tt, "没有找到匹配的清单。")
}

func TestUISettingsShowsOnlyXIU2AsDefaultTrackerSource(t *testing.T) {
	const address = "udp://recommended.example:1337/announce"
	s := uiFixture(t, "settings")
	s.trackerAutomatic = trackers.List{URLs: []string{address}}
	tt := ui.NewTester(s.view, 1040, 720)
	uiScrollTo(t, tt, "管理地址")
	uiRequireText(t, tt, "查找下载来源", "管理地址")
	uiClick(t, tt, "管理地址")
	uiRequireText(t, tt, "查找下载来源", "Dipper 推荐清单", "1 个地址 · 自动更新 · 来自 XIU2")
	if tt.HasText("默认地址") {
		t.Fatal("manager still shows a separate default address list")
	}
	uiClick(t, tt, "查看 Dipper 推荐清单")
	uiRequireText(t, tt, address)
	uiClick(t, tt, "取消")
	if s.trackerManagerOpen {
		t.Fatal("cancel left the manager open")
	}
}

type trackerTestEngine struct {
	downloadLimitTestEngine
	applied, disabled, remote []string
}

func (engine *trackerTestEngine) SetTrackerSettings(urls, disabled []string) {
	engine.applied = slices.Clone(urls)
	engine.disabled = slices.Clone(disabled)
}

func (engine *trackerTestEngine) SetRemoteTrackers(urls []string) {
	engine.remote = slices.Clone(urls)
}

func TestTrackerManagerPreviewsSavesAndKeepsLastGoodList(t *testing.T) {
	const manual = "udp://manual.example:1337/announce"
	const subscribed = "https://tracker.example/announce"
	fail := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case <-fail:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			_, _ = w.Write([]byte(manual + "\n" + subscribed + "\n" + manual))
		}
	}))
	defer server.Close()
	s := uiFixture(t, "settings")
	database := filepath.Join(s.Dir, "dipper.db")
	kernel, err := core.Open(database, core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })
	s.Core, s.Ctx = kernel, t.Context()
	engine := &trackerTestEngine{}
	s.Engine = engine
	updates := make(chan func(), 1)
	s.Update = func(update func()) { updates <- update }
	waitForRefresh := func() {
		t.Helper()
		select {
		case update := <-updates:
			update()
		case <-time.After(3 * time.Second):
			t.Fatal("tracker list read never finished")
		}
	}

	tt := ui.NewTester(s.view, 1040, 720)
	uiScrollTo(t, tt, "管理地址")
	uiClick(t, tt, "管理地址")
	uiScrollTo(t, tt, "添加地址")
	uiClick(t, tt, "添加地址")
	draft := s.trackerManager
	draft.input = manual + "\n" + manual
	tt.Frame()
	uiScrollTo(t, tt, "添加到列表")
	uiClick(t, tt, "添加到列表")
	uiScrollTo(t, tt, "订阅清单")
	uiClick(t, tt, "订阅清单")
	draft.input = server.URL
	tt.Frame()
	uiScrollTo(t, tt, "预览地址")
	uiClick(t, tt, "预览地址")
	waitForRefresh()
	tt.Frame()
	uiRequireText(t, tt, "读到 2 个有效地址，其中 1 个已配置。")
	uiScrollTo(t, tt, "添加订阅")
	uiClick(t, tt, "添加订阅")
	if len(engine.applied) != 0 || len(s.TrackerLists) != 0 {
		t.Fatal("confirming the preview applied unsaved settings")
	}
	uiClick(t, tt, "保存修改")
	settings, err := kernel.GetSettings()
	want := []string{manual, subscribed}
	if err != nil || settings.ExtraTrackers != manual || len(settings.TrackerLists) != 1 ||
		settings.TrackerLists[0].URL != server.URL || settings.TrackerLists[0].FetchedAt.IsZero() ||
		!slices.Equal(settings.ActiveExtraTrackers(), want) || !slices.Equal(engine.applied, want) {
		t.Fatalf("tracker save did not persist and apply: %+v, applied=%v, error=%v", settings, engine.applied, err)
	}

	close(fail)
	s.openTrackerManager()
	draft = s.trackerManager
	s.readTrackerList(draft, 0)
	waitForRefresh()
	if !draft.failed || !slices.Equal(draft.lists[0].Trackers, want) {
		t.Fatalf("failed refresh replaced cached list: %+v", draft)
	}
	draft.selectList(server.URL)
	tt.Frame()
	uiClick(t, tt, "删除订阅")
	uiClick(t, tt, "取消")
	if len(s.TrackerLists) != 1 || !slices.Equal(engine.applied, want) {
		t.Fatal("cancel applied edits")
	}
	s.openTrackerManager()
	s.trackerManager.selectList(server.URL)
	tt.Frame()
	uiClick(t, tt, "删除订阅")
	uiClick(t, tt, "保存修改")
	settings, err = kernel.GetSettings()
	if err != nil || len(settings.TrackerLists) != 0 || !slices.Equal(engine.applied, []string{manual}) {
		t.Fatalf("deleted subscription was not removed: %+v, applied=%v, error=%v", settings, engine.applied, err)
	}
}

func TestTrackerManagerDeletesRecommendedListDuringRefresh(t *testing.T) {
	const cached = "udp://cached.example:1337/announce"
	const fetched = "https://updated.example/announce"
	s := uiFixture(t, "settings")
	database := filepath.Join(s.Dir, "dipper.db")
	kernel, err := core.Open(database, core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })
	engine := &trackerTestEngine{}
	s.Core, s.Engine, s.Source, s.Ctx = kernel, engine, source.New(""), t.Context()
	s.trackerAutomatic = trackers.List{URLs: []string{cached}}
	s.applyAutomaticTrackerList()
	if !slices.Equal(engine.remote, []string{cached}) {
		t.Fatalf("cached recommendation should be used by default: %v", engine.remote)
	}
	release := make(chan struct{})
	updates := make(chan func(), 1)
	s.Update = func(update func()) { updates <- update }
	s.trackerFetcher = func(ctx context.Context, _ string) ([]byte, error) {
		select {
		case <-release:
			return []byte(fetched + "\n"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	s.refreshAutomaticTrackerList()

	s.openTrackerManager()
	tt := ui.NewTester(s.view, 1040, 720)
	tt.Advance(300 * time.Millisecond)
	uiClick(t, tt, "查看 Dipper 推荐清单")
	uiRequireText(t, tt, cached)
	uiClick(t, tt, "删除推荐清单")
	if !slices.Equal(engine.remote, []string{cached}) {
		t.Fatal("deleting the list applied unsaved settings")
	}
	uiClick(t, tt, "保存修改")
	close(release)
	select {
	case update := <-updates:
		update()
	case <-time.After(3 * time.Second):
		t.Fatal("automatic list refresh did not finish")
	}
	if len(engine.remote) != 0 {
		t.Fatalf("the deleted recommendation still supplies addresses: %v", engine.remote)
	}

	reopened, err := core.Open(database, core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	s.Core = reopened
	settings, err := reopened.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	cache := trackers.Load(filepath.Join(s.Dir, "trackers.json"))
	if !slices.Equal(cache.URLs, []string{fetched}) || len(activeAutomaticTrackers(cache, settings.AutomaticTrackersDisabled)) != 0 {
		t.Fatalf("startup would apply the deleted list: %+v, removed=%v", cache, settings.AutomaticTrackersDisabled)
	}
	s.trackerAutomatic = cache
	if err := s.loadSaved(); err != nil {
		t.Fatal(err)
	}
	s.openTrackerManager()
	tt.Frame()
	uiClick(t, tt, "查看 Dipper 推荐清单")
	uiClick(t, tt, "添加回来")
	uiClick(t, tt, "保存修改")
	settings, err = reopened.GetSettings()
	if err != nil || settings.AutomaticTrackersDisabled || !slices.Equal(engine.remote, []string{fetched}) {
		t.Fatalf("restored recommendation did not reach the settings and engine: %+v, %v, %v", settings, engine.remote, err)
	}
}

func TestTrackerManagerPersistsRecommendedAddressDeletionAndRestore(t *testing.T) {
	const address = "udp://recommended.example:1337/announce"
	s := uiFixture(t, "settings")
	s.trackerAutomatic = trackers.List{URLs: []string{address}}
	database := filepath.Join(s.Dir, "dipper.db")
	kernel, err := core.Open(database, core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })
	engine := &trackerTestEngine{}
	s.Core, s.Engine, s.Source = kernel, engine, source.New("")
	tt := ui.NewTester(s.view, 1040, 720)
	uiScrollTo(t, tt, "管理地址")
	uiClick(t, tt, "管理地址")
	uiClick(t, tt, "查看 Dipper 推荐清单")
	uiClick(t, tt, "删除 "+address)
	if !slices.Equal(s.trackerManager.disabled, []string{address}) {
		t.Fatal("deleting the recommended address did not change the draft")
	}
	if tt.HasText(address) {
		t.Fatal("deleted address remained in the active list")
	}
	uiClick(t, tt, "取消")
	if len(s.DisabledTrackers) != 0 || len(engine.disabled) != 0 {
		t.Fatal("cancel applied the address change")
	}
	uiClick(t, tt, "管理地址")
	uiClick(t, tt, "查看 Dipper 推荐清单")
	uiClick(t, tt, "删除 "+address)
	uiClick(t, tt, "保存修改")
	settings, err := kernel.GetSettings()
	if err != nil || !slices.Equal(settings.DisabledTrackers, []string{address}) || !slices.Equal(engine.disabled, []string{address}) {
		t.Fatalf("saved exclusion did not reach settings and engine: %v, %v, %v", settings.DisabledTrackers, engine.disabled, err)
	}
	reopened, err := core.Open(database, core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	s.Core = reopened
	if err := s.loadSaved(); err != nil {
		t.Fatal(err)
	}
	uiClick(t, tt, "管理地址")
	uiClick(t, tt, "查看 Dipper 推荐清单")
	uiClick(t, tt, "已删除的地址（1）")
	uiClick(t, tt, "添加回来 "+address)
	uiClick(t, tt, "保存修改")
	settings, err = reopened.GetSettings()
	if err != nil || len(settings.DisabledTrackers) != 0 || len(engine.disabled) != 0 {
		t.Fatalf("restoring the address did not clear the saved exclusion: %v, %v, %v", settings.DisabledTrackers, engine.disabled, err)
	}
}

func TestTrackerManagerRejectsInvalidManualBatch(t *testing.T) {
	s := uiFixture(t, "settings")
	s.ExtraTrackers = "udp://first.example:1337/announce"
	s.openTrackerManager()
	draft := s.trackerManager
	draft.edit("manual", -1, "udp://second.example:1337/announce\nftp://invalid.example/announce")
	draft.addManual()
	if !draft.failed || len(draft.manual) != 1 || draft.manual[0] != s.ExtraTrackers {
		t.Fatalf("invalid batch was partially applied: %+v", draft)
	}
}
