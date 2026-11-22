package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
	"github.com/steampixel007/Dipper/internal/proxy"
	"github.com/steampixel007/Dipper/internal/source"
)

func TestUIProxyCustomDraftKeepsActiveSettingsUntilSave(t *testing.T) {
	s := uiFixture(t, "settings")
	kernel, err := core.Open(filepath.Join(s.Dir, "dipper.db"), core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close(); proxy.Configure(proxy.ModeAuto, "") })
	s.Core = kernel
	tt := ui.NewTester(s.view, 1040, 720)
	uiScrollTo(t, tt, "自定义")
	uiClick(t, tt, "自定义")
	if s.SaveError != "" {
		t.Fatalf("opening the custom editor raised a save error: %s", s.SaveError)
	}
	uiScrollTo(t, tt, "保存代理")
	uiClick(t, tt, "保存代理")
	uiRequireText(t, tt, "请填写代理地址。")
	uiScrollTo(t, tt, "代理地址")
	uiClick(t, tt, "代理地址")
	tt.Type("127.0.0.1:7897")
	s.save()
	settings, err := kernel.GetSettings()
	if err != nil || settings.ProxyMode == proxy.ModeCustom || s.SaveError != "" {
		t.Fatalf("general save applied the proxy draft: mode=%q, UI error=%q, error=%v", settings.ProxyMode, s.SaveError, err)
	}
	uiScrollTo(t, tt, "代理用户名")
	uiClick(t, tt, "代理用户名")
	tt.Type("demo@local")
	uiScrollTo(t, tt, "代理密码")
	uiClick(t, tt, "代理密码")
	tt.Type("fixture:only")
	uiScrollTo(t, tt, "保存代理")
	uiClick(t, tt, "保存代理")
	settings, err = kernel.GetSettings()
	address, username, password := proxy.SplitAddress(settings.ProxyURL)
	if err != nil || settings.ProxyMode != proxy.ModeCustom || address != "http://127.0.0.1:7897" || username != "demo@local" || password != "fixture:only" {
		t.Fatalf("explicit save did not apply the authenticated proxy: mode=%q, error=%v", settings.ProxyMode, err)
	}
	uiRequireText(t, tt, "自定义代理：http://127.0.0.1:7897")
}

type downloadLimitTestEngine struct {
	downloadEngine
	limit int
}

func (engine *downloadLimitTestEngine) SetDownloadLimit(limit int)     { engine.limit = limit }
func (*downloadLimitTestEngine) SetUploadLimit(int64) error            { return nil }
func (*downloadLimitTestEngine) SetSeedTarget(time.Duration)           {}
func (*downloadLimitTestEngine) SetRatioTarget(float64)                {}
func (*downloadLimitTestEngine) SetTrackerSettings([]string, []string) {}
func (*downloadLimitTestEngine) SetRemoteTrackers([]string)            {}
func (*downloadLimitTestEngine) Snapshot() []downloader.Snapshot       { return nil }

