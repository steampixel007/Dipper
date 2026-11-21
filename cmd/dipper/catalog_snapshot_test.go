package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/source"
)

type cancelOnSecondCheck struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int32
}

func (c *cancelOnSecondCheck) Err() error {
	if c.checks.Add(1) >= 2 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestCatalogSnapshotCancellationLeavesNoFiles(t *testing.T) {
	dir := t.TempDir()
	base, cancel := context.WithCancel(t.Context())
	ctx := &cancelOnSecondCheck{Context: base, cancel: cancel}
	snapshot := catalogSnapshot{
		Key:  catalogKey{Origin: "https://mikanani.me", Year: 2026, Season: "春"},
		Rows: []source.Anime{{ID: "cancelled", Title: "cancelled"}},
	}
	var disk catalogDisk
	disk.write(ctx, dir, snapshot)

	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); !os.IsNotExist(err) {
		t.Fatalf("cancelled snapshot was published: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".catalog-") {
			t.Fatalf("cancelled snapshot left temporary file %q", entry.Name())
		}
	}
}

func TestCatalogSnapshotCleansOnlyStaleTemps(t *testing.T) {
	dir := t.TempDir()
	stale, fresh := filepath.Join(dir, ".catalog-stale"), filepath.Join(dir, ".catalog-fresh")
	for _, path := range []string{stale, fresh} {
		if err := os.WriteFile(path, []byte("partial"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	snapshot := catalogSnapshot{
		Key:  catalogKey{Origin: "https://mikanani.me", Year: 2026, Season: "春"},
		Rows: []source.Anime{{ID: "fresh", Title: "fresh"}},
	}
	var disk catalogDisk
	disk.write(t.Context(), dir, snapshot)

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temporary snapshot survived cleanup: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh temporary snapshot was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); err != nil {
		t.Fatalf("catalog snapshot was not published: %v", err)
	}
}

func TestCatalogSnapshotRendersBeforeRefresh(t *testing.T) {
	resetCoverCache(t)
	catalog, err := os.ReadFile("../../internal/source/testdata/catalog-source.html")
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/Home/BangumiCoverFlowByDayOfWeek" {
			http.NotFound(response, request)
			return
		}
		if requests.Add(1) == 1 {
			_, _ = response.Write(catalog)
			return
		}
		select {
		case <-release:
			response.WriteHeader(http.StatusServiceUnavailable)
		case <-request.Context().Done():
		}
	}))
	defer server.Close()
	defer server.CloseClientConnections()
	first := uiFixture(t, "browse")
	first.Year, first.Season = "2026", "秋"
	first.Source = source.New(server.URL)
	updates := coverUpdates(t, first)
	first.refreshCatalog()
	awaitCoverUpdate(t, updates, func() bool { return !first.SourceLoading })
	if first.SourceError != "" || len(first.Catalog) == 0 {
		t.Fatalf("initial catalog did not load: %q", first.SourceError)
	}
	poster, err := os.ReadFile("../../internal/source/testdata/poster-original.webp")
	if err != nil {
		t.Fatal(err)
	}
	firstCover := coverURL(first.Catalog[0].Cover)
	first.coverLoader.disk.write(first.CoverDir, coverCacheKey(firstCover), poster)

	reopened := uiFixture(t, "browse")
	reopened.Dir, reopened.CoverDir = first.Dir, first.CoverDir
	reopened.Year, reopened.Season = first.Year, first.Season
	reopened.Source = source.New(server.URL)
	updates = coverUpdates(t, reopened)
	started := time.Now()
	reopened.refreshCatalog()
	awaitCoverUpdate(t, updates, func() bool { return len(reopened.Catalog) != 0 })
	tester := ui.NewTester(reopened.view, 1320, 860)
	uiRequireText(t, tester, first.Catalog[0].Title)
	awaitCoverUpdate(t, updates, func() bool { return cachedCover(firstCover) != nil })
	tester.Frame()
	t.Logf("cached catalog and first cover ready in %v with the catalog response held", time.Since(started))

	close(release)
	awaitCoverUpdate(t, updates, func() bool { return !reopened.SourceLoading })
	tester.Frame()
	uiRequireText(t, tester, first.Catalog[0].Title, "正在显示上次加载的番剧", "重试加载")

	reopened.Year = "2025"
	reopened.refreshCatalog()
	awaitCoverUpdate(t, updates, func() bool { return !reopened.SourceLoading })
	tester.Frame()
	if tester.HasText(first.Catalog[0].Title) || len(reopened.Catalog) != 0 {
		t.Fatal("catalog snapshot was reused for another year")
	}
}
