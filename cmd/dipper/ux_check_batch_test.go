package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
)

func TestUXBatchFailedChecksContinuesAndReportsPartialFailure(t *testing.T) {
	var seeded atomic.Bool
	var mu sync.Mutex
	calls := map[string]int{}
	kernel, err := core.Open(filepath.Join(t.TempDir(), "batch.db"), core.Options{
		DisableScheduler: true,
		Downloader:       &submissionTestEngine{snapshots: map[string]downloader.Snapshot{}},
		Feed: func(_ context.Context, _, group string) ([]core.Resource, error) {
			mu.Lock()
			calls[group]++
			mu.Unlock()
			if seeded.Load() && group == "batch-b" {
				return nil, nil
			}
			return nil, errors.New("network unavailable")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })

	ctx := context.Background()
	create := func(animeID, groupID string) core.Subscription {
		t.Helper()
		resource := core.Resource{ID: animeID + "-old", Title: "1080p", Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("ab", 20), Size: 100}
		sub, err := kernel.CreateSubscription(ctx, core.Anime{ID: animeID, Title: animeID}, core.Group{ID: groupID, Name: groupID}, core.Rule{}, []core.Resource{resource}, []string{resource.ID})
		if err != nil && sub.ID == "" {
			t.Fatalf("seed %s: %v", animeID, err)
		}
		return sub
	}
	a := create("追番甲", "batch-a")
	b := create("追番乙", "batch-b")
	c := create("追番丙", "batch-c")
	d := create("追番丁", "batch-d")
	for _, sub := range []core.Subscription{a, b, c, d} {
		_ = kernel.Check(ctx, sub.ID)
	}
	if err := kernel.SetPaused(ctx, d.ID, true); err != nil {
		t.Fatal(err)
	}

	seeded.Store(true)
	mu.Lock()
	calls = map[string]int{}
	mu.Unlock()
	s := uiFixture(t, "following")
	s.Core, s.Engine, s.Ctx = kernel, &downloader.Engine{}, ctx
	updates := make(chan func(), 8)
	s.Update = func(update func()) { updates <- update }
	state, err := kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.project(state, nil)

	tester := ui.NewTester(s.view, 1040, 720)
	uiClick(t, tester, "重试检查")
	if !s.Busy || !strings.Contains(s.Notice, "正在检查 3 个追番的更新") {
		t.Fatalf("batch did not start once: busy=%v notice=%q", s.Busy, s.Notice)
	}
	s.retryFailedChecks()
	finishSubmission(t, s, updates)

	mu.Lock()
	gotCalls := map[string]int{}
	for group, count := range calls {
		gotCalls[group] = count
	}
	mu.Unlock()
	for _, group := range []string{"batch-a", "batch-b", "batch-c"} {
		if gotCalls[group] != 1 {
			t.Fatalf("%s checked %d times, want once: %v", group, gotCalls[group], gotCalls)
		}
	}
	if gotCalls["batch-d"] != 0 {
		t.Fatalf("paused follow was checked: %v", gotCalls)
	}
	if !strings.Contains(s.Notice, "已检查 3 个追番，2 个仍需重试") || s.SaveError != "2 个追番检查失败，请打开对应剧集库查看原因并重试。" {
		t.Fatalf("partial result misreported: notice=%q error=%q", s.Notice, s.SaveError)
	}
	state, err = kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	subs := map[string]core.Subscription{}
	for _, sub := range state.Subscriptions {
		subs[sub.ID] = sub
	}
	if subs[a.ID].Failures != 2 || subs[c.ID].Failures != 2 {
		t.Fatalf("failed follow was skipped: a=%d c=%d", subs[a.ID].Failures, subs[c.ID].Failures)
	}
	if subs[b.ID].LastError != "" || subs[b.ID].Failures != 0 || subs[b.ID].LastCheck.IsZero() {
		t.Fatalf("healthy follow was not updated: %#v", subs[b.ID])
	}
	if !subs[d.ID].Paused || subs[d.ID].Failures != 1 {
		t.Fatalf("paused follow changed: %#v", subs[d.ID])
	}

	s.check(b.ID)
	if strings.Contains(s.Notice, "RSS") || s.Notice != "正在检查更新…" {
		t.Fatalf("single check uses jargon: %q", s.Notice)
	}
	finishSubmission(t, s, updates)
	if s.Notice != "检查更新完成" || strings.Contains(s.Notice, "RSS") {
		t.Fatalf("single check completion uses jargon: %q", s.Notice)
	}
}