func TestUISettingsDownloadLimitKeepsIndependentSettings(t *testing.T) {
	const manual = "udp://manual.example:1337/announce"
	const replacement = "https://tracker.example/announce"
	s := uiFixture(t, "settings")
	database := filepath.Join(s.Dir, "dipper.db")
	kernel, err := core.Open(database, core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })
	engine := &downloadLimitTestEngine{}
	s.Core, s.Engine, s.Source = kernel, engine, source.New("")
	if err := s.loadSaved(); err != nil {
		t.Fatal(err)
	}
	lists := []core.TrackerList{{URL: "https://lists.example/trackers.txt", Trackers: []string{replacement}}}
	if err := s.saveTrackerSettings(manual, lists, nil, false); err != nil {
		t.Fatal(err)
	}
	s.TransferLimits.duration.value = "37"

	tt := ui.NewTester(s.view, 1040, 720)
	uiScrollTo(t, tt, "填写同时下载任务数")
	uiClick(t, tt, "填写同时下载任务数")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("11")
	tt.Key(0, ui.KeyEnter)
	uiRequireText(t, tt, "请输入 1–10 之间的整数。")
	invalid, err := kernel.GetSettings()
	if err != nil || invalid.DownloadLimit != 0 || engine.limit != 0 {
		t.Fatalf("invalid input changed the default: saved=%d live=%d error=%v", invalid.DownloadLimit, engine.limit, err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("1")
	tt.Key(0, ui.KeyEnter)
	settings, err := kernel.GetSettings()
	if err != nil || settings.DownloadLimit != 1 || engine.limit != 1 || s.SaveError != "" {
		t.Fatalf("selected limit did not persist and apply: saved=%d live=%d UI=%q error=%v", settings.DownloadLimit, engine.limit, s.SaveError, err)
	}
	if settings.ExtraTrackers != manual || len(settings.TrackerLists) != 1 || settings.TrackerLists[0].URL != lists[0].URL ||
		settings.SeedMinutes != 0 || s.TransferLimits.duration.value != "37" {
		t.Fatalf("saving concurrency changed tracker settings or the duration draft: %+v, draft=%+v", settings, s.TransferLimits)
	}
	s.saveTransferLimits()
	if err := s.saveTrackerSettings(replacement, lists, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := kernel.Close(); err != nil {
		t.Fatal(err)
	}
	uiClick(t, tt, "填写同时下载任务数")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("2")
	uiClick(t, tt, "保存同时下载任务数")
	uiRequireText(t, tt, "无法保存同时下载任务数，请重试。")
	if s.DownloadLimit != 1 || engine.limit != 1 || s.SaveError != "" {
		t.Fatalf("failed save changed the active limit or the global error: state=%d live=%d error=%q", s.DownloadLimit, engine.limit, s.SaveError)
	}
	reopened, err := core.Open(database, core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	reloaded := uiFixture(t, "settings")
	reopenedEngine := &downloadLimitTestEngine{}
	reloaded.Core, reloaded.Engine, reloaded.Source = reopened, reopenedEngine, source.New("")
	if err := reloaded.loadSaved(); err != nil {
		t.Fatal(err)
	}
	if reloaded.DownloadLimit != 1 || reopenedEngine.limit != 1 || reloaded.ExtraTrackers != replacement || reloaded.TransferLimits.duration.value != "37" {
		t.Fatalf("restarted with limit=%d/%d trackers=%q duration=%q", reloaded.DownloadLimit, reopenedEngine.limit, reloaded.ExtraTrackers, reloaded.TransferLimits.duration.value)
	}
}

func TestUITransferLimitsPersistAndRetainFiniteValues(t *testing.T) {
	kernel, err := core.Open(filepath.Join(t.TempDir(), "dipper.db"), core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })
	s := uiFixture(t, "settings")
	s.Core = kernel
	settings, err := kernel.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.Root = settings.DownloadRoot
	s.loadTransferLimits(settings)
	s.TransferLimits.upload = limitInput{value: "1.25"}
	s.TransferLimits.duration = limitInput{value: "37"}
	s.TransferLimits.ratio = limitInput{value: "2.75"}
	s.save()
	settings, err = kernel.GetSettings()
	if err != nil || s.SaveError != "" || settings.EffectiveUploadLimit() != 1048576 || settings.EffectiveSeedDuration() != 0 || settings.EffectiveSeedRatio() != 1 {
		t.Fatalf("general save affected transfer draft: %+v, error=%v, UI error=%q", settings, err, s.SaveError)
	}
	s.saveTransferLimits()
	settings, err = kernel.GetSettings()
	if err != nil || settings.EffectiveUploadLimit() != 1310720 || settings.EffectiveSeedDuration() != 37*time.Minute || settings.EffectiveSeedRatio() != 2.75 {
		t.Fatalf("custom limits not applied: %+v, error=%v", settings, err)
	}
	s.TransferLimits.upload.unlimited = true
	s.TransferLimits.duration.unlimited = true
	s.TransferLimits.ratio.unlimited = true
	s.saveTransferLimits()
	settings, err = kernel.GetSettings()
	if err != nil || settings.EffectiveUploadLimit() != -1 || settings.EffectiveSeedDuration() != -1 || settings.EffectiveSeedRatio() != -1 {
		t.Fatalf("unlimited choices not applied: %+v, error=%v", settings, err)
	}
	s.loadTransferLimits(settings)
	if s.TransferLimits.upload.value != "1.25" || s.TransferLimits.duration.value != "37" || s.TransferLimits.ratio.value != "2.75" {
		t.Fatalf("finite values lost: %+v", s.TransferLimits)
	}
}
