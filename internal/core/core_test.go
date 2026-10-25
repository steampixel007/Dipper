package core

import (
	"context"
	"crypto/sha1"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeDownloader struct {
	mu       sync.Mutex
	calls    []DownloadTask
	accepted map[string]DownloadTask
	fail     bool
	hook     func(DownloadTask) error
}

func (d *fakeDownloader) Ensure(task DownloadTask) error {
	d.mu.Lock()
	d.calls = append(d.calls, task)
	fail, hook := d.fail, d.hook
	d.mu.Unlock()
	if hook != nil {
		if err := hook(task); err != nil {
			return err
		}
	}
	if fail {
		return errors.New("disk is full")
	}
	if err := task.Context.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.accepted == nil {
		d.accepted = map[string]DownloadTask{}
	}
	if _, exists := d.accepted[task.Hash]; !exists {
		d.accepted[task.Hash] = task
	}
	return nil
}

func (d *fakeDownloader) count() int                         { d.mu.Lock(); defer d.mu.Unlock(); return len(d.calls) }
func (d *fakeDownloader) setFail(v bool)                     { d.mu.Lock(); d.fail = v; d.mu.Unlock() }
func (d *fakeDownloader) setHook(h func(DownloadTask) error) { d.mu.Lock(); d.hook = h; d.mu.Unlock() }

func resource(id string, n int, title string) Resource {
	return Resource{ID: id, Title: title, Magnet: fmt.Sprintf("magnet:?xt=urn:btih:%040x", n), Size: 100}
}

var anime = Anime{ID: "anime", Title: "测试番剧"}
var group = Group{ID: "group", Name: "字幕组"}

func openTest(t *testing.T, path string, opts Options) *Core {
	t.Helper()
	if opts.DownloadRoot == "" {
		opts.DownloadRoot = filepath.Join(filepath.Dir(path), "downloads")
	}
	opts.DisableScheduler = true
	c, err := Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	return c
}

func create(t *testing.T, c *Core, a Anime, g Group, rule Rule, history []Resource, ids []string) Subscription {
	t.Helper()
	sub, err := c.CreateSubscription(context.Background(), a, g, rule, history, ids)
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

func current(t *testing.T, c *Core, id string) Subscription {
	t.Helper()
	subs, err := c.Subscriptions()
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range subs {
		if sub.ID == id {
			return sub
		}
	}
	t.Fatalf("missing subscription %q", id)
	return Subscription{}
}

func waitFor(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition did not become true")
}

func TestRulesAndSuggestions(t *testing.T) {
	cases := []struct {
		title string
		rule  Rule
		want  bool
	}{
		{"ｘ２６５ １０８０Ｐ 简体", Rule{All: []string{"X265", "1080p"}, None: []string{"合集"}, Any: [][]string{{"CHS", "简体"}, {"X265", "HEVC"}}}, true},
		{"1080p 繁体", Rule{All: []string{"1080p", "简体"}}, false},
		{"1080p 合集", Rule{None: []string{"合集"}}, false},
		{"未知标题", Rule{None: []string{"合集"}}, true},
		{"未知标题", Rule{}, true},
		{"CHS HEVC", Rule{Any: [][]string{{"CHS", "简体"}, {"x264", "AVC"}}}, false},
		{"繁體", Rule{All: []string{"繁体"}}, false},
		{"？", Rule{All: []string{"?"}}, true},
		{"anything", Rule{Any: [][]string{{" ", ""}}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			got := Match(tc.title, tc.rule)
			if got.Matches != tc.want || got.Reason == "" {
				t.Fatalf("%+v", got)
			}
		})
	}
	if got := ParseEpisode("[组] 剧名 - ０３.５ [1080p]"); got != "3.5" {
		t.Fatal(got)
	}
	if got := ParseEpisode("[组] 剧名 第12話 [1080p]"); got != "12" {
		t.Fatal(got)
	}
	if got := ParseEpisode("[组] 剧名 [1080p][2026]"); got != "" {
		t.Fatal(got)
	}
	words := SuggestWords("[组] 剧名 - 03 [1080P][HEVC][简日内嵌][MKV]")
	if !reflect.DeepEqual(words, []string{"1080p", "hevc", "简日内嵌", "mkv"}) {
		t.Fatal(words)
	}
}

func TestHTTPFeedGlobalHashDedupAndDifferentReleases(t *testing.T) {
	a := resource("source-a", 1, "Show - 01 1080p")
	alias := resource("different-source-id", 1, "Show - 01 1080p")
	b := resource("source-b", 2, "Show - 01 1080p")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		json.NewEncoder(w).Encode([]Resource{a, alias, b})
	}))
	defer server.Close()
	feed := func(ctx context.Context, _, _ string) ([]Resource, error) {
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var resources []Resource
		err = json.NewDecoder(resp.Body).Decode(&resources)
		return resources, err
	}
	d := &fakeDownloader{}
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d, Feed: feed})
	sub := create(t, c, anime, group, Rule{}, nil, nil)
	if err := c.Check(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.Check(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	create(t, c, Anime{ID: "second", Title: "Second"}, Group{ID: "second-group"}, Rule{}, []Resource{alias}, []string{alias.ID})
	if d.count() != 2 || requests.Load() != 2 {
		t.Fatalf("calls %d requests %d", d.count(), requests.Load())
	}
	snap, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tasks) != 2 {
		t.Fatalf("tasks %+v", snap.Tasks)
	}
	for _, item := range current(t, c, sub.ID).Items {
		if !item.Read || item.Status != ItemDownloaded {
			t.Fatal(item)
		}
	}
}

