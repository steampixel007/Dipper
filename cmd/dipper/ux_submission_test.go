package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
	"github.com/steampixel007/Dipper/internal/source"
)

type submissionTestEngine struct {
	removalTestEngine
	submitted []core.DownloadTask
	snapshots map[string]downloader.Snapshot
	gate      chan struct{}
	reject    bool
}

func (e *submissionTestEngine) Ensure(task core.DownloadTask) error {
	e.mu.Lock()
	e.submitted = append(e.submitted, task)
	gate, reject := e.gate, e.reject
	e.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-task.Context.Done():
			return task.Context.Err()
		}
	}
	if reject {
		return errors.New("scripted disk failure")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tasks == nil {
		e.tasks = map[string]core.DownloadTask{}
	}
	e.tasks[task.Hash] = task
	return nil
}

func (e *submissionTestEngine) Snapshot() []downloader.Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	var snapshots []downloader.Snapshot
	for hash, task := range e.tasks {
		snapshot, ok := e.snapshots[hash]
		if !ok {
			snapshot = downloader.Snapshot{ID: hash, Folder: task.Directory, Progress: .43}
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func (e *submissionTestEngine) submittedIDs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var ids []string
	for _, task := range e.submitted {
		ids = append(ids, task.Resource.ID)
	}
	return ids
}

func submissionFixture(t *testing.T, seed bool) (*State, *submissionTestEngine, chan func(), core.Subscription) {
	t.Helper()
	engine := &submissionTestEngine{snapshots: map[string]downloader.Snapshot{}}
	root := t.TempDir()
	kernel, err := core.Open(filepath.Join(root, "submission.db"), core.Options{
		DownloadRoot: filepath.Join(root, "downloads"), Downloader: engine, DisableScheduler: true,
		Feed: func(context.Context, string, string) ([]core.Resource, error) {
			return []core.Resource{{ID: "future", Title: "[字幕组] 下一集 [1080p]", Magnet: fmt.Sprintf("magnet:?xt=urn:btih:%040x", 10)}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := kernel.Close(); err != nil {
			t.Error(err)
		}
	})
	s := uiFixture(t, "detail")
	s.Core, s.Engine, s.Ctx = kernel, engine, context.Background()
	s.AnimeID = "submission-anime"
	s.KnownAnime[s.AnimeID] = Anime{ID: s.AnimeID, Title: "提交测试"}
	s.Groups, s.GroupIDs, s.Group = []string{"新字幕组"}, []string{"submission-group"}, 0
	s.GroupCounts = []int{9}
	s.Rule = Rule{All: []string{"1080p"}}
	s.ResourceList, s.History = nil, nil
	s.Checked = map[string]bool{}
	for index, id := range []string{"active", "completed", "failed", "new", "new-alias", "unknown", "existing-alias", "excluded", "unchecked"} {
		hashIndex := index + 1
		if id == "new-alias" {
			hashIndex = 4
		}
		if id == "existing-alias" {
			hashIndex = 1
		}
		title := "[字幕组] 同名资源 [1080p]"
		if id == "excluded" {
			title = "[字幕组] 同名资源 [720p]"
		}
		size := int64(300 * 1024 * 1024)
		if id == "unknown" {
			size = 0
		}
		s.History = append(s.History, source.Resource{
			ID: id, Title: title, Magnet: fmt.Sprintf("magnet:?xt=urn:btih:%040x", hashIndex), Size: size,
		})
		s.ResourceList = append(s.ResourceList, Resource{ID: id, Title: title, MB: int(size / 1024 / 1024)})
		s.Checked[id] = id != "unchecked"
	}
	var saved core.Subscription
	if seed {
		saved, err = kernel.CreateSubscription(s.Ctx, core.Anime{ID: s.AnimeID, Title: "提交测试"},
			core.Group{ID: "existing-group", Name: "已有字幕组"}, core.Rule{All: []string{"1080p"}},
			coreResources(s.History[:3]), []string{"active", "completed", "failed"})
		if err != nil {
			t.Fatal(err)
		}
		for index, resource := range coreResources(s.History[:3]) {
			hash, err := core.ResourceHash(resource)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := downloader.Snapshot{ID: hash, Progress: .43}
			if index == 1 {
				snapshot.Progress = 1
			}
			if index == 2 {
				snapshot.Error = "scripted failure"
			}
			engine.snapshots[hash] = snapshot
		}
		engine.submitted = nil
	}
	state, err := kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.project(state, engine.Snapshot())
	updates := make(chan func(), 8)
	s.Update = func(update func()) { updates <- update }
	return s, engine, updates, saved
}

func finishSubmission(t *testing.T, s *State, updates chan func()) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for s.Busy {
		select {
		case update := <-updates:
			update()
		case <-deadline.C:
			t.Fatalf("submission did not finish: notice=%q error=%q", s.Notice, s.SaveError)
		}
	}
}

func submissionSnapshot(t *testing.T, s *State) core.State {
	t.Helper()
	state, err := s.Core.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestUXSubmissionMixedSelectionUsesOnlyNewTasks(t *testing.T) {
	for _, backfill := range []bool{false, true} {
		t.Run(fmt.Sprint(backfill), func(t *testing.T) {
			s, engine, updates, saved := submissionFixture(t, true)
			before := submissionSnapshot(t, s)
			delete(s.resourceHashes, "active")
			s.History[0].Magnet = ""
			if backfill {
				s.Backfill, s.EditID = true, saved.ID
				s.Groups, s.GroupIDs = []string{"已有字幕组"}, []string{"existing-group"}
				s.Rule = Rule{}
			}
			s.Checked["excluded"] = false
			if selected, _ := s.selected(); len(selected) != 7 {
				t.Fatalf("existing or duplicate checked rows disappeared from selection: %v", selected)
			}
			tt := ui.NewTester(s.view, 1320, 860)
			uiRequireText(t, tt, "本次下载 2 个资源，已知大小 300 MB，另有 1 条大小未知", "已有任务：未完成 1 个，已完成 1 个，失败 1 个")
			label := "下载并追番"
			if backfill {
				label = "下载选中资源"
			}
			uiClick(t, tt, label)
			if !s.Busy {
				t.Fatalf("direct click did not start once: busy=%v", s.Busy)
			}
			finishSubmission(t, s, updates)
			if got := engine.submittedIDs(); !slices.Equal(got, []string{"new", "unknown"}) {
				t.Fatalf("submitted IDs = %v, want only new and unknown", got)
			}
			after := submissionSnapshot(t, s)
			if s.SaveError != "" {
				t.Fatal(s.SaveError)
			}
			for _, sub := range after.Subscriptions {
				if sub.ID == saved.ID {
					if backfill {
						if !reflect.DeepEqual(sub.Rule, saved.Rule) {
							t.Fatalf("backfill changed saved rule: %+v", sub.Rule)
						}
						for _, id := range []string{"active", "completed", "failed"} {
							if !reflect.DeepEqual(sub.Items[id], before.Subscriptions[0].Items[id]) {
								t.Fatalf("backfill rewrote existing item %s", id)
							}
						}
					}
					continue
				}
				for id, item := range sub.Items {
					if item.Explicit != (id == "new" || id == "unknown") {
						t.Fatalf("submitted selection disagrees for %s: %+v", id, item)
					}
				}
			}
			if backfill {
				if s.Page != "downloads" || !strings.Contains(s.Notice, "已开始下载 2 个资源") {
					t.Fatalf("backfill feedback: page=%s notice=%q", s.Page, s.Notice)
				}
			} else if len(after.Subscriptions) != 2 || s.Page != "following" {
				t.Fatalf("follow did not save and navigate: subs=%d page=%s", len(after.Subscriptions), s.Page)
			}
		})
	}
}

func TestUXSubmissionDirectClickGuardsOneOperation(t *testing.T) {
	s, engine, updates, _ := submissionFixture(t, false)
	for id := range s.Checked {
		s.Checked[id] = id == "new"
	}
	gate := make(chan struct{})
	engine.gate = gate
	released := false
	t.Cleanup(func() {
		if !released {
			close(gate)
		}
	})
	s.Suggestions = true
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "下载并追番")
	if !s.Busy || s.Suggestions || s.WordOpen {
		t.Fatalf("wrong direct pending state: busy=%v suggestions=%v word=%v", s.Busy, s.Suggestions, s.WordOpen)
	}
	uiClick(t, tt, "正在提交…")
	s.commit()
	close(gate)
	released = true
	finishSubmission(t, s, updates)
	if got := engine.submittedIDs(); !slices.Equal(got, []string{"new"}) {
		t.Fatalf("repeat activation submitted %v", got)
	}
	if state := submissionSnapshot(t, s); len(state.Subscriptions) != 1 || len(state.Tasks) != 1 {
		t.Fatalf("repeat activation created %d subscriptions and %d tasks", len(state.Subscriptions), len(state.Tasks))
	}
	if s.Page != "following" || s.SaveError != "" || !strings.Contains(s.Notice, "已保存追番") {
		t.Fatalf("missing success feedback: page=%s notice=%q error=%q", s.Page, s.Notice, s.SaveError)
	}
}

func TestUXSubmissionExistingFailuresHaveRecovery(t *testing.T) {
	s, engine, _, saved := submissionFixture(t, true)
	s.Backfill, s.EditID = true, saved.ID
	for id := range s.Checked {
		s.Checked[id] = id == "active" || id == "completed" || id == "failed" || id == "existing-alias"
	}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "所选资源已有下载任务", "已有任务：未完成 1 个，已完成 1 个，失败 1 个")
	uiClick(t, tt, "下载选中资源")
	if s.Busy || len(engine.submittedIDs()) != 0 {
		t.Fatal("existing-only backfill submitted")
	}
	uiClick(t, tt, "查看失败任务并重试")
	if s.Page != "downloads" || s.DownloadFilter != downloadsFailed {
		t.Fatalf("recovery did not open failed downloads: page=%s filter=%d", s.Page, s.DownloadFilter)
	}
	if got := engine.submittedIDs(); len(got) != 0 {
		t.Fatalf("recovery navigation implicitly retried %v", got)
	}
	uiRequireText(t, tt, "scripted failure")
}

func TestUXSubmissionFutureOnlyAndRuleIsolation(t *testing.T) {
	for _, editing := range []bool{false, true} {
		t.Run(fmt.Sprint(editing), func(t *testing.T) {
			s, engine, updates, saved := submissionFixture(t, editing)
			before := submissionSnapshot(t, s)
			label := "开启自动追番"
			if editing {
				s.Editing, s.EditID = true, saved.ID
				s.Rule = Rule{None: []string{"720p"}}
				label = "保存追番条件"
			} else {
				s.Checked = map[string]bool{}
			}
			tt := ui.NewTester(s.view, 1320, 860)
			uiClick(t, tt, label)
			if !s.Busy {
				t.Fatal("future-only save did not directly start")
			}
			finishSubmission(t, s, updates)
			if len(engine.submittedIDs()) != 0 {
				t.Fatal("future-only save submitted current resources")
			}
			after := submissionSnapshot(t, s)
			if len(after.Subscriptions) != 1 || !reflect.DeepEqual(after.Subscriptions[0].Rule, coreRule(s.Rule)) {
				t.Fatalf("future rule not saved: %+v", after.Subscriptions)
			}
			if editing && !reflect.DeepEqual(after.Subscriptions[0].Items, before.Subscriptions[0].Items) {
				t.Fatal("rule edit rewrote existing items")
			}
			if !editing {
				for id, item := range after.Subscriptions[0].Items {
					if item.Explicit || !item.Read {
						t.Fatalf("future-only follow changed current selection %s: %+v", id, item)
					}
				}
			}
			for range 2 {
				if err := s.Core.Check(s.Ctx, after.Subscriptions[0].ID); err != nil {
					t.Fatal(err)
				}
			}
			if !slices.Equal(engine.submittedIDs(), []string{"future"}) {
				t.Fatalf("saved rule did not automatically follow a new release once: %v", engine.submittedIDs())
			}
		})
	}
}

func TestUXSubmissionExistingOnlyStillFollowsFutureResources(t *testing.T) {
	s, engine, updates, saved := submissionFixture(t, true)
	for id := range s.Checked {
		s.Checked[id] = id == "active" || id == "completed" || id == "failed" || id == "existing-alias"
	}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "本次不下载，只追后续更新", "已有任务：未完成 1 个，已完成 1 个，失败 1 个")
	uiClick(t, tt, "开启自动追番")
	if !s.Busy {
		t.Fatal("existing-only follow did not start directly")
	}
	finishSubmission(t, s, updates)
	after := submissionSnapshot(t, s)
	if len(after.Subscriptions) != 2 || len(engine.submittedIDs()) != 0 {
		t.Fatalf("existing-only follow repeated current downloads: subs=%d ids=%v", len(after.Subscriptions), engine.submittedIDs())
	}
	for _, sub := range after.Subscriptions {
		if sub.ID == saved.ID {
			continue
		}
		for id, item := range sub.Items {
			if item.Explicit {
				t.Fatalf("existing task %s was submitted again", id)
			}
		}
		if err := s.Core.Check(s.Ctx, sub.ID); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(engine.submittedIDs(), []string{"future"}) {
		t.Fatalf("existing-only follow did not admit future release: %v", engine.submittedIDs())
	}
}

func TestUXSubmissionErrorKeepsSelectionForRetry(t *testing.T) {
	s, engine, updates, saved := submissionFixture(t, true)
	s.Backfill, s.EditID = true, "missing-subscription"
	for id := range s.Checked {
		s.Checked[id] = id == "new"
	}
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "下载选中资源")
	if !s.Busy {
		t.Fatal("error scenario did not start directly")
	}
	finishSubmission(t, s, updates)
	if !strings.Contains(s.SaveError, "下载提交失败") || s.Notice != "" || s.Page != "detail" || !s.Checked["new"] {
		t.Fatalf("failed submit lost actionable state: page=%s notice=%q error=%q checked=%v", s.Page, s.Notice, s.SaveError, s.Checked)
	}
	if len(engine.submittedIDs()) != 0 {
		t.Fatal("failed submit admitted a task")
	}
	s.EditID = saved.ID
	tt.Frame()
	uiClick(t, tt, "下载选中资源")
	finishSubmission(t, s, updates)
	if s.SaveError != "" || !slices.Equal(engine.submittedIDs(), []string{"new"}) {
		t.Fatalf("one-click retry failed: error=%q submitted=%v", s.SaveError, engine.submittedIDs())
	}
}

func TestUXSubmissionPartialFailureReportsSavedFollow(t *testing.T) {
	s, engine, updates, _ := submissionFixture(t, false)
	for id := range s.Checked {
		s.Checked[id] = id == "new"
	}
	engine.reject = true
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "下载并追番")
	if !s.Busy {
		t.Fatal("partial failure scenario did not start directly")
	}
	finishSubmission(t, s, updates)
	if s.Page != "following" || !strings.Contains(s.SaveError, "追番已保存，部分下载没有开始") {
		t.Fatalf("partial failure hid saved follow: page=%s error=%q", s.Page, s.SaveError)
	}
	state := submissionSnapshot(t, s)
	if len(state.Subscriptions) != 1 || !slices.Equal(engine.submittedIDs(), []string{"new"}) {
		t.Fatalf("partial failure lost or repeated operation: subs=%d ids=%v", len(state.Subscriptions), engine.submittedIDs())
	}
}
