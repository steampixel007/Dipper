package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/source"
)

func requireNoSourceDetails(t *testing.T, tester *ui.Tester) {
	t.Helper()
	for _, text := range tester.Texts() {
		lower := strings.ToLower(text)
		for _, detail := range []string{"mikan:", "https://", "http://", "deadline exceeded", "access challenge", "response structure", "read response"} {
			if strings.Contains(lower, detail) {
				t.Fatalf("source detail leaked in %q", text)
			}
		}
	}
}

func TestLibraryHeroMapsSourceErrorsToLocalCopy(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"timeout", "mikan: GET https://mikanime.tv/RSS/Bangumi?id=1: context deadline exceeded", "网络连接超时，请稍后再试。"},
		{"connection", "mikan: GET https://mikanime.tv/Home/Bangumi/1: dial tcp: connectex: No connection could be made", "网络连接失败，请检查网络后重试。"},
		{"status", "mikan: HTTP 503 from https://mikanime.tv/Home/Bangumi/1", "来源网站暂时不可用（503），请稍后再试。"},
		{"challenge", "mikan: access challenge", "来源网站要求人机验证，请稍后再试。"},
		{"structure", "mikan: unrecognized or inconsistent response structure: missing RSS 2.0 channel", "来源页面的结构发生变化，请稍后再试。"},
		{"not-found", "mikan: anime or subtitle group not found", "没有找到这部番剧或字幕组，请返回后重试。"},
		{"too-large", "mikan: response exceeds size limit", "来源网站返回的内容过大，请稍后再试。"},
		{"incomplete", "mikan: cannot confirm complete history", "资源还没加载完整，请稍后再试。"},
		{"eof", `mikan: GET https://mikanime.tv/RSS/Bangumi: Get "https://mikanani.me/rss/bangumi": EOF`, "网络连接失败，请检查网络后重试。"},
		{"unexpected-eof", "mikan: read response: unexpected EOF", "网络连接失败，请检查网络后重试。"},
		{"unknown", "mikan: unknown response failure", "暂时无法获取来源网站的信息，请稍后再试。"},
		{"prefix", "检查更新失败：RSS 请求超时", "网络连接超时，请稍后再试。"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			state := uiFixture(t, "episodes")
			sub := &state.Subscriptions[0]
			state.LibraryID = sub.ID
			sub.CheckFailed, sub.LastError = true, test.raw
			tester := ui.NewTester(state.view, 1320, 860)
			uiRequireText(t, tester, "检查更新失败："+test.want, "重试检查 "+sub.GroupName)
			if strings.Contains(strings.Join(tester.Texts(), "\n"), "检查更新失败：检查更新失败") {
				t.Fatal("duplicated failure prefix")
			}
			requireNoSourceDetails(t, tester)
		})
	}
}

func TestLibraryHeroPreservesLocalCheckErrors(t *testing.T) {
	for _, message := range []string{
		"mkdir /downloads/anime: permission denied",
		"open /downloads/anime: file not found",
		"写入下载目录失败：磁盘空间不足",
		"rpc error: code = Unavailable",
		"open /downloads/anime.torrent: unexpected EOF",
		"create content folder: mkdir /downloads/timeout/anime: permission denied",
		"open /downloads/connection refused/anime.torrent: access denied",
	} {
		t.Run(message, func(t *testing.T) {
			state := uiFixture(t, "episodes")
			sub := &state.Subscriptions[0]
			state.LibraryID = sub.ID
			sub.CheckFailed, sub.LastError = true, message
			tester := ui.NewTester(state.view, 1320, 860)
			uiRequireText(t, tester, "检查更新失败："+message, "重试检查 "+sub.GroupName)
		})
	}
}

func TestLibraryHeroPreservesDownloaderCheckFailure(t *testing.T) {
	state, engine, updates, _ := submissionFixture(t, false)
	sub, err := state.Core.CreateSubscription(state.Ctx,
		core.Anime{ID: state.AnimeID, Title: "下载错误测试"},
		core.Group{ID: "check-group", Name: "字幕组"}, core.Rule{All: []string{"1080p"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	engine.reject = true
	checkErr := state.Core.Check(state.Ctx, sub.ID)
	if checkErr == nil || !strings.Contains(checkErr.Error(), "scripted disk failure") {
		t.Fatalf("expected downloader failure after successful feed, got %v", checkErr)
	}
	snapshot, err := state.Core.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	state.project(snapshot, engine.Snapshot())
	state.Page, state.LibraryID = "episodes", sub.ID
	tester := ui.NewTester(state.view, 1320, 860)
	uiRequireText(t, tester, "检查更新失败："+checkErr.Error(), "重试检查 字幕组")
	uiClick(t, tester, "重试检查 字幕组")
	finishSubmission(t, state, updates)
	if !strings.Contains(state.SaveError, "scripted disk failure") || state.Notice != "" {
		t.Fatalf("single check lost its actionable download error: error=%q notice=%q", state.SaveError, state.Notice)
	}
}

func TestCatalogFailureAllowsSeasonRecovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("seasonStr") == "春" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(routeCatalogMarkup))
	}))
	defer server.Close()
	state := uiFixture(t, "browse")
	state.Source, state.Ctx = source.New(server.URL), t.Context()
	state.Year, state.Season = "2026", "春"
	updates := make(chan func(), 16)
	state.Update = func(update func()) { updates <- update }
	waitCatalog := func() {
		t.Helper()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		for state.SourceLoading {
			select {
			case update := <-updates:
				update()
			case <-deadline.C:
				t.Fatal("catalog request did not finish")
			}
		}
	}
	state.refreshCatalog()
	waitCatalog()
	if state.SourceError == "" {
		t.Fatal("spring request should fail")
	}
	tester := ui.NewTester(state.view, 1040, 720)
	tester.SetPreferences(ui.Preferences{TextScale: 2})
	uiRequireText(t, tester, "重试加载", "下一季")
	for _, label := range []string{"重试加载", "上一季", "下一季", "回到当季"} {
		element, ok := tester.Find(label)
		if !ok || element.W <= 0 || element.H <= 0 || element.Y < 0 || element.Y+element.H > 720 {
			t.Fatalf("recovery action %q is not reachable: %+v", label, element)
		}
	}
	uiClick(t, tester, "下一季")
	waitCatalog()
	tester.Frame()
	if state.Season != "夏" || state.SourceError != "" || len(state.Catalog) != 1 {
		t.Fatalf("season recovery failed: season=%s error=%q catalog=%v", state.Season, state.SourceError, state.Catalog)
	}
	uiRequireText(t, tester, "节目")
}

func TestCatalogSourceErrorMapsToLocalCopy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	state := uiFixture(t, "browse")
	state.Source = source.New(server.URL)
	state.Ctx = t.Context()
	var mu sync.Mutex
	state.Update = func(fn func()) {
		mu.Lock()
		defer mu.Unlock()
		fn()
	}
	state.refreshCatalog()
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		loading, failure := state.SourceLoading, state.SourceError
		mu.Unlock()
		if !loading {
			if failure != "来源网站暂时不可用（503），请稍后再试。" {
				t.Fatalf("catalog error = %q", failure)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("catalog request did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	tester := ui.NewTester(state.view, 1320, 860)
	uiRequireText(t, tester, "来源网站暂时不可用（503），请稍后再试。", "重试加载")
	requireNoSourceDetails(t, tester)
}
