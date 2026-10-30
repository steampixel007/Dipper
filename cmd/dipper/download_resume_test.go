package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
	"github.com/egoist/mygo/ui"
)

type resumeTestEngine struct {
	mu       sync.Mutex
	snapshot downloader.Snapshot
	commands []string
}

func (e *resumeTestEngine) Ensure(task core.DownloadTask) error {
	if task.Hash != e.snapshot.ID {
		return fmt.Errorf("unexpected task %q", task.Hash)
	}
	return nil
}

func (e *resumeTestEngine) Snapshot() []downloader.Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return []downloader.Snapshot{e.snapshot}
}

func (e *resumeTestEngine) Pause(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if id != e.snapshot.ID {
		return fmt.Errorf("pause of unexpected task %q", id)
	}
	e.commands = append(e.commands, "pause")
	e.snapshot.Paused = true
	return nil
}

func (e *resumeTestEngine) Resume(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if id != e.snapshot.ID {
		return fmt.Errorf("resume of unexpected task %q", id)
	}
	e.commands = append(e.commands, "resume")
	e.snapshot.Paused, e.snapshot.ManuallyStopped = false, false
	return nil
}

func (*resumeTestEngine) PlayableFiles(string) ([]string, error) {
	return nil, fmt.Errorf("unexpected play")
}
func (*resumeTestEngine) Stop(string) error           { return fmt.Errorf("unexpected stop") }
func (*resumeTestEngine) Remove(string) error         { return fmt.Errorf("unexpected removal") }
func (*resumeTestEngine) SetUploadLimit(int64) error  { return fmt.Errorf("unexpected upload limit") }
func (*resumeTestEngine) SetSeedTarget(time.Duration) {}

func TestUIDownloadTransferActionChangesIntentOnce(t *testing.T) {
	for _, tc := range []struct {
		name       string
		paused     bool
		stopped    bool
		button     string
		command    string
		wantPaused bool
	}{
		{name: "stopped", stopped: true, button: "继续", command: "resume"},
		{name: "paused", paused: true, button: "继续", command: "resume"},
		{name: "paused-and-stopped", paused: true, stopped: true, button: "继续", command: "resume"},
		{name: "downloading", button: "暂停", command: "pause", wantPaused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hash := strings.Repeat("ab", 20)
			engine := &resumeTestEngine{snapshot: downloader.Snapshot{
				ID: hash, Progress: .43, Paused: tc.paused, ManuallyStopped: tc.stopped,
			}}
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
			resource := core.Resource{ID: "episode-1", Title: "[Resume] 01 [1080p]", Magnet: "magnet:?xt=urn:btih:" + hash}
			if _, err := kernel.CreateSubscription(context.Background(), core.Anime{ID: "resume-anime", Title: "继续下载"}, core.Group{ID: "resume-group"}, core.Rule{}, []core.Resource{resource}, []string{resource.ID}); err != nil {
				t.Fatal(err)
			}
			state, err := kernel.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			s := uiFixture(t, "downloads")
			s.Core, s.Engine, s.Ctx = kernel, engine, context.Background()
			updates := make(chan func(), 2)
			s.Update = func(fn func()) { updates <- fn }
			s.project(state, engine.Snapshot())
			tt := ui.NewTester(s.view, 1320, 860)
			uiClick(t, tt, tc.button)
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			for s.Notice != "下载状态已更新" {
				select {
				case update := <-updates:
					update()
				case <-deadline.C:
					t.Fatalf("download action did not finish: error=%q", s.SaveError)
				}
			}
			if s.SaveError != "" {
				t.Fatalf("download action failed: %s", s.SaveError)
			}
			engine.mu.Lock()
			commands := append([]string(nil), engine.commands...)
			engine.mu.Unlock()
			if len(commands) != 1 || commands[0] != tc.command {
				t.Errorf("one %q click sent %v; want one %s command", tc.button, commands, tc.command)
			}
			if len(s.Downloads) != 1 {
				t.Fatalf("projected downloads = %+v", s.Downloads)
			}
			download := s.Downloads[0]
			if download.Paused != tc.wantPaused || download.Stopped || download.Progress != .43 {
				t.Errorf("one %q click left paused=%v stopped=%v progress=%v; want paused=%v stopped=false progress=0.43", tc.button, download.Paused, download.Stopped, download.Progress, tc.wantPaused)
			}
			tt.Frame()
			next := "暂停"
			if tc.wantPaused {
				next = "继续"
			}
			if _, ok := tt.Find(next); !ok {
				t.Errorf("updated download card lacks %q action", next)
			}
		})
	}
}