func TestDeselectedAndExcludedSurviveRestartWithoutRetroactiveRule(t *testing.T) {
	a, b, x := resource("picked", 1, "1080p"), resource("skipped", 2, "1080p"), resource("excluded", 3, "720p")
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	d := &fakeDownloader{}
	opts := Options{Downloader: d, Feed: func(context.Context, string, string) ([]Resource, error) { return []Resource{a, b, x}, nil }}
	c := openTest(t, path, opts)
	sub := create(t, c, anime, group, Rule{All: []string{"1080p"}}, []Resource{a, b, x}, []string{a.ID})
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	d2 := &fakeDownloader{}
	opts.Downloader = d2
	c = openTest(t, path, opts)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return d2.count() == 1 })
	if err := c.UpdateRule(context.Background(), sub.ID, Rule{}); err != nil {
		t.Fatal(err)
	}
	if err := c.Check(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	sub = current(t, c, sub.ID)
	if sub.Items[b.ID].Status != ItemSkipped || !sub.Items[b.ID].Read || sub.Items[x.ID].Status != ItemExcluded {
		t.Fatalf("%+v", sub.Items)
	}
	if d2.count() != 1 {
		t.Fatalf("restart should replay only accepted task, got %d", d2.count())
	}
}

func TestDurableOutboxFailureRetriesEvenWithEmptyFeed(t *testing.T) {
	a := resource("a", 1, "1080p")
	d := &fakeDownloader{fail: true}
	path := filepath.Join(t.TempDir(), "state.db")
	c := openTest(t, path, Options{Downloader: d})
	sub, err := c.CreateSubscription(context.Background(), anime, group, Rule{}, []Resource{a}, []string{a.ID})
	if err == nil || sub.ID == "" {
		t.Fatal("creation must return committed subscription and submission error")
	}
	sub = current(t, c, sub.ID)
	if sub.Items[a.ID].Read || sub.Items[a.ID].Status != ItemPending {
		t.Fatal(sub.Items)
	}
	tasks, _ := c.ListTasks()
	if len(tasks) != 1 || tasks[0].Accepted || tasks[0].LastError == "" || tasks[0].NextRetry.IsZero() {
		t.Fatal(tasks)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	d2 := &fakeDownloader{}
	c = openTest(t, path, Options{Downloader: d2, Feed: func(context.Context, string, string) ([]Resource, error) { return nil, nil }})
	if err := c.Check(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	if d2.count() != 1 || !current(t, c, sub.ID).Items[a.ID].Read {
		t.Fatal("saved outbox not delivered")
	}
	if err := c.Check(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	if d2.count() != 1 {
		t.Fatal("accepted intent submitted twice")
	}
}

func TestPauseDoesNotFetchOrReadAndResumeChecksCurrentRule(t *testing.T) {
	var calls atomic.Int32
	a, b := resource("a", 1, "1080p"), resource("b", 2, "720p")
	d := &fakeDownloader{}
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d, Feed: func(context.Context, string, string) ([]Resource, error) { calls.Add(1); return []Resource{a, b}, nil }})
	sub := create(t, c, anime, group, Rule{}, nil, nil)
	if err := c.SetPaused(context.Background(), sub.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := c.Check(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || len(current(t, c, sub.ID).Items) != 0 {
		t.Fatal("paused check touched feed")
	}
	if err := c.UpdateRule(context.Background(), sub.ID, Rule{All: []string{"1080p"}}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetPaused(context.Background(), sub.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := c.Check(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	sub = current(t, c, sub.ID)
	if calls.Load() != 1 || d.count() != 1 || sub.Items[b.ID].Status != ItemExcluded || !sub.Items[b.ID].Read {
		t.Fatal(sub)
	}
}

func TestInFlightFeedRevisionDiscardsOldResult(t *testing.T) {
	for _, action := range []string{"pause", "rule", "pause-resume"} {
		t.Run(action, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			d := &fakeDownloader{}
			c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d, Feed: func(context.Context, string, string) ([]Resource, error) {
				close(entered)
				<-release
				return []Resource{resource("a", 1, "1080p")}, nil
			}})
			sub := create(t, c, anime, group, Rule{}, nil, nil)
			result := make(chan error, 1)
			go func() { result <- c.Check(context.Background(), sub.ID) }()
			<-entered
			if action == "rule" {
				if err := c.UpdateRule(context.Background(), sub.ID, Rule{All: []string{"720p"}}); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := c.SetPaused(context.Background(), sub.ID, true); err != nil {
					t.Fatal(err)
				}
				if action == "pause-resume" {
					if err := c.SetPaused(context.Background(), sub.ID, false); err != nil {
						t.Fatal(err)
					}
				}
			}
			close(release)
			if err := <-result; err == nil {
				t.Fatal("stale check succeeded")
			}
			if len(current(t, c, sub.ID).Items) != 0 || d.count() != 0 {
				t.Fatal("stale result was committed")
			}
		})
	}
}

func TestPauseDuringEnsureCancelsAndLeavesUnreadOutbox(t *testing.T) {
	entered := make(chan struct{})
	d := &fakeDownloader{}
	d.setHook(func(task DownloadTask) error { close(entered); <-task.Context.Done(); return task.Context.Err() })
	a := resource("a", 1, "1080p")
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d, Feed: func(context.Context, string, string) ([]Resource, error) { return []Resource{a}, nil }})
	sub := create(t, c, anime, group, Rule{}, nil, nil)
	result := make(chan error, 1)
	go func() { result <- c.Check(context.Background(), sub.ID) }()
	<-entered
	if err := c.SetPaused(context.Background(), sub.ID, true); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled ensure succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("pause failed to cancel Ensure")
	}
	sub = current(t, c, sub.ID)
	if sub.Items[a.ID].Read || sub.Items[a.ID].Status != ItemPending {
		t.Fatal(sub.Items)
	}
	tasks, _ := c.ListTasks()
	if len(tasks) != 1 || tasks[0].Accepted {
		t.Fatal(tasks)
	}
	d.setHook(nil)
	if err := c.SetPaused(context.Background(), sub.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := c.RetryPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !current(t, c, sub.ID).Items[a.ID].Read {
		t.Fatal("pending task was lost")
	}
}

func TestPauseDuringIndependentRetryAndCrossSubscriptionAcceptance(t *testing.T) {
	a := resource("a", 1, "1080p")
	d := &fakeDownloader{fail: true}
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d})
	sub, err := c.CreateSubscription(context.Background(), anime, group, Rule{}, []Resource{a}, []string{a.ID})
	if err == nil {
		t.Fatal("expected initial failure")
	}
	d.setFail(false)
	entered := make(chan struct{})
	d.setHook(func(task DownloadTask) error { close(entered); <-task.Context.Done(); return task.Context.Err() })
	result := make(chan error, 1)
	go func() { result <- c.RetryPending(context.Background()) }()
	<-entered
	if err := c.SetPaused(context.Background(), sub.ID, true); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("retry ignored pause")
		}
	case <-time.After(time.Second):
		t.Fatal("pending admission not cancelled")
	}
	d.setHook(nil)
	create(t, c, anime, Group{ID: "other"}, Rule{}, []Resource{a}, []string{a.ID})
	if current(t, c, sub.ID).Items[a.ID].Read {
		t.Fatal("other subscription marked paused subscription read")
	}
	if err := c.SetPaused(context.Background(), sub.ID, false); err != nil {
		t.Fatal(err)
	}
	before := d.count()
	if err := c.RetryPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !current(t, c, sub.ID).Items[a.ID].Read || d.count() != before {
		t.Fatal("accepted hash was not reconciled without resubmission")
	}
}

