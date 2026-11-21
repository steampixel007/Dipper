package main

import (
	"fmt"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/source"
)

func resetCoverCache(t *testing.T) {
	t.Helper()
	coversMu.Lock()
	savedCovers, savedOrder := covers, coverOrder
	covers, coverOrder = map[string]*ui.Bitmap{}, nil
	coversMu.Unlock()
	t.Cleanup(func() {
		coversMu.Lock()
		covers, coverOrder = savedCovers, savedOrder
		coversMu.Unlock()
	})
}

func coverUpdates(t *testing.T, state *State) <-chan func() {
	t.Helper()
	ctx := t.Context()
	updates := make(chan func(), 8)
	state.Ctx = ctx
	state.Update = func(update func()) {
		select {
		case updates <- update:
		case <-ctx.Done():
		}
	}
	return updates
}

func awaitCoverUpdate(t *testing.T, updates <-chan func(), ready func() bool) {
	t.Helper()
	timeout := time.NewTimer(8 * time.Second)
	defer timeout.Stop()
	for !ready() {
		select {
		case update := <-updates:
			update()
		case <-timeout.C:
			t.Fatal("cover state did not reach the expected condition")
		}
	}
}

func TestCoverCacheEvictsLeastRecentlyUsedBeyondCap(t *testing.T) {
	resetCoverCache(t)
	state := newState("")
	state.coverLoader.keep = map[string]bool{"visible": true}
	state.coverLoader.ratios = make(map[string]float32)
	bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 540, 768)))
	capacity := int(coverMemoryCap / coverBitmapBytes(bitmap))
	state.admitCover("visible", bitmap)
	for index := range capacity {
		state.admitCover(fmt.Sprintf("older-%d", index), bitmap)
	}
	state.admitCover("incoming", bitmap)
	for _, key := range []string{"visible", "incoming", fmt.Sprintf("older-%d", capacity-1)} {
		if cachedCover(key) == nil {
			t.Errorf("expected %s to survive eviction", key)
		}
	}
	if cachedCover("older-0") != nil || len(covers) != capacity {
		t.Fatalf("expected byte-budget eviction of the oldest unused cover, got %d entries", len(covers))
	}
}

func TestUIPosterReturnKeepsDecodedArtwork(t *testing.T) {
	resetCoverCache(t)
	const count = 96
	data, err := os.ReadFile("../../internal/source/testdata/poster-original.webp")
	if err != nil {
		t.Fatal(err)
	}
	bitmap, err := decodeCover(data)
	if err != nil {
		t.Fatal(err)
	}
	state := uiFixture(t, "browse")
	state.IntroHidden = true
	state.Catalog = nil
	state.coverLoader.ratios = make(map[string]float32)
	for index := range count {
		key := fmt.Sprintf("poster-%03d", index)
		state.Catalog = append(state.Catalog, Anime{ID: key, Title: key, Cover: key, Day: 0})
	}
	first, last := state.Catalog[0].Cover, state.Catalog[count-1].Cover
	state.admitCover(first, bitmap)
	tester := ui.NewTester(state.view, 1320, 860)
	if !slices.Contains(state.coverLoader.visible, first) {
		t.Fatal("first poster did not start in view")
	}
	tester.Scroll(750, 500, 0, 30000)
	if !slices.Contains(state.coverLoader.visible, last) || slices.Contains(state.coverLoader.visible, first) {
		t.Fatal("scroll did not leave the first poster")
	}
	for _, anime := range state.Catalog[1:] {
		state.admitCover(anime.Cover, bitmap)
	}
	tester.Scroll(750, 500, 0, -30000)
	if !slices.Contains(state.coverLoader.visible, first) {
		t.Fatal("scroll did not return to the first poster")
	}
	if cachedCover(first) != bitmap {
		t.Fatalf("returned poster lost its decoded bitmap after viewing %d posters (%d still cached)", count, len(covers))
	}
}

