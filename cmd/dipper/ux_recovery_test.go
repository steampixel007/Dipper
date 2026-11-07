package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
)

type uxRecoveryEngine struct {
	resumeTestEngine
	admissionError error
	resumeError    error
	admitted       bool
	admissions     int
	started        chan struct{}
	release        chan struct{}
	startOnce      sync.Once
}

func (e *uxRecoveryEngine) Ensure(task core.DownloadTask) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.admissions++
	if e.admissionError != nil {
		return e.admissionError
	}
	e.admitted = true
	e.snapshot.Folder = task.Directory
	return nil
}

func (e *uxRecoveryEngine) Snapshot() []downloader.Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.admitted {
		return nil
	}
	return []downloader.Snapshot{e.snapshot}
}

func (e *uxRecoveryEngine) Resume(id string) error {
	e.mu.Lock()
	if id != e.snapshot.ID {
		e.mu.Unlock()
		return errors.New("unexpected recovery task")
	}
	e.commands = append(e.commands, "resume")
	err, admitted := e.resumeError, e.admitted
	e.mu.Unlock()
	if e.started != nil {
		e.startOnce.Do(func() { close(e.started) })
		<-e.release
	}
	if err != nil {
		return err
	}
	if !admitted {
		return downloader.ErrNotFound
	}
	e.mu.Lock()
	e.snapshot.Paused, e.snapshot.ManuallyStopped, e.snapshot.AutoStopped = false, false, false
	e.snapshot.Error = ""
	e.mu.Unlock()
	return nil
}

func uxRecoveryFixture(t *testing.T, engine *uxRecoveryEngine) (*State, *ui.Tester, chan func()) {
	t.Helper()
	root := t.TempDir()
	kernel, err := core.Open(filepath.Join(root, "dipper.db"), core.Options{
		DownloadRoot: filepath.Join(root, "downloads"), Downloader: engine, DisableScheduler: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := kernel.Close(); err != nil {
			t.Error(err)
		}
	})
	engine.snapshot.ID = strings.Repeat("ab", 20)
	resource := core.Resource{ID: "recovery-1", Title: "[Recovery] 01 [1080p]", Magnet: "magnet:?xt=urn:btih:" + engine.snapshot.ID}
	sub, err := kernel.CreateSubscription(t.Context(), core.Anime{ID: "recovery-anime", Title: "恢复下载"}, core.Group{ID: "recovery-group"}, core.Rule{}, []core.Resource{resource}, []string{resource.ID})
	if err != nil && sub.ID == "" {
		t.Fatal(err)
	}
	state, err := kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s := uiFixture(t, "episodes")
	s.Core, s.Engine, s.Ctx, s.LibraryID = kernel, engine, t.Context(), sub.ID
	updates := make(chan func(), 4)
	s.Update = func(fn func()) { updates <- fn }
	s.project(state, engine.Snapshot())
	return s, ui.NewTester(s.view, 1320, 860), updates
}

func uxAwaitRecovery(t *testing.T, s *State, updates <-chan func()) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for s.Busy || s.Notice == "" && s.SaveError == "" {
		select {
		case update := <-updates:
			update()
		case <-deadline.C:
			t.Fatalf("recovery did not finish: busy=%v downloads=%+v", s.Busy, s.Downloads)
		}
	}
	if s.Busy {
		t.Fatal("recovery left the interface busy after completion")
	}
}

func TestUIEpisodeLibraryRecoveryResumesOnce(t *testing.T) {
	for _, tc := range []struct {
		name    string
		paused  bool
		stopped bool
		failed  bool
	}{
		{name: "paused", paused: true},
		{name: "stopped", stopped: true},
		{name: "paused-and-stopped", paused: true, stopped: true},
		{name: "failed", failed: true},
		{name: "failed-and-paused", failed: true, paused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &uxRecoveryEngine{resumeTestEngine: resumeTestEngine{snapshot: downloader.Snapshot{
				Progress: .43, Paused: tc.paused, ManuallyStopped: tc.stopped,
			}}}
			if tc.failed {
				engine.snapshot.Error = "transfer interrupted"
			}
			s, tt, updates := uxRecoveryFixture(t, engine)
			button, notice := "继续", "下载状态已更新"
			if tc.failed {
				button, notice = "重试", "已重试下载"
			}
			libraryID := s.LibraryID
			uiClick(t, tt, button+" "+engine.snapshot.ID)
			uxAwaitRecovery(t, s, updates)
			engine.mu.Lock()
			commands := append([]string(nil), engine.commands...)
			engine.mu.Unlock()
			if len(commands) != 1 || commands[0] != "resume" {
				t.Fatalf("one recovery click sent %v; want exactly one resume", commands)
			}
			if len(s.Downloads) != 1 {
				t.Fatalf("downloads after recovery: %+v", s.Downloads)
			}
			download := s.Downloads[0]
			if download.Paused || download.Stopped || download.Failed || download.Progress != .43 {
				t.Fatalf("recovery did not preserve progress and clear failure/stop state: %+v", download)
			}
			if s.Notice != notice || s.SaveError != "" || s.Page != "episodes" || s.LibraryID != libraryID {
				t.Fatalf("recovery result: notice=%q error=%q page=%q library=%q", s.Notice, s.SaveError, s.Page, s.LibraryID)
			}
			tt.Frame()
			if tt.HasText("重试 "+download.ID) || tt.HasText("继续 "+download.ID) {
				t.Fatal("recovered row still offers a stale recovery action")
			}
		})
	}
}

