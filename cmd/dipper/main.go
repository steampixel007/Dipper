package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/appdata"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
	"github.com/steampixel007/Dipper/internal/instance"
	"github.com/steampixel007/Dipper/internal/proxy"
	"github.com/steampixel007/Dipper/internal/source"
	"github.com/steampixel007/Dipper/internal/trackers"
	"github.com/steampixel007/Dipper/internal/update"
)

var version = "dev"

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--software-update-helper" {
		os.Exit(update.RunHelper(os.Args[2]))
	}
	render := flag.String("render", "", "Render fixture screenshots for UI review")
	data := flag.String("data", "", "Application data directory")
	origin := flag.String("source", "", "Mikan origin override")
	probe := flag.Bool("probe-source", false, "Read public catalog, detail, history and RSS without starting downloads")
	quitAfter := flag.Duration("quit-after", 0, "Quit after a duration for native startup checks")
	root := flag.String("download-root", "", "Initial download root for new application data")
	readyAddress := flag.String("software-update-ready-address", "", "Private post-update readiness address")
	readyToken := flag.String("software-update-ready-token", "", "Private post-update readiness token")
	flag.Parse()
	loadImages()
	if *render != "" {
		if err := renderScreens(*render); err != nil {
			log.Fatal(err)
		}
		return
	}
	src := source.New(*origin)
	if *probe {
		if err := probeSource(src); err != nil {
			log.Fatal(err)
		}
		return
	}
	var launchGate *update.Gate
	if *readyAddress == "" && *readyToken == "" {
		executable, executableErr := os.Executable()
		if executableErr != nil {
			failStartup(executableErr)
			return
		}
		gateCtx, gateCancel := context.WithTimeout(context.Background(), 5*time.Second)
		gate, gateErr := update.AcquireLaunchGate(gateCtx, executable)
		gateCancel()
		if gateErr != nil {
			failStartup(fmt.Errorf("%s%w", softwareText.startBlocked, gateErr))
			return
		}
		launchGate = gate
		defer func() {
			if launchGate != nil {
				launchGate.Close()
			}
		}()
		update.CleanupCompleted(executable)
	} else {
		if err := update.AuthorizeRestart(*readyAddress, *readyToken); err != nil {
			failStartup(fmt.Errorf("%s%w", softwareText.handshakeRejected, err))
			return
		}
	}
	paths, err := appdata.Resolve(*data)
	if err != nil {
		failStartup(err)
		return
	}
	if err = os.MkdirAll(paths.DataDir, 0700); err != nil {
		failStartup(err)
		return
	}
	launchCtx, launchCancel := context.WithTimeout(context.Background(), 5*time.Second)
	owner, err := instance.Acquire(launchCtx, paths.DataDir)
	launchCancel()
	if err != nil {
		failStartup(err)
		return
	}
	if owner == nil {
		return
	}
	if launchGate != nil {
		launchGate.Close()
		launchGate = nil
	}
	defer owner.Close()
	if err = os.MkdirAll(paths.LogDir, 0700); err != nil {
		failStartup(err)
		return
	}
	if err = os.MkdirAll(paths.ConfigDir, 0700); err != nil {
		failStartup(err)
		return
	}
	logPath := paths.Log()
	if info, e := os.Stat(logPath); e == nil && info.Size() > 2<<20 {
		_ = os.Remove(logPath + ".1")
		_ = os.Rename(logPath, logPath+".1")
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		failStartup(err)
		return
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, nil)))
	mygo.App.SetName("Dipper")
	mygo.App.SetPath(mygo.PathUserData, paths.DataDir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter := &engineAdapter{}
	kernel, err := core.Open(filepath.Join(paths.DataDir, "dipper.db"), core.Options{DownloadRoot: *root, Downloader: adapter, Feed: func(ctx context.Context, animeID, groupID string) ([]core.Resource, error) {
		resources, err := src.Feed(ctx, animeID, groupID)
		return coreResources(resources), err
	}})
	if err != nil {
		failStartup(fmt.Errorf("无法打开追番状态：%w", err))
		return
	}
	defer kernel.Close()
	settings, err := kernel.GetSettings()
	if err != nil {
		failStartup(err)
		return
	}
	proxy.Configure(settings.ProxyMode, settings.ProxyURL)
	trackersPath := filepath.Join(paths.DataDir, "trackers.json")
	savedList := trackers.Load(trackersPath)
	engine, err := downloader.New(downloader.Config{DataDir: filepath.Join(paths.DataDir, "engine"), UploadLimit: settings.UploadLimitBytes, SeedTarget: seedDuration(settings.SeedMinutes), RatioTarget: core.SeedRatioTarget(settings.SeedRatio), ExtraTrackers: core.ParseTrackers(settings.ExtraTrackers), RemoteTrackers: savedList.URLs})
	if err != nil {
		failStartup(fmt.Errorf("无法打开下载状态：%w", err))
		return
	}
	defer engine.Close()
	adapter.engine = engine
	if savedList.Stale(time.Now()) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			list, err := trackers.Refresh(ctx, trackersPath, time.Now(), trackers.FetchHTTP)
			if err != nil {
				slog.Warn("tracker list refresh failed", "err", err)
			}
			engine.SetRemoteTrackers(list.URLs)
		}()
	}
	s := newAppState(ctx, paths.DataDir, src, kernel, engine)
	s.ConfigDir = paths.ConfigDir
	s.CoverDir = paths.CoverDir
	mygo.Power.OnSuspend(func() { engine.SetSuspended(true) })
	mygo.Power.OnResume(func() { engine.SetSuspended(false) })
	if err = s.loadSaved(); err != nil {
		failStartup(err)
		return
	}
	s.ApplyAppearance = func(value string) { mygo.Theme.SetSource(mygo.ThemeSource(normalizeAppearance(value))) }
	s.ApplyAppearance(s.Appearance)
	if *origin != "" {
		if err = src.Retarget(*origin); err != nil {
			failStartup(err)
			return
		}
		s.SourceRoute = s.sourceLabel(*origin)
	}
	if err = kernel.Start(); err != nil {
		failStartup(err)
		return
	}
	quitting := false
	s.Quit = func() { mygo.App.Quit() }
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if !s.prepareAppQuit() {
			e.PreventDefault()
			return
		}
		quitting = true
		cancel()
		if err := kernel.Close(); err != nil {
			slog.Error("core shutdown", "error", err)
		}
		if err := engine.Close(); err != nil {
			slog.Error("engine shutdown", "error", err)
		}
		slog.Info("stopped", "version", version)
	})
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{Title: "Dipper · 自动追番", Width: 1320, Height: 900, MinWidth: 1040, MinHeight: 720, TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 64, BackgroundColor: windowBackground(), Content: ui.View(s.view)})
		if err := win.SetIcon(brandIconPNG); err != nil {
			slog.Warn("window icon", "error", err)
		}
		s.Update = func(fn func()) {
			if ctx.Err() == nil {
				win.Update(func() {
					if ctx.Err() == nil {
						fn()
					}
				})
			}
		}
		showMain := func() {
			win.Show()
			if win.IsMinimized() {
				win.Restore()
			}
			win.Focus()
		}
		mygo.App.OnActivate(func(bool) { showMain() })
		s.ShowWindow = showMain
		s.HideWindow = func() {
			win.Hide()
			slog.Info("window hidden; background active")
		}
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-owner.Activations():
					s.Update(func() {
						showMain()
						slog.Info("existing instance activated", "visible", win.IsVisible(), "focused", win.IsFocused(), "minimized", win.IsMinimized())
					})
				}
			}
		}()
		s.OpenFolder = func(path string) {
			if err := mygo.Shell.OpenPath(path); err != nil {
				s.Notice = "无法打开文件夹：" + err.Error()
			}
		}
		s.PlayFile = func(path string) error {
			if err := mygo.Shell.OpenPath(path); err != nil {
				return fmt.Errorf("无法播放：%w", err)
			}
			return nil
		}
		s.CopyText = func(text string) {
			mygo.Clipboard.WriteText(text)
		}
		s.PickFolder = func() {
			current := s.Root
			go func() {
				paths, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "选择番剧下载位置", Directory: true, DefaultPath: current})
				s.Update(func() {
					if e != nil {
						s.SaveError = e.Error()
					} else if len(paths) > 0 {
						s.Root = paths[0]
						s.save()
					}
				})
			}()
		}
		trays := &trayController{platform: s.Platform, create: func() (desktopTray, error) {
			tray, err := mygo.NewTray(mygo.TrayOptions{ID: "dipper.main", Icon: brandTrayPNG, IconIsTemplate: s.Platform == "darwin", ToolTip: "Dipper · 自动追番", Menu: mygo.NewMenu([]*mygo.MenuItem{
				{Label: trayMenuLabel(s.Platform, "打开 Dipper"), Click: func(*mygo.MenuItem, *mygo.Window) { showMain() }},
				{Label: trayMenuLabel(s.Platform, windowText.quit), Click: func(*mygo.MenuItem, *mygo.Window) { s.exitApp() }},
			})})
			if err != nil {
				return nil, err
			}
			if s.Platform != "darwin" {
				setTrayIcon := trayIconSetter(tray.Dark, func(png []byte) error { return tray.SetIcon(png, false) })
				setTrayIcon()
				tray.OnAppearanceChange(setTrayIcon)
			}
			tray.OnClick(showMain)
			return tray, nil
		}}
		s.CheckTray = trays.ensure
		s.TrayAvailable = trays.ensure()
		mygo.App.OnWillQuit(func(*mygo.QuitEvent) { trays.destroy() })
		win.OnClose(func(e *mygo.CloseEvent) {
			if !quitting {
				e.PreventDefault()
				win.Update(s.requestClose)
			}
		})
		s.startAutomaticSoftwareCheck()
		if err := update.NotifyReady(*readyAddress, *readyToken); err != nil {
			slog.Error("post-update readiness handshake failed", "error", err)
			s.SoftwareUpdate.StartupError = softwareText.startupError
		}
		s.refreshCatalog()
		_ = os.WriteFile(filepath.Join(paths.DataDir, "ready"), []byte(version), 0644)
		slog.Info("ready", "version", version)
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					state, e := kernel.Snapshot()
					snapshots := engine.Snapshot()
					s.Update(func() {
						if e != nil {
							s.SaveError = e.Error()
							return
						}
						s.project(state, snapshots)
					})
				}
			}
		}()
		if *quitAfter > 0 {
			go func() {
				timer := time.NewTimer(*quitAfter)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					mygo.App.Quit()
				}
			}()
		}
	})
	if err = mygo.App.Run(); err != nil {
		failStartup(err)
	}
}