func TestCoverDiskCacheEvictsOldestBeyondCap(t *testing.T) {
	dir := t.TempDir()
	oldest, recent, incoming := coverCacheKey("oldest"), coverCacheKey("recent"), coverCacheKey("incoming")
	base := time.Unix(1700000000, 0)
	for index, key := range []string{oldest, recent} {
		path := filepath.Join(dir, key)
		if err := os.WriteFile(path, make([]byte, 40), 0644); err != nil {
			t.Fatal(err)
		}
		used := base.Add(time.Duration(index) * time.Minute)
		if err := os.Chtimes(path, used, used); err != nil {
			t.Fatal(err)
		}
	}
	var disk coverDisk
	disk.write(dir, incoming, make([]byte, 30))
	disk.evict(100, incoming)
	if _, err := os.Stat(filepath.Join(dir, oldest)); !os.IsNotExist(err) {
		t.Fatalf("oldest file survived eviction: %v", err)
	}
	for _, key := range []string{recent, incoming} {
		if _, err := os.Stat(filepath.Join(dir, key)); err != nil {
			t.Fatal(err)
		}
	}
	if disk.total != 70 {
		t.Fatalf("disk size = %d, want 70", disk.total)
	}
}

func TestCoverDiskCacheKeepsProtectedFileBeyondCap(t *testing.T) {
	dir := t.TempDir()
	protected, disposable := coverCacheKey("protected"), coverCacheKey("disposable")
	if err := os.WriteFile(filepath.Join(dir, protected), make([]byte, 120), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, disposable), make([]byte, 40), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, protected), old, old); err != nil {
		t.Fatal(err)
	}
	var disk coverDisk
	disk.index(dir)
	disk.evict(100, protected)

	if _, err := os.Stat(filepath.Join(dir, protected)); err != nil {
		t.Fatalf("protected cover was evicted above the byte budget: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, disposable)); !os.IsNotExist(err) {
		t.Fatalf("unprotected cover survived eviction: %v", err)
	}
	if disk.total != 120 {
		t.Fatalf("disk size = %d, want 120", disk.total)
	}
}

func TestCoverDiskWriteProtectsViewportFile(t *testing.T) {
	dir := t.TempDir()
	kept, discarded, incoming := coverCacheKey("kept"), coverCacheKey("discarded"), coverCacheKey("incoming")
	for _, key := range []string{kept, discarded} {
		if err := os.WriteFile(filepath.Join(dir, key), make([]byte, 80), 0644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, kept), old, old); err != nil {
		t.Fatal(err)
	}
	var disk coverDisk
	disk.limit = 100
	disk.write(dir, incoming, make([]byte, 50), "kept")

	if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
		t.Fatalf("viewport cover was evicted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, discarded)); !os.IsNotExist(err) {
		t.Fatalf("unprotected cover survived eviction: %v", err)
	}
	if disk.total != 130 {
		t.Fatalf("disk size = %d, want 130", disk.total)
	}
}

