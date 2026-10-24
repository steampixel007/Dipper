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

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
	"dipper.local/dipper/internal/instance"
	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const version = "0.1.0"

func main() {
	render := flag.String("render", "", "Render fixture screenshots for UI review")
	data := flag.String("data", "", "Application data directory")
	origin := flag.String("source", "", "Mikan origin override")
	probe := flag.Bool("probe-source", false, "Read public catalog, detail, history and RSS without starting downloads")
	quitAfter := flag.Duration("quit-after", 0, "Quit after a duration for native startup checks")
	root := flag.String("download-root", "", "Initial download root for new application data")
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
	if *data == "" {
		exe, _ := os.Executable()
		*data = filepath.Join(filepath.Dir(exe), "..", "data")
	}
	abs, err := filepath.Abs(*data)
	if err != nil {
		failStartup(err)
		return
	}
	if err = os.MkdirAll(abs, 0755); err != nil {
		failStartup(err)
		return
	}
	launchCtx, launchCancel := context.WithTimeout(context.Background(), 5*time.Second)
	owner, err := instance.Acquire(launchCtx, abs)
	launchCancel()
	if err != nil {
		failStartup(err)
		return
	}
	if owner == nil {
		return
	}
	defer owner.Close()
	logPath := filepath.Join(abs, "app.log")
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
	mygo.App.SetPath(mygo.PathUserData, abs)
	mygo.Theme.SetSource(mygo.ThemeLight)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newState(abs)
	s.Ctx = ctx
	s.Source = src
	adapter := &engineAdapter{}
	kernel, err := core.Open(filepath.Join(abs, "dipper.db"), core.Options{DownloadRoot: *root, Downloader: adapter, Feed: func(ctx context.Context, animeID, groupID string) ([]core.Resource, error) {
		resources, err := src.Feed(ctx, animeID, groupID)
		return coreResources(resources), err
	}})
	if err != nil {
		failStartup(fmt.Errorf("无法打开追番状态：%w", err))
		return
	}
	defer kernel.Close()
	s.Core = kernel
	settings, err := kernel.GetSettings()
	if err != nil {
		failStartup(err)
		return
	}
	engine, err := downloader.New(downloader.Config{DataDir: filepath.Join(abs, "engine"), UploadLimit: settings.UploadLimitBytes})
	if err != nil {
		failStartup(fmt.Errorf("无法打开下载状态：%w", err))
		return
	}
	defer engine.Close()
	adapter.engine = engine
	s.Engine = engine
	mygo.Power.OnSuspend(func() { engine.SetSuspended(true) })
	mygo.Power.OnResume(func() { engine.SetSuspended(false) })
	if err = s.loadSaved(); err != nil {
		failStartup(err)
		return
	}
	if err = kernel.Start(); err != nil {
		failStartup(err)
		return
	}
	quitting := false
	s.Quit = func() { mygo.App.Quit() }
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
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
		win := mygo.NewWindow(mygo.WindowOptions{Title: "Dipper · 自动追番", Width: 1320, Height: 900, MinWidth: 1040, MinHeight: 720, TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 64, BackgroundColor: "#f3f3f3", Content: ui.View(s.view)})
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
		tray, err := mygo.NewTray(mygo.TrayOptions{Icon: brandTrayPNG, ToolTip: "Dipper · 自动追番", Menu: mygo.NewMenu([]*mygo.MenuItem{
			{Label: "打开 Dipper", Click: func(*mygo.MenuItem, *mygo.Window) { showMain() }},
			{Label: "完全退出", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
		})})
		if err == nil && tray != nil {
			win.OnClose(func(e *mygo.CloseEvent) {
				if !quitting {
					e.PreventDefault()
					win.Hide()
					slog.Info("window hidden; background active")
				}
			})
		} else {
			s.Notice = "系统托盘不可用，关闭窗口将完全退出。"
			win.OnClose(func(*mygo.CloseEvent) { mygo.App.Quit() })
		}
		s.refreshCatalog()
		_ = os.WriteFile(filepath.Join(abs, "ready"), []byte(version), 0644)
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
