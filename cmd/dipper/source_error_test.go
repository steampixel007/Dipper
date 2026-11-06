package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
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
		{"unknown", "rpc error: code = Unavailable", "暂时无法获取来源网站的信息，请稍后再试。"},
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