func TestUIEpisodeLibraryRetryAdmitsPendingTask(t *testing.T) {
	engine := &uxRecoveryEngine{admissionError: errors.New("disk is full")}
	s, tt, updates := uxRecoveryFixture(t, engine)
	if len(s.Downloads) != 1 || !s.Downloads[0].Failed {
		t.Fatalf("expected a durable failed task: %+v", s.Downloads)
	}
	engine.admissionError = nil
	uiClick(t, tt, "重试 "+engine.snapshot.ID)
	uxAwaitRecovery(t, s, updates)
	engine.mu.Lock()
	admissions, admitted, commands := engine.admissions, engine.admitted, append([]string(nil), engine.commands...)
	engine.mu.Unlock()
	if admissions != 2 || !admitted || len(commands) != 1 || commands[0] != "resume" {
		t.Fatalf("retry did not use Resume then core admission: admissions=%d admitted=%v commands=%v", admissions, admitted, commands)
	}
	tasks, err := s.Core.ListTasks()
	if err != nil || len(tasks) != 1 || tasks[0].LastError != "" || s.Downloads[0].Failed || s.SaveError != "" {
		t.Fatalf("pending retry left failed core/UI state: tasks=%+v downloads=%+v error=%v/%q", tasks, s.Downloads, err, s.SaveError)
	}
}

func TestUIEpisodeLibraryRecoveryBusyAndError(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "continue"
		if failed {
			name = "retry"
		}
		t.Run(name, func(t *testing.T) {
			engine := &uxRecoveryEngine{
				resumeTestEngine: resumeTestEngine{snapshot: downloader.Snapshot{Progress: .43, Paused: !failed}},
				resumeError:      errors.New("recovery disk unavailable"),
				started:          make(chan struct{}),
				release:          make(chan struct{}),
			}
			if failed {
				engine.snapshot.Error = "transfer interrupted"
			}
			var releaseOnce sync.Once
			s, tt, updates := uxRecoveryFixture(t, engine)
			t.Cleanup(func() { releaseOnce.Do(func() { close(engine.release) }) })
			button := "继续 "
			if failed {
				button = "重试 "
			}
			button += engine.snapshot.ID
			s.Busy = true
			tt.Frame()
			uiClick(t, tt, button)
			engine.mu.Lock()
			calls := len(engine.commands)
			engine.mu.Unlock()
			if calls != 0 {
				t.Fatal("busy recovery dispatched an engine command")
			}
			s.Busy = false
			tt.Frame()
			uiClick(t, tt, button)
			select {
			case <-engine.started:
			case <-time.After(5 * time.Second):
				t.Fatal("recovery did not reach the engine")
			}
			if !s.Busy {
				t.Fatal("in-flight recovery did not guard the interface")
			}
			uiClick(t, tt, button)
			engine.mu.Lock()
			calls = len(engine.commands)
			engine.mu.Unlock()
			if calls != 1 {
				t.Fatalf("duplicate in-flight recovery: %d resume commands", calls)
			}
			releaseOnce.Do(func() { close(engine.release) })
			uxAwaitRecovery(t, s, updates)
			if s.SaveError != "recovery disk unavailable" || s.Page != "episodes" {
				t.Fatalf("recovery error lost: error=%q page=%q", s.SaveError, s.Page)
			}
			tt.Frame()
			uiRequireText(t, tt, "recovery disk unavailable")
			if _, ok := tt.Find(button); !ok {
				t.Fatal("failed recovery cannot be tried again from the episode row")
			}
			engine.mu.Lock()
			engine.resumeError = nil
			engine.mu.Unlock()
			uiClick(t, tt, button)
			uxAwaitRecovery(t, s, updates)
			engine.mu.Lock()
			calls = len(engine.commands)
			engine.mu.Unlock()
			if calls != 2 || s.SaveError != "" || s.Downloads[0].Failed || s.Downloads[0].Paused || s.Downloads[0].Stopped {
				t.Fatalf("recovery after an error failed: commands=%d error=%q downloads=%+v", calls, s.SaveError, s.Downloads)
			}
		})
	}
}

