package main

import (
	"context"
	"errors"
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

type removalTestEngine struct {
	mu          sync.Mutex
	tasks       map[string]core.DownloadTask
	calls       int
	fail        bool
	cleanupErr  error
	commitErr   error
	started     chan struct{}
	allow       chan struct{}
	finished    chan struct{}
	releaseOnce sync.Once
}

func (e *removalTestEngine) Ensure(task core.DownloadTask) error {
	e.mu.Lock()
	e.calls++
	fail, started, allow, finished := e.fail, e.started, e.allow, e.finished
	e.mu.Unlock()
	if fail {
		return errors.New("disk is full")
	}
	if started != nil {
		close(started)
		defer close(finished)
		select {
		case <-task.Context.Done():
			return task.Context.Err()
		case <-allow:
		}
	}
	if err := task.Context.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tasks == nil {
		e.tasks = make(map[string]core.DownloadTask)
	}
	e.tasks[task.Hash] = task
	return nil
}

func (e *removalTestEngine) Snapshot() []downloader.Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	var snapshots []downloader.Snapshot
	for id, task := range e.tasks {
		snapshots = append(snapshots, downloader.Snapshot{ID: id, Folder: task.Directory, Progress: .43})
	}
	return snapshots
}

func (e *removalTestEngine) Remove(id string) error {
	e.mu.Lock()
	_, exists := e.tasks[id]
	allow, finished, commitErr, cleanupErr := e.allow, e.finished, e.commitErr, e.cleanupErr
	e.mu.Unlock()
	if !exists {
		if allow != nil {
			// Model admission completing after Remove observed no registered task.
			e.releaseOnce.Do(func() { close(allow) })
			<-finished
		}
		return downloader.ErrNotFound
	}
	if commitErr != nil {
		return commitErr
	}
	e.mu.Lock()
	delete(e.tasks, id)
	e.mu.Unlock()
	return cleanupErr
}

func (*removalTestEngine) PlayableFiles(string) ([]string, error) {
	return nil, fmt.Errorf("unexpected play")
}
func (*removalTestEngine) Pause(string) error          { return fmt.Errorf("unexpected pause") }
func (*removalTestEngine) Resume(string) error         { return fmt.Errorf("unexpected resume") }
func (*removalTestEngine) Stop(string) error           { return fmt.Errorf("unexpected stop") }
func (*removalTestEngine) SetUploadLimit(int64) error  { return fmt.Errorf("unexpected upload limit") }
func (*removalTestEngine) SetSeedTarget(time.Duration) {}

func removalFixture(t *testing.T, engine *removalTestEngine) (*State, *ui.Tester, core.Subscription, string, chan func()) {
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
	hash := strings.Repeat("ef", 20)
	resource := core.Resource{ID: "remove-1", Title: "[Remove] 01 [1080p]", Magnet: "magnet:?xt=urn:btih:" + hash}
	sub, err := kernel.CreateSubscription(t.Context(), core.Anime{ID: "remove-anime", Title: "删除任务"}, core.Group{ID: "remove-group"}, core.Rule{}, []core.Resource{resource}, []string{resource.ID})
	if err != nil && sub.ID == "" {
		t.Fatal(err)
	}
	state, err := kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s := uiFixture(t, "downloads")
	s.Core, s.Engine, s.Ctx = kernel, engine, t.Context()
	updates := make(chan func(), 2)
	s.Update = func(fn func()) { updates <- fn }
	s.project(state, engine.Snapshot())
	return s, ui.NewTester(s.view, 1320, 860), sub, hash, updates
}

func awaitRemovalAction(t *testing.T, s *State, updates <-chan func()) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for range 2 {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatalf("removal did not finish: notice=%q error=%q", s.Notice, s.SaveError)
		}
	}
}

