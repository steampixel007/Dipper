package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
)

func TestUISettingsShowBuiltinTrackers(t *testing.T) {
	s := uiFixture(t, "settings")
	tt := ui.NewTester(s.view, 1040, 720)
	uiScrollTo(t, tt, "管理 Tracker")
	uiClick(t, tt, "管理 Tracker")
	uiRequireText(t, tt, "管理下载连接", "手动添加的服务器", "订阅的列表", "Dipper 自动提供的地址")
	uiClick(t, tt, "查看内置地址")
	uiRequireText(t, tt, "内置备用地址", "udp://tracker.opentrackr.org:1337/announce")
	uiClick(t, tt, "取消")
	if s.trackerManagerOpen {
		t.Fatal("cancel left the manager open")
	}
}

type trackerTestEngine struct {
	downloadEngine
	applied []string
}

func (engine *trackerTestEngine) SetExtraTrackers(urls []string) {
	engine.applied = slices.Clone(urls)
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
	uiScrollTo(t, tt, "管理 Tracker")
	uiClick(t, tt, "管理 Tracker")
	uiClick(t, tt, "添加服务器")
	draft := s.trackerManager
	draft.input = manual + "\n" + manual
	tt.Frame()
	uiClick(t, tt, "添加到列表")
	uiClick(t, tt, "订阅列表")
	draft.input = server.URL
	tt.Frame()
	uiClick(t, tt, "预览地址")
	waitForRefresh()
	tt.Frame()
	uiRequireText(t, tt, "读到 2 个有效地址，其中 1 个已配置。")
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
	draft.lists[0].Disabled = true
	tt.Frame()
	uiClick(t, tt, "取消")
	if s.TrackerLists[0].Disabled || !slices.Equal(engine.applied, want) {
		t.Fatal("cancel applied edits")
	}
	s.openTrackerManager()
	s.trackerManager.lists[0].Disabled = true
	tt.Frame()
	uiClick(t, tt, "保存修改")
	settings, err = kernel.GetSettings()
	if err != nil || !settings.TrackerLists[0].Disabled || !slices.Equal(engine.applied, []string{manual}) {
		t.Fatalf("disabled list was not persisted and removed from the user tier: %+v, applied=%v, error=%v", settings, engine.applied, err)
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