func TestRestartReplaysAcceptedTransfersOfPausedSubscriptions(t *testing.T) {
	a := resource("a", 1, "1080p")
	path := filepath.Join(t.TempDir(), "state.db")
	d := &fakeDownloader{}
	c := openTest(t, path, Options{Downloader: d})
	sub := create(t, c, anime, group, Rule{}, []Resource{a}, []string{a.ID})
	if err := c.SetPaused(context.Background(), sub.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	d2 := &fakeDownloader{}
	c = openTest(t, path, Options{Downloader: d2})
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return d2.count() == 1 })
	if !current(t, c, sub.ID).Paused {
		t.Fatal("restoration changed subscription pause")
	}
	tasks, _ := c.ListTasks()
	if len(tasks) != 1 || tasks[0].Task.Directory != filepath.Join(sub.Directory, tasks[0].Task.Hash) {
		t.Fatal(tasks)
	}
}

func TestRemoveSubscriptionKeepsAcceptedTasksAndDropsPendingOnes(t *testing.T) {
	accepted := resource("a", 1, "1080p")
	pending := resource("b", 2, "720p")
	other := resource("c", 3, "国语")
	pendingHash, _ := ResourceHash(pending)
	d := &fakeDownloader{}
	d.setHook(func(task DownloadTask) error {
		if task.Hash == pendingHash {
			return errors.New("submission refused")
		}
		return nil
	})
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d})
	sub, err := c.CreateSubscription(context.Background(), anime, group, Rule{}, []Resource{accepted, pending}, []string{accepted.ID, pending.ID})
	if err != nil && sub.ID == "" {
		t.Fatalf("creation failed before commit: %v", err)
	}
	d.setHook(nil)
	otherSub := create(t, c, anime, Group{ID: "other"}, Rule{}, []Resource{other}, []string{other.ID})
	if err := c.RemoveSubscription(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	subs, _ := c.Subscriptions()
	if len(subs) != 1 || subs[0].ID != otherSub.ID {
		t.Fatal(subs)
	}
	found := false
	animes, err := c.Anime()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range animes {
		if a.Anime.ID == anime.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("removal dropped the anime directory record")
	}
	tasks, _ := c.ListTasks()
	if len(tasks) != 2 {
		t.Fatal(tasks)
	}
	for _, r := range tasks {
		if r.Task.Hash == pendingHash {
			t.Fatal("pending task survived removal")
		}
	}
	if err := c.RemoveSubscription(context.Background(), sub.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removing a gone subscription: %v", err)
	}
}

