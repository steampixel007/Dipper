package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func onlyNewBackfill(s *State) {
	s.Backfill = true
	for id := range s.Checked {
		s.Checked[id] = id == "new"
	}
}

func TestUXBackfillPausedQueuesUntilManualResume(t *testing.T) {
	s, engine, updates, saved := submissionFixture(t, true)
	if err := s.Core.SetPaused(s.Ctx, saved.ID, true); err != nil {
		t.Fatal(err)
	}
	state, err := s.Core.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.project(state, engine.Snapshot())
	s.EditID = saved.ID
	onlyNewBackfill(s)

	tester := ui.NewTester(s.view, 1040, 720)
	texts := strings.Join(tester.Texts(), "\n")
	s.commit()
	finishSubmission(t, s, updates)

	subscription, ok := s.subscription(saved.ID)
	if !ok || !subscription.Paused {
		t.Fatalf("paused backfill changed the follow: %#v", subscription)
	}
	if s.Page != "downloads" || !strings.Contains(s.Notice, "已排队 1 个下载任务，恢复追番后自动开始。") {
		t.Fatalf("paused backfill was not described as queued: page=%q notice=%q", s.Page, s.Notice)
	}
	if s.SaveError != "" {
		t.Fatal(s.SaveError)
	}
	if slices.Contains(engine.submittedIDs(), "new") {
		t.Fatalf("paused backfill started downloading: %v", engine.submittedIDs())
	}
	state = submissionSnapshot(t, s)
	for _, sub := range state.Subscriptions {
		if sub.ID != saved.ID {
			continue
		}
		item, ok := sub.Items["new"]
		if !ok || item.Status != "pending" {
			t.Fatalf("paused backfill did not save a pending task: %#v", item)
		}
	}
	if !strings.Contains(texts, "追番已暂停；现在只会排队，恢复追番后开始下载。") {
		t.Fatalf("paused backfill footer does not explain waiting: %q", texts)
	}
	if err := s.Core.SetPaused(s.Ctx, saved.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.Core.RetryPending(s.Ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(engine.submittedIDs(), "new") {
		t.Fatalf("queued backfill did not start after manual resume: %v", engine.submittedIDs())
	}
}

func TestUXBackfillPausedActionQueuesUntilResume(t *testing.T) {
	s, engine, updates, saved := submissionFixture(t, true)
	if err := s.Core.SetPaused(s.Ctx, saved.ID, true); err != nil {
		t.Fatal(err)
	}
	state, err := s.Core.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.project(state, engine.Snapshot())
	s.EditID = saved.ID
	onlyNewBackfill(s)

	tester := ui.NewTester(s.view, 1040, 720)
	uiClick(t, tester, "下载选中资源")
	finishSubmission(t, s, updates)

	subscription, ok := s.subscription(saved.ID)
	if !ok || !subscription.Paused {
		t.Fatalf("paused backfill changed the follow: %#v", subscription)
	}
	if s.Page != "downloads" || !strings.Contains(s.Notice, "恢复追番后自动开始。") {
		t.Fatalf("paused backfill was not described as queued: page=%q notice=%q", s.Page, s.Notice)
	}
	if s.SaveError != "" {
		t.Fatal(s.SaveError)
	}
	if slices.Contains(engine.submittedIDs(), "new") {
		t.Fatalf("paused backfill started downloading: %v", engine.submittedIDs())
	}
}

func TestUXBackfillPartialDeliveryKeepsSavedTasks(t *testing.T) {
	s, engine, updates, saved := submissionFixture(t, true)
	s.EditID = saved.ID
	onlyNewBackfill(s)
	engine.reject = true

	tester := ui.NewTester(s.view, 1040, 720)
	uiClick(t, tester, "下载选中资源")
	finishSubmission(t, s, updates)

	if s.Page != "downloads" {
		t.Fatalf("saved backfill returned to %q instead of downloads", s.Page)
	}
	if strings.Contains(s.Notice, "提交失败") || !strings.Contains(s.SaveError, "已记录 1 个下载任务") {
		t.Fatalf("partial delivery misreported: notice=%q error=%q", s.Notice, s.SaveError)
	}
	engine.mu.Lock()
	engine.reject = false
	engine.mu.Unlock()
	engine.submitted = nil
	if err := s.Core.RetryPending(s.Ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(engine.submittedIDs(), "new") {
		t.Fatalf("saved backfill was not retried: %v", engine.submittedIDs())
	}
}

func TestUXBackfillDuplicateActivationSubmitsOnce(t *testing.T) {
	s, engine, updates, saved := submissionFixture(t, true)
	s.EditID = saved.ID
	onlyNewBackfill(s)
	engine.gate = make(chan struct{})

	tester := ui.NewTester(s.view, 1040, 720)
	button, ok := tester.Find("下载选中资源")
	if !ok {
		t.Fatalf("missing backfill action; texts=%q", tester.Texts())
	}
	uiClick(t, tester, "下载选中资源")
	tester.ClickAt(button.X+button.W/2, button.Y+button.H/2)
	tester.Advance(300 * time.Millisecond)
	close(engine.gate)
	finishSubmission(t, s, updates)

	ids := engine.submittedIDs()
	if len(ids) != 1 || ids[0] != "new" {
		t.Fatalf("duplicate activation delivered %v", ids)
	}
	if s.SaveError != "" || s.Page != "downloads" {
		t.Fatalf("duplicate activation finished incorrectly: page=%q error=%q", s.Page, s.SaveError)
	}
}

func TestUXBackfillEmptySelectionDoesNotSubmit(t *testing.T) {
	s, engine, _, saved := submissionFixture(t, true)
	s.EditID = saved.ID
	onlyNewBackfill(s)
	for id := range s.Checked {
		s.Checked[id] = false
	}

	tester := ui.NewTester(s.view, 1040, 720)
	uiRequireText(t, tester, "请选择要下载的已有资源", "下载选中资源")
	uiClick(t, tester, "下载选中资源")

	if s.Busy {
		t.Fatal("empty backfill entered the busy state")
	}
	if ids := engine.submittedIDs(); len(ids) != 0 {
		t.Fatalf("empty backfill submitted %v", ids)
	}
	state := submissionSnapshot(t, s)
	if len(state.Subscriptions) != 1 || len(state.Subscriptions[0].Items) != 3 {
		t.Fatalf("empty backfill changed saved tasks: %#v", state.Subscriptions)
	}
}

func TestUXBackfillBatchPartialFailureKeepsSuccessAndRetriesFailure(t *testing.T) {
	s, engine, updates, saved := submissionFixture(t, true)
	s.Backfill, s.EditID = true, saved.ID
	for id := range s.Checked {
		s.Checked[id] = id == "new" || id == "unknown"
	}
	s.History[5].Magnet = ""
	s.History[5].TorrentURL = "http://127.0.0.1:1/missing.torrent"

	tester := ui.NewTester(s.view, 1040, 720)
	uiClick(t, tester, "下载选中资源")
	finishSubmission(t, s, updates)

	state := submissionSnapshot(t, s)
	item, ok := state.Subscriptions[0].Items["unknown"]
	if !ok || item.Status != "pending" || item.LastError == "" {
		t.Fatalf("failed batch member is not retryable: %#v", item)
	}
	if !slices.Contains(engine.submittedIDs(), "new") {
		t.Fatalf("successful batch member was lost: %v", engine.submittedIDs())
	}
	if s.Page != "downloads" || !strings.Contains(s.SaveError, "已记录 2 个下载任务，部分还没开始") {
		t.Fatalf("partial batch was misreported: page=%q notice=%q error=%q", s.Page, s.Notice, s.SaveError)
	}
	before := len(engine.submittedIDs())
	if err := s.Core.RetryPending(s.Ctx); err == nil {
		t.Fatal("retrying an unreachable torrent unexpectedly succeeded")
	}
	if len(engine.submittedIDs()) != before {
		t.Fatalf("retry duplicated the successful task: before=%d after=%v", before, engine.submittedIDs())
	}
	state = submissionSnapshot(t, s)
	if item = state.Subscriptions[0].Items["unknown"]; item.Status != "pending" || item.LastError == "" {
		t.Fatalf("failed batch member was dropped after retry: %#v", item)
	}
}
