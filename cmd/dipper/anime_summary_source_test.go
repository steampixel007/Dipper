package main

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"dipper.local/dipper/internal/source"
)

func TestCatalogRefreshRetainsLoadedAnimeSummary(t *testing.T) {
	state := uiFixture(t, "browse")
	state.Subscriptions = nil
	body, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "catalog-source.html"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/Home/BangumiCoverFlowByDayOfWeek" {
			t.Errorf("unexpected request: %s", request.URL)
			http.NotFound(writer, request)
			return
		}
		writer.Write(body)
	}))
	defer server.Close()
	var coverURLs []string
	for _, match := range regexp.MustCompile(`data-src="([^"]+)"`).FindAllStringSubmatch(string(body), -1) {
		coverURLs = append(coverURLs, server.URL+html.UnescapeString(match[1]))
	}
	coversMu.Lock()
	for _, coverURL := range coverURLs {
		covers[coverURL] = covers["3887.jpg"]
	}
	coversMu.Unlock()
	t.Cleanup(func() {
		coversMu.Lock()
		defer coversMu.Unlock()
		for _, coverURL := range coverURLs {
			delete(covers, coverURL)
		}
	})
	state.Ctx, state.Source = context.Background(), source.New(server.URL)
	state.Year, state.Season = "2026", "秋"
	state.KnownAnime["4031"] = Anime{ID: "4031", Title: "旧标题", Summary: "已经读过的番剧简介。"}
	updates := make(chan func(), 1)
	state.Update = func(update func()) { updates <- update }
	state.refreshCatalog()
	select {
	case update := <-updates:
		update()
	case <-time.After(5 * time.Second):
		t.Fatal("catalog refresh did not report a result")
	}
	if state.SourceError != "" || state.SourceLoading {
		t.Fatalf("catalog did not load: %q", state.SourceError)
	}
	if anime := state.anime("4031"); anime.Title != "天是红河岸" || anime.Summary != "已经读过的番剧简介。" {
		t.Fatalf("catalog refresh lost the loaded synopsis or retained stale catalog data: %+v", anime)
	}
}