func TestCoverDiskCacheCleansOnlyStaleTemps(t *testing.T) {
	dir := t.TempDir()
	stale, fresh := filepath.Join(dir, ".cover-stale"), filepath.Join(dir, ".cover-fresh")
	for _, path := range []string{stale, fresh} {
		if err := os.WriteFile(path, []byte("partial"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	var disk coverDisk
	disk.write(dir, coverCacheKey("new"), []byte("cover"))

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temporary cover survived cleanup: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh temporary cover was removed: %v", err)
	}
}

func TestCoverWarmHitBypassesColdRequest(t *testing.T) {
	resetCoverCache(t)
	state := uiFixture(t, "browse")
	poster, err := assets.ReadFile("assets/" + state.Catalog[0].Cover)
	if err != nil {
		t.Fatal(err)
	}
	requests := make(chan string, 8)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		select {
		case <-release:
			_, _ = w.Write(poster)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	state.Source = source.New(server.URL)
	updates := coverUpdates(t, state)
	cold, warm := server.URL+"/cold.jpg", server.URL+"/warm.jpg"
	state.coverLoader.disk.write(state.CoverDir, coverCacheKey(warm), poster)
	state.coverLoader.visible = []string{cold, coverURL(cold + "?width=400")}
	state.syncCovers()
	if len(state.coverLoader.wanted) != 1 {
		t.Fatal("normalized duplicate poster was queued twice")
	}
	awaitCoverUpdate(t, updates, func() bool { return state.coverLoader.fetching[cold] != nil })
	select {
	case path := <-requests:
		if path != "/cold.jpg" {
			t.Fatalf("first request = %s", path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cold request never started")
	}
	state.coverLoader.visible = append(state.coverLoader.visible, warm)
	state.syncCovers()
	awaitCoverUpdate(t, updates, func() bool { return cachedCover(warm) != nil })
	if cachedCover(cold) != nil {
		t.Fatal("blocked cold request unexpectedly completed")
	}
	state.coverLoader.visible = nil
	state.syncCovers()
	awaitCoverUpdate(t, updates, func() bool { return len(state.coverLoader.fetching) == 0 })
	select {
	case path := <-requests:
		t.Fatalf("unexpected duplicate or cache-hit request: %s", path)
	default:
	}
}

func TestCoverColdFirstBatchStartsBeforeAnyResponse(t *testing.T) {
	resetCoverCache(t)
	state := uiFixture(t, "browse")
	poster, err := assets.ReadFile("assets/" + state.Catalog[0].Cover)
	if err != nil {
		t.Fatal(err)
	}
	const firstBatch = 6
	catalog, err := os.ReadFile("../../internal/source/testdata/catalog-source.html")
	if err != nil {
		t.Fatal(err)
	}
	starts := make(chan struct{}, firstBatch)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Home/BangumiCoverFlowByDayOfWeek" {
			_, _ = w.Write(catalog)
			return
		}
		starts <- struct{}{}
		select {
		case <-release:
			_, _ = w.Write(poster)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer server.CloseClientConnections()
	state.Source = source.New(server.URL)
	updates := coverUpdates(t, state)
	started := time.Now()
	rows, err := state.Source.Catalog(t.Context(), 2026, "autumn")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < firstBatch {
		t.Fatalf("fixture has %d posters, need %d", len(rows), firstBatch)
	}
	for _, row := range rows[:firstBatch] {
		state.coverLoader.visible = append(state.coverLoader.visible, coverURL(row.Cover))
	}
	state.syncCovers()
	deadline := time.NewTimer(4 * time.Second)
	defer deadline.Stop()
	for count := 0; count < firstBatch; {
		select {
		case update := <-updates:
			update()
		case <-starts:
			count++
		case <-deadline.C:
			t.Fatalf("only %d cold posters started while responses were held", count)
		}
	}
	t.Logf("six cold cover requests started in %v", time.Since(started))
	close(release)
	awaitCoverUpdate(t, updates, func() bool {
		return !slices.ContainsFunc(state.coverLoader.visible, func(key string) bool { return cachedCover(key) == nil })
	})
	t.Logf("six cold covers decoded and visible in %v", time.Since(started))
}

func TestCoverPrefetchUsesIdleSlotsBeforeVisibleResponse(t *testing.T) {
	resetCoverCache(t)
	state := uiFixture(t, "browse")
	requests := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		<-r.Context().Done()
	}))
	defer server.Close()
	defer server.CloseClientConnections()
	state.Source = source.New(server.URL)
	updates := coverUpdates(t, state)
	state.coverLoader.visible = []string{server.URL + "/visible.jpg"}
	for index := range 3 {
		state.coverLoader.nearby = append(state.coverLoader.nearby, fmt.Sprintf("%s/near-%d.jpg", server.URL, index))
	}
	started := time.Now()
	state.syncCovers()
	seen := make(map[string]bool)
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for len(seen) < 4 {
		select {
		case update := <-updates:
			update()
		case path := <-requests:
			seen[path] = true
		case <-deadline.C:
			t.Fatalf("lookahead waited for a held response, started requests: %v", seen)
		}
	}
	for _, path := range []string{"/visible.jpg", "/near-0.jpg", "/near-1.jpg", "/near-2.jpg"} {
		if !seen[path] {
			t.Fatalf("request %s not prioritized: %v", path, seen)
		}
	}
	t.Logf("visible and three near-viewport requests started in %v with all responses held", time.Since(started))
	if len(state.coverLoader.fetching) != 4 {
		t.Fatalf("expected one visible and three speculative downloads, got %d flights", len(state.coverLoader.fetching))
	}
}

func TestCoverStartedBeforeScrollIsCachedForReturn(t *testing.T) {
	resetCoverCache(t)
	state := uiFixture(t, "browse")
	poster, err := assets.ReadFile("assets/" + state.Catalog[0].Cover)
	if err != nil {
		t.Fatal(err)
	}
	requests := make(chan string, 8)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		select {
		case <-release:
			_, _ = w.Write(poster)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer server.CloseClientConnections()
	state.Source = source.New(server.URL)
	updates := coverUpdates(t, state)
	cold, warm := server.URL+"/cold.jpg", server.URL+"/warm.jpg"
	state.coverLoader.disk.write(state.CoverDir, coverCacheKey(warm), poster)
	state.coverLoader.visible = []string{cold}
	state.syncCovers()
	awaitCoverUpdate(t, updates, func() bool { return state.coverLoader.fetching[cold] != nil })
	select {
	case path := <-requests:
		if path != "/cold.jpg" {
			t.Fatalf("first request = %s", path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cold request never started")
	}

	state.coverLoader.visible = []string{warm}
	state.syncCovers()
	close(release)
	awaitCoverUpdate(t, updates, func() bool {
		return len(state.coverLoader.fetching) == 0 && cachedCover(warm) != nil && cachedCover(cold) != nil
	})
	bitmap := cachedCover(cold)
	if bitmap == nil {
		t.Fatal("a cover that finished offscreen discarded its decoded bitmap")
	}
	if _, err := os.Stat(filepath.Join(state.CoverDir, coverCacheKey(cold))); err != nil {
		t.Fatalf("scrolling away discarded an already-started cover: %v", err)
	}

	returnedAt := time.Now()
	state.coverLoader.visible = []string{cold}
	state.syncCovers()
	if cachedCover(cold) != bitmap || state.coverLoader.probes != 0 {
		t.Fatal("return scheduled a disk read instead of reusing the decoded bitmap")
	}
	t.Logf("return to decoded cover: %v", time.Since(returnedAt))
	select {
	case path := <-requests:
		t.Fatalf("return unexpectedly requested %s again", path)
	default:
	}
}

func TestCoverVisibleMissPreemptsOffscreenDownload(t *testing.T) {
	resetCoverCache(t)
	state := uiFixture(t, "browse")
	poster, err := assets.ReadFile("assets/" + state.Catalog[0].Cover)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, source.MaxConcurrentCovers)
	canceled := make(chan struct{}, source.MaxConcurrentCovers)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/visible.jpg" {
			started <- struct{}{}
			<-r.Context().Done()
			canceled <- struct{}{}
			return
		}
		_, _ = w.Write(poster)
	}))
	defer server.Close()
	defer server.CloseClientConnections()
	state.Source = source.New(server.URL)
	updates := coverUpdates(t, state)
	visible := server.URL + "/visible.jpg"
	for index := range source.MaxConcurrentCovers {
		state.coverLoader.visible = append(state.coverLoader.visible, fmt.Sprintf("%s/offscreen-%d.jpg", server.URL, index))
	}
	state.syncCovers()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for count := 0; count < source.MaxConcurrentCovers; {
		select {
		case update := <-updates:
			update()
		case <-started:
			count++
		case <-deadline.C:
			t.Fatalf("only %d offscreen requests started", count)
		}
	}

	state.coverLoader.visible = []string{visible}
	state.syncCovers()
	awaitCoverUpdate(t, updates, func() bool { return cachedCover(visible) != nil })
	select {
	case <-canceled:
	case <-deadline.C:
		t.Fatal("no offscreen request was canceled for a visible cover")
	}
}