func TestRemoveSubscriptionCancelsInFlightSubmissionAndDropsItsTask(t *testing.T) {
	a := resource("a", 1, "1080p")
	d := &fakeDownloader{}
	entered := make(chan struct{})
	d.setHook(func(task DownloadTask) error { close(entered); <-task.Context.Done(); return task.Context.Err() })
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d, Feed: func(context.Context, string, string) ([]Resource, error) { return []Resource{a}, nil }})
	sub := create(t, c, anime, group, Rule{}, nil, nil)
	result := make(chan error, 1)
	go func() { result <- c.Check(context.Background(), sub.ID) }()
	<-entered
	if err := c.RemoveSubscription(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled ensure succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("removal failed to cancel Ensure")
	}
	subs, _ := c.Subscriptions()
	if len(subs) != 0 {
		t.Fatal(subs)
	}
	tasks, _ := c.ListTasks()
	if len(tasks) != 0 {
		t.Fatal(tasks)
	}
	d.setHook(nil)
	if err := c.RetryPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.count() != 1 {
		t.Fatal("removed subscription's pending task was resubmitted")
	}
}

func TestDirectoriesAreFixedAndPreviewMatchesCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	d := &fakeDownloader{}
	c := openTest(t, path, Options{Downloader: d})
	preview, err := c.PreviewDirectory(anime)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := c.Anime()
	if len(before) != 0 {
		t.Fatal("preview persisted anime")
	}
	sub := create(t, c, anime, group, Rule{}, nil, nil)
	if sub.Directory != preview {
		t.Fatalf("preview %q actual %q", preview, sub.Directory)
	}
	settings, _ := c.GetSettings()
	rootA := settings.DownloadRoot
	settings.DownloadRoot = filepath.Join(filepath.Dir(path), "other-root")
	settings.UploadLimitBytes = 42
	if err := c.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	sub2 := create(t, c, Anime{ID: anime.ID, Title: "renamed anime"}, Group{ID: "other"}, Rule{}, nil, nil)
	if sub2.Directory != sub.Directory {
		t.Fatal("same anime moved to new root/title")
	}
	other := create(t, c, Anime{ID: "new", Title: anime.Title}, group, Rule{}, nil, nil)
	if !strings.HasPrefix(other.Directory, settings.DownloadRoot) {
		t.Fatal(other.Directory)
	}
	collisionAnime := Anime{ID: "collision", Title: anime.Title}
	preview, _ = c.PreviewDirectory(collisionAnime)
	collision := create(t, c, collisionAnime, group, Rule{}, nil, nil)
	if collision.Directory == other.Directory || collision.Directory != preview {
		t.Fatal("same title collision")
	}
	a := resource("a", 1, "1080p")
	if err := c.DownloadExisting(context.Background(), sub.ID, []Resource{a}, []string{a.ID}); err != nil {
		t.Fatal(err)
	}
	tasks, _ := c.ListTasks()
	if len(tasks) != 1 || !strings.HasPrefix(tasks[0].Task.Directory, rootA) || filepath.Base(tasks[0].Task.Directory) != tasks[0].Task.Hash {
		t.Fatal(tasks)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	c = openTest(t, path, Options{DownloadRoot: "ignored"})
	got, _ := c.GetSettings()
	if got.DownloadRoot != settings.DownloadRoot || got.UploadLimitBytes != 42 {
		t.Fatal(got)
	}
	if current(t, c, sub.ID).Directory != sub.Directory {
		t.Fatal("directory changed on restart")
	}
	for _, name := range []string{"CON", "aux.txt", "LPT1", "../a:b?*", "...", "a/b\\c"} {
		clean := safeName(name)
		if clean == "" || strings.ContainsAny(clean, `<>:"/\|?*`) {
			t.Fatal(clean)
		}
	}
}

