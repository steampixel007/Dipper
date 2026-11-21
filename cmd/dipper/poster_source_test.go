package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/source"
)

func TestLoadCoversShowsOriginalPosterThroughGridAndCache(t *testing.T) {
	original, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "poster-original.webp"))
	if err != nil {
		t.Fatal(err)
	}
	square, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "poster-square.webp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"browse", "following"} {
		for _, cached := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/thumbnailCached%v", page, cached), func(t *testing.T) {
				state := uiFixture(t, page)
				resetCoverCache(t)
				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if r.URL.Query().Has("width") || r.URL.Query().Has("height") {
						w.Write(square)
						return
					}
					w.Write(original)
				}))
				defer server.Close()
				state.Source = source.New(server.URL)
				state.IntroHidden = true
				anime := state.Catalog[0]
				anime.Cover = server.URL + "/images/Bangumi/202610/8e2c071c.jpg?width=400&height=400&format=webp"
				state.Catalog = []Anime{anime}
				state.KnownAnime[anime.ID] = anime
				state.Subscriptions = []Subscription{{ID: "poster", AnimeID: anime.ID, GroupName: "字幕组"}}
				thumbnailPath := filepath.Join(state.CoverDir, fmt.Sprintf("%x", sha256.Sum256([]byte(anime.Cover))))
				if cached {
					if err := os.MkdirAll(state.CoverDir, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(thumbnailPath, square, 0644); err != nil {
						t.Fatal(err)
					}
				}
				updates := coverUpdates(t, state)
				tester := ui.NewTester(state.view, 1304, 891)
				key := coverURL(anime.Cover)
				for attempt := range 2 {
					if attempt > 0 {
						state.enterPage("settings")
						tester.Frame()
						coversMu.Lock()
						delete(covers, key)
						coversMu.Unlock()
						if state.coverLoader.keep[key] {
							t.Fatal("poster remained requested after leaving the page")
						}
						state.enterPage(page)
						tester.Frame()
					}
					awaitCoverUpdate(t, updates, func() bool { return cachedCover(key) != nil })
					bitmap := cachedCover(key)
					width, height := bitmap.Size()
					if width != 362 || height != 512 {
						t.Fatalf("decoded poster = %dx%d, want proportional 362x512 (reload %d)", width, height, attempt)
					}
					tester.Frame()
					label := "查看 " + anime.Title
					if page == "following" {
						label = "打开追番 字幕组"
					}
					tile := requireLayoutBound(t, tester, label, 1304, 891)
					title := requireLayoutBound(t, tester, anime.Title, 1304, 891)
					_, cardWidth := gridColumns(1304)
					wantHeight := cardWidth * float32(height) / float32(width)
					if gap := title.Y - tile.Y - wantHeight; gap < 9 || gap > 11 {
						t.Fatalf("grid crops or pads original poster: tile=%+v title=%+v imageHeight=%g", tile, title, wantHeight)
					}
				}
				if got := requests.Load(); got != 1 {
					t.Fatalf("original poster should be fetched once then restored from disk, got %d requests", got)
				}
				data, err := os.ReadFile(filepath.Join(state.CoverDir, coverCacheKey(key)))
				if err != nil || !bytes.Equal(data, original) {
					t.Fatalf("disk cache did not preserve the original artwork: %v", err)
				}
				if cached {
					data, err := os.ReadFile(thumbnailPath)
					if err != nil || !bytes.Equal(data, square) {
						t.Fatalf("original poster reused the cropped thumbnail's cache entry: err=%v", err)
					}
				}
			})
		}
	}
}