func TestCoverVisibleOvertakesQueuedPrefetch(t *testing.T) {
	resetCoverCache(t)
	state := uiFixture(t, "browse")
	poster, err := assets.ReadFile("assets/" + state.Catalog[0].Cover)
	if err != nil {
		t.Fatal(err)
	}
	requests := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		_, _ = w.Write(poster)
	}))
	defer server.Close()
	state.Source = source.New(server.URL)
	updates := coverUpdates(t, state)
	for index := range source.MaxConcurrentCovers {
		if _, err := state.Source.Cover(t.Context(), fmt.Sprintf("%s/spend-%d.jpg", server.URL, index)); err != nil {
			t.Fatal(err)
		}
		<-requests
	}
	first, near := server.URL+"/visible.jpg", server.URL+"/near.jpg"
	state.coverLoader.nearby = []string{near}
	state.syncCovers()
	awaitCoverUpdate(t, updates, func() bool { return state.coverLoader.fetching[near] != nil })
	state.coverLoader.visible = []string{first}
	state.syncCovers()
	awaitCoverUpdate(t, updates, func() bool { return cachedCover(first) != nil && cachedCover(near) != nil })
	got := []string{<-requests, <-requests}
	if !slices.Equal(got, []string{"/visible.jpg", "/near.jpg"}) {
		t.Fatalf("request order = %v, want visible before prefetch", got)
	}
}

