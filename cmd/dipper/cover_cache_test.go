package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

func resetCoverCache(t *testing.T) {
	t.Helper()
	coversMu.Lock()
	savedCovers, savedOrder, savedLoading := covers, coverOrder, loadingCovers
	covers, coverOrder, loadingCovers = map[string]*ui.Bitmap{}, nil, map[string]bool{}
	coversMu.Unlock()
	t.Cleanup(func() {
		coversMu.Lock()
		covers, coverOrder, loadingCovers = savedCovers, savedOrder, savedLoading
		coversMu.Unlock()
	})
}

func TestCoverCacheEvictsLeastRecentlyUsedBeyondCap(t *testing.T) {
	uiTestImages.Do(loadImages)
	coversMu.Lock()
	dummy := covers["3887.jpg"]
	coversMu.Unlock()
	if dummy == nil {
		t.Fatal("fixture bitmap missing")
	}
	resetCoverCache(t)
	inUse := map[string]bool{"keep.jpg": true}
	coversMu.Lock()
	covers["keep.jpg"] = dummy
	coverOrder = append(coverOrder, "keep.jpg")
	for i := 0; i <= coverCacheCap; i++ {
		url := fmt.Sprintf("old-%d.jpg", i)
		covers[url] = dummy
		coverOrder = append(coverOrder, url)
	}
	coversMu.Unlock()
	evictCovers(inUse)
	coversMu.Lock()
	defer coversMu.Unlock()
	if covers["keep.jpg"] == nil {
		t.Fatal("in-use cover was evicted")
	}
	if len(covers) > coverCacheCap {
		t.Fatalf("cache holds %d covers over the cap", len(covers))
	}
	if covers["old-0.jpg"] != nil {
		t.Fatal("least recently used cover was kept")
	}
}

func TestCoverDiskCacheEvictsOldestBeyondCap(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.jpg")
	if err := os.WriteFile(keep, make([]byte, 10), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"old-1.jpg", "old-2.jpg", "old-3.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, 40), 0644); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Unix(1700000000, 0)
	if err := os.Chtimes(keep, base, base); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"old-1.jpg", "old-2.jpg", "old-3.jpg"} {
		mod := base.Add(time.Duration(i+1) * time.Minute)
		if err := os.Chtimes(filepath.Join(dir, name), mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	evictCoverFiles(dir, 100, map[string]bool{keep: true})
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("kept file was evicted")
	}
	if _, err := os.Stat(filepath.Join(dir, "old-1.jpg")); !os.IsNotExist(err) {
		t.Fatal("oldest file was not evicted")
	}
	for _, name := range []string{"old-2.jpg", "old-3.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was evicted though the cache is within the cap", name)
		}
	}
}

func TestLoadCoversKeepsCurrentRowsThroughEviction(t *testing.T) {
	uiTestImages.Do(loadImages)
	resetCoverCache(t)
	s := uiFixture(t, "browse")
	poster, err := assets.ReadFile("assets/" + s.Catalog[0].Cover)
	if err != nil {
		t.Fatal(err)
	}
	s.CoverDir = t.TempDir()
	if err := os.MkdirAll(s.CoverDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.CoverDir, fmt.Sprintf("%x", sha256.Sum256([]byte(s.Catalog[0].Cover)))), poster, 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Ctx = ctx
	updates := make(chan func(), 64)
	s.Update = func(fn func()) { updates <- fn }
	s.Source = source.New("http://127.0.0.1")
	url := s.Catalog[0].Cover
	s.loadCovers(ctx, []source.Anime{{ID: s.Catalog[0].ID, Cover: url}})
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for loadingCovers[url] {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatal("cover load did not finish")
		}
	}
	dummy := covers[url]
	if dummy == nil {
		t.Fatal("cached cover was not decoded")
	}
	coversMu.Lock()
	for i := 0; i <= coverCacheCap; i++ {
		older := fmt.Sprintf("older-%d.jpg", i)
		covers[older] = dummy
		coverOrder = append([]string{older}, coverOrder...)
	}
	coversMu.Unlock()
	evictCovers(nil)
	coversMu.Lock()
	defer coversMu.Unlock()
	if covers[url] == nil {
		t.Fatal("current page cover was evicted")
	}
}