func newAppState(ctx context.Context, dir string, src *source.Client, kernel *core.Core, engine *downloader.Engine) *State {
	s := newState(dir)
	s.Ctx = ctx
	s.Source = src
	s.Core = kernel
	s.Engine = engine
	return s
}

func trayIconPNG(dark bool) []byte {
	if dark {
		return brandTrayDarkPNG
	}
	return brandTrayPNG
}

func trayIconSetter(dark func() bool, set func(png []byte) error) func() {
	return func() {
		if err := set(trayIconPNG(dark())); err != nil {
			slog.Warn("tray icon", "error", err)
		}
	}
}

func failStartup(err error) {
	log.Print(err)
	showStartupError(err.Error())
}

func probeSource(src *source.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	now := time.Now()
	catalog, err := src.Catalog(ctx, now.Year(), seasons[(int(now.Month())-1)/3])
	if err != nil {
		return err
	}
	detail, err := src.Detail(ctx, "4063")
	if err != nil {
		return err
	}
	history, err := src.History(ctx, "4063", "370")
	if err != nil {
		return err
	}
	feed, err := src.Feed(ctx, "4063", "370")
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(map[string]any{"catalog": len(catalog), "anime": detail.Anime, "groups": len(detail.Groups), "history": len(history), "rss": len(feed)}, "", "  ")
	fmt.Println(string(b))
	return nil
}