func TestCoverPrefetchWaitsForQueuedVisible(t *testing.T) {
	resetCoverCache(t)
	state := uiFixture(t, "browse")
	state.Source = source.New("http://127.0.0.1:1")
	coverUpdates(t, state)
	state.coverLoader.visible = []string{"visible"}
	state.coverLoader.wanted = []string{"visible", "near"}
	state.coverLoader.keep = map[string]bool{"visible": true, "near": true}
	state.coverLoader.stages = map[string]coverStage{"near": coverMiss}
	state.coverLoader.fetching = map[string]*coverFlight{"visible": {}}
	state.pumpCovers()
	if flight := state.coverLoader.fetching["near"]; flight != nil {
		t.Fatal("a prefetch was scheduled while a visible cover still waited for admission")
	}
}

func TestCoverRateLimitLeavesWarmHitsAvailable(t *testing.T) {
	resetCoverCache(t)
	state := uiFixture(t, "browse")
	poster, err := assets.ReadFile("assets/" + state.Catalog[0].Cover)
	if err != nil {
		t.Fatal(err)
	}
	requests := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		w.Header().Set("Retry-After", "75")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	state.Source = source.New(server.URL)
	updates := coverUpdates(t, state)
	limited, warm, cold := server.URL+"/limited.jpg", server.URL+"/warm.jpg", server.URL+"/cold.jpg"
	state.coverLoader.disk.write(state.CoverDir, coverCacheKey(warm), poster)
	state.coverLoader.visible = []string{limited, warm}
	state.syncCovers()
	awaitCoverUpdate(t, updates, func() bool {
		return cachedCover(warm) != nil && state.coverLoader.stages[limited] == coverMiss
	})
	state.coverLoader.visible = append(state.coverLoader.visible, cold)
	state.syncCovers()
	awaitCoverUpdate(t, updates, func() bool { return state.coverLoader.stages[cold] == coverMiss })
	if delay := time.Until(state.coverLoader.pauseUntil); delay < 70*time.Second {
		t.Fatalf("Retry-After was not respected: remaining pause %v", delay)
	}
	if path := <-requests; path != "/limited.jpg" {
		t.Fatalf("first request = %s, want /limited.jpg", path)
	}
	if len(state.coverLoader.fetching) != 0 {
		t.Fatal("cold cover was scheduled during backoff")
	}
}

func TestUIPosterDemandFollowsViewport(t *testing.T) {
	for _, page := range []string{"browse", "following"} {
		t.Run(page, func(t *testing.T) {
			state := uiFixture(t, page)
			resetCoverCache(t)
			state.IntroHidden = true
			state.Catalog, state.Subscriptions = nil, nil
			for index := range 48 {
				id := fmt.Sprint(index)
				anime := Anime{ID: id, Title: "Anime " + id, Cover: "fixture-" + id, Day: 0}
				state.Catalog = append(state.Catalog, anime)
				state.KnownAnime[id] = anime
				state.Subscriptions = append(state.Subscriptions, Subscription{ID: id, AnimeID: id, GroupName: "Group " + id})
			}
			tester := ui.NewTester(state.view, 1320, 860)
			if !slices.Contains(state.coverLoader.visible, "fixture-0") {
				t.Fatalf("first layout did not register visible posters: %v", state.coverLoader.visible)
			}
			if len(state.coverLoader.nearby) == 0 {
				t.Fatal("first layout did not register any near-viewport posters")
			}
			if !slices.Contains(state.coverLoader.nearby, "fixture-15") {
				t.Fatalf("second offscreen row was not registered for prefetch: %v", state.coverLoader.nearby)
			}
			if slices.Contains(state.coverLoader.visible, "fixture-47") || slices.Contains(state.coverLoader.nearby, "fixture-47") {
				t.Fatal("far poster scheduled at startup")
			}
			tester.Scroll(750, 500, 0, 10000)
			if !slices.Contains(state.coverLoader.visible, "fixture-47") || slices.Contains(state.coverLoader.visible, "fixture-0") {
				t.Fatalf("scroll did not update visible posters: %v", state.coverLoader.visible)
			}
		})
	}
}