func TestManualDownloadOverridesOnlyExplicitHistoryItems(t *testing.T) {
	a, b, x := resource("a", 1, "1080p"), resource("b", 2, "1080p"), resource("x", 3, "720p")
	d := &fakeDownloader{}
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d})
	rule := Rule{All: []string{"1080p"}}
	sub := create(t, c, anime, group, rule, []Resource{a, b, x}, nil)
	if err := c.DownloadExisting(context.Background(), sub.ID, []Resource{a, b, x}, []string{"unknown"}); err == nil {
		t.Fatal("unknown selection accepted")
	}
	if len(current(t, c, sub.ID).Items) != 3 || d.count() != 0 {
		t.Fatal("invalid selection mutated state")
	}
	d.setFail(true)
	if err := c.DownloadExisting(context.Background(), sub.ID, []Resource{a, b, x}, []string{a.ID, x.ID}); err == nil {
		t.Fatal("expected failure")
	}
	sub = current(t, c, sub.ID)
	if sub.Items[a.ID].Read || sub.Items[x.ID].Read || sub.Items[b.ID].Status != ItemSkipped || !reflect.DeepEqual(sub.Rule, NormalizeRule(rule)) {
		t.Fatal(sub)
	}
	d.setFail(false)
	if err := c.RetryPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.DownloadExisting(context.Background(), sub.ID, []Resource{a, b, x}, []string{a.ID, x.ID}); err != nil {
		t.Fatal(err)
	}
	if d.count() != 4 {
		t.Fatalf("duplicate manual selection was resubmitted: %d", d.count())
	}
}

func TestHashUsesConfirmedMagnetOrTorrentInfoOnly(t *testing.T) {
	fakeID := strings.Repeat("a", 40)
	if _, err := ResourceHash(Resource{ID: fakeID, TorrentURL: "https://example.test/" + fakeID + ".torrent"}); err == nil {
		t.Fatal("source identity mistaken for info hash")
	}
	rawHash, _ := hex.DecodeString(strings.Repeat("01", 20))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(rawHash)
	got, err := ResourceHash(Resource{Magnet: "magnet:?xt=urn:btih:" + strings.ToLower(encoded)})
	if err != nil || got != hex.EncodeToString(rawHash) {
		t.Fatal(got, err)
	}
	info := []byte("d6:lengthi1e4:name5:a.mkve")
	torrent := append([]byte("d4:info"), info...)
	torrent = append(torrent, 'e')
	h := sha1.Sum(info)
	expected := hex.EncodeToString(h[:])
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write(torrent) }))
	defer server.Close()
	r := Resource{ID: fakeID, Title: "opaque", TorrentURL: server.URL + "/" + fakeID + ".torrent"}
	d := &fakeDownloader{}
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d})
	create(t, c, anime, group, Rule{}, []Resource{r}, []string{r.ID})
	tasks, _ := c.ListTasks()
	if requests.Load() != 1 || len(tasks) != 1 || tasks[0].Task.Hash != expected || tasks[0].Task.Resource.Magnet != "magnet:?xt=urn:btih:"+expected {
		t.Fatal(tasks, requests.Load())
	}
}