func TestUIEpisodeLibraryCompletedKeepsPlayAndFolder(t *testing.T) {
	s := uiFixture(t, "episodes")
	s.LibraryID = s.Subscriptions[0].ID
	download := s.Downloads[2]
	download.Progress, download.Paused, download.Stopped, download.Failed = 1, true, true, false
	download.Folder = t.TempDir()
	s.Downloads = []Download{download}
	path := filepath.Join(download.Folder, "episode.mkv")
	s.Engine = &playTestEngine{files: []string{path}}
	played := make(chan string, 1)
	s.PlayFile = func(path string) error { played <- path; return nil }
	opened := ""
	s.OpenFolder = func(folder string) { opened = folder }
	updates := make(chan func(), 4)
	s.Update = func(fn func()) { updates <- fn }
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "播放 "+download.ID)
	uxAwaitRecovery(t, s, updates)
	select {
	case actual := <-played:
		if actual != path {
			t.Fatalf("completed row opened %q, want %q", actual, path)
		}
	default:
		t.Fatal("completed row did not invoke playback")
	}
	tt.Frame()
	uiClick(t, tt, "打开文件夹 "+download.ID)
	if opened != download.Folder {
		t.Fatalf("completed row opened folder %q, want %q", opened, download.Folder)
	}
	if tt.HasText("继续 "+download.ID) || tt.HasText("重试 "+download.ID) {
		t.Fatal("completed episode offers recovery instead of playback")
	}
}

type uxRecoveryRemovalEngine struct {
	*removalTestEngine
	removed []string
}

func (e *uxRecoveryRemovalEngine) Remove(id string) error {
	e.mu.Lock()
	task := e.tasks[id]
	e.removed = append(e.removed, id)
	e.mu.Unlock()
	if err := e.removalTestEngine.Remove(id); err != nil {
		return err
	}
	return os.RemoveAll(task.Directory)
}

func TestUIRemoveExplicitFilesConfirmationAndSafeCancel(t *testing.T) {
	engine := &removalTestEngine{}
	s, tt, _, hash, updates := removalFixture(t, engine)
	fileEngine := &uxRecoveryRemovalEngine{removalTestEngine: engine}
	s.Engine = fileEngine
	engine.mu.Lock()
	task := engine.tasks[hash]
	engine.mu.Unlock()
	if err := os.MkdirAll(task.Directory, 0o755); err != nil {
		t.Fatal(err)
	}
	selectedFile := filepath.Join(task.Directory, "episode.mkv")
	if err := os.WriteFile(selectedFile, []byte("selected episode"), 0o600); err != nil {
		t.Fatal(err)
	}
	retainedFile := filepath.Join(t.TempDir(), "other-episode.mkv")
	if err := os.WriteFile(retainedFile, []byte("other episode"), 0o600); err != nil {
		t.Fatal(err)
	}
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
	viewport := tt.Image().Bounds()
	tt.Scroll(float32(viewport.Dx())*.6, float32(viewport.Dy())/2, 0, 400)
	tt.Frame()
	uiClick(t, tt, "删除")
	action, ok := tt.Find("确认删除任务和文件")
	text, textOK := tt.Find("删除任务和文件")
	if !ok || !textOK || text.X < action.X || text.X+text.W > action.X+action.W+1 {
		t.Fatalf("explicit files action truncates at 200%% text: action=%+v text=%+v", action, text)
	}
	uiClick(t, tt, "保留任务")
	if s.RemovePrompt || len(fileEngine.removed) != 0 {
		t.Fatal("cancel dispatched deletion or retained the dialog")
	}
	tasks, err := s.Core.ListTasks()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("cancel removed the core task: %+v %v", tasks, err)
	}
	if _, err := os.Stat(selectedFile); err != nil {
		t.Fatalf("cancel removed episode files: %v", err)
	}
	uiClick(t, tt, "删除")
	uiRequireText(t, tt, "删除任务和文件")
	uiClick(t, tt, "确认删除任务和文件")
	if s.RemovePrompt {
		t.Fatal("one confirmation did not dismiss the deletion dialog")
	}
	awaitRemovalAction(t, s, updates)
	tasks, err = s.Core.ListTasks()
	if err != nil || len(tasks) != 0 || len(fileEngine.removed) != 1 || fileEngine.removed[0] != hash {
		t.Fatalf("confirmation did not delete exactly the selected task: tasks=%+v removed=%v error=%v", tasks, fileEngine.removed, err)
	}
	if _, err := os.Stat(selectedFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("selected files remain after confirmed deletion: %v", err)
	}
	if _, err := os.Stat(retainedFile); err != nil {
		t.Fatalf("deletion affected unrelated files: %v", err)
	}
}