func TestUIDeleteDrainsAdmissionBeforeRemovingTransfer(t *testing.T) {
	engine := &removalTestEngine{fail: true}
	s, tt, _, hash, updates := removalFixture(t, engine)
	engine.mu.Lock()
	engine.fail = false
	engine.started, engine.allow, engine.finished = make(chan struct{}), make(chan struct{}), make(chan struct{})
	started, finished := engine.started, engine.finished
	engine.mu.Unlock()
	retry := make(chan error, 1)
	go func() { retry <- s.Core.RetryPending(t.Context()) }()
	t.Cleanup(func() {
		engine.releaseOnce.Do(func() { close(engine.allow) })
		<-finished
	})
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("retry did not reach admission")
	}
	uiClick(t, tt, "删除")
	uiClick(t, tt, "确认删除下载任务")
	awaitRemovalAction(t, s, updates)
	<-retry
	if s.SaveError != "" || s.Notice != "已删除该下载任务" {
		t.Fatalf("delete result: notice=%q error=%q", s.Notice, s.SaveError)
	}
	if snapshots := engine.Snapshot(); len(snapshots) != 0 {
		t.Fatalf("confirmed deletion left a transfer admitted after Remove: %+v", snapshots)
	}
	if tasks, err := s.Core.ListTasks(); err != nil || len(tasks) != 0 {
		t.Fatalf("confirmed deletion kept delivery %s: %+v %v", hash, tasks, err)
	}
}

func TestUIRemovalCommitBoundaryUpdatesCoreAndReportsErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		redownload bool
		committed  bool
	}{
		{name: "delete-cleanup", committed: true},
		{name: "redownload-cleanup", redownload: true, committed: true},
		{name: "delete-checkpoint"},
		{name: "redownload-checkpoint", redownload: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &removalTestEngine{}
			s, tt, _, _, updates := removalFixture(t, engine)
			if tc.committed {
				engine.cleanupErr = errors.Join(downloader.ErrRemoveCleanup, errors.New("held metadata file"))
			} else {
				engine.commitErr = errors.New("cannot save task checkpoint")
			}
			uiClick(t, tt, "删除")
			button := "确认删除下载任务"
			if tc.redownload {
				button = "确认删除并重新下载"
			}
			uiClick(t, tt, button)
			awaitRemovalAction(t, s, updates)
			if s.SaveError == "" {
				t.Fatal("removal failure was not shown")
			}
			tasks, err := s.Core.ListTasks()
			if err != nil {
				t.Fatal(err)
			}
			if !tc.committed {
				if len(tasks) != 1 || !tasks[0].Accepted || len(engine.Snapshot()) != 1 {
					t.Fatalf("uncommitted removal changed intent: tasks=%+v transfers=%+v", tasks, engine.Snapshot())
				}
				return
			}
			if len(engine.Snapshot()) != 0 {
				t.Fatal("committed cleanup failure kept engine task")
			}
			engine.cleanupErr = nil
			if err := s.Core.RetryPending(t.Context()); err != nil {
				t.Fatal(err)
			}
			tasks, err = s.Core.ListTasks()
			if err != nil {
				t.Fatal(err)
			}
			if tc.redownload {
				if len(tasks) != 1 || !tasks[0].Accepted || len(engine.Snapshot()) != 1 {
					t.Fatalf("cleanup error lost redownload: tasks=%+v transfers=%+v", tasks, engine.Snapshot())
				}
			} else if len(tasks) != 0 || len(engine.Snapshot()) != 0 {
				t.Fatalf("cleanup error resurrected deleted task: tasks=%+v transfers=%+v", tasks, engine.Snapshot())
			}
		})
	}
}

func TestUIRedownloadAfterUnsubscribeKeepsOriginalDestination(t *testing.T) {
	engine := &removalTestEngine{}
	s, tt, sub, hash, updates := removalFixture(t, engine)
	if err := s.Core.RemoveSubscription(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	uiClick(t, tt, "删除")
	uiClick(t, tt, "确认删除并重新下载")
	awaitRemovalAction(t, s, updates)
	if s.SaveError != "" || s.Notice != "已删除任务，将重新下载" {
		t.Fatalf("redownload result: notice=%q error=%q", s.Notice, s.SaveError)
	}
	if err := s.Core.RetryPending(t.Context()); err != nil {
		t.Fatal(err)
	}
	engine.mu.Lock()
	task, exists := engine.tasks[hash]
	calls := engine.calls
	engine.mu.Unlock()
	if !exists || calls != 2 || task.Directory != filepath.Join(sub.Directory, hash) {
		t.Fatalf("redownload after unsubscribe: exists=%v calls=%d task=%+v", exists, calls, task)
	}
}