func TestUnresolvableTorrentIntentSurvivesRestart(t *testing.T) {
	var ready atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			http.Error(w, "offline", 503)
			return
		}
		w.Write([]byte("d4:infod6:lengthi1e4:name5:a.mkvee"))
	}))
	defer server.Close()
	r := Resource{ID: "opaque", Title: "1080p", TorrentURL: server.URL + "/opaque.torrent"}
	path := filepath.Join(t.TempDir(), "state.db")
	d := &fakeDownloader{}
	c := openTest(t, path, Options{Downloader: d})
	sub, err := c.CreateSubscription(context.Background(), anime, group, Rule{}, []Resource{r}, []string{r.ID})
	if err == nil {
		t.Fatal("failed resolution should report error")
	}
	if current(t, c, sub.ID).Items[r.ID].Read {
		t.Fatal("unresolved intent read")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	ready.Store(true)
	c = openTest(t, path, Options{Downloader: d})
	if err := c.RetryPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.count() != 1 || !current(t, c, sub.ID).Items[r.ID].Read {
		t.Fatal("unresolved intent lost")
	}
}

func TestCloseCancelsAndWaitsForFetchAndEnsure(t *testing.T) {
	for _, stage := range []string{"fetch", "ensure"} {
		t.Run(stage, func(t *testing.T) {
			entered, exited := make(chan struct{}), make(chan struct{})
			d := &fakeDownloader{}
			feed := func(ctx context.Context, _, _ string) ([]Resource, error) {
				close(entered)
				<-ctx.Done()
				close(exited)
				return nil, ctx.Err()
			}
			if stage == "ensure" {
				feed = func(context.Context, string, string) ([]Resource, error) {
					return []Resource{resource("a", 1, "1080p")}, nil
				}
				d.setHook(func(task DownloadTask) error {
					close(entered)
					<-task.Context.Done()
					close(exited)
					return task.Context.Err()
				})
			}
			c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: d, Feed: feed})
			sub := create(t, c, anime, group, Rule{}, nil, nil)
			result := make(chan error, 1)
			go func() { result <- c.Check(context.Background(), sub.ID) }()
			<-entered
			closed := make(chan error, 1)
			go func() { closed <- c.Close() }()
			select {
			case err := <-closed:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("Close did not cancel operation")
			}
			select {
			case <-exited:
			default:
				t.Fatal("Close returned before operation exited")
			}
			if err := <-result; err == nil {
				t.Fatal("cancelled operation succeeded")
			}
			if _, err := c.Snapshot(); !errors.Is(err, ErrClosed) {
				t.Fatal(err)
			}
		})
	}
}

func TestBackoffAndDefaultFifteenMinuteInterval(t *testing.T) {
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Now: func() time.Time { return now }, Feed: func(context.Context, string, string) ([]Resource, error) {
		return nil, errors.New("source unavailable")
	}})
	sub := create(t, c, anime, group, Rule{}, nil, nil)
	if sub.NextCheck.Sub(now) != 15*time.Minute {
		t.Fatal(sub.NextCheck)
	}
	for n := 1; n <= 3; n++ {
		if err := c.Check(context.Background(), sub.ID); err == nil {
			t.Fatal("failed source hidden")
		}
		sub = current(t, c, sub.ID)
		if sub.Failures != n || sub.NextCheck.Sub(now) != time.Minute*time.Duration(1<<(n-1)) {
			t.Fatal(sub)
		}
	}
	if c.backoff(100) != time.Hour {
		t.Fatal("backoff not capped")
	}
}

func TestSchedulerChecksAndStops(t *testing.T) {
	var calls atomic.Int32
	dir := t.TempDir()
	c, err := Open(filepath.Join(dir, "state.db"), Options{DownloadRoot: dir, Interval: 20 * time.Millisecond, Feed: func(context.Context, string, string) ([]Resource, error) { calls.Add(1); return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	create(t, c, anime, group, Rule{}, nil, nil)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return calls.Load() >= 2 })
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	n := calls.Load()
	time.Sleep(40 * time.Millisecond)
	if calls.Load() != n {
		t.Fatal("scheduler kept running")
	}
}
