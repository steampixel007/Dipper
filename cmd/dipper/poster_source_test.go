package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

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
				state.Ctx = t.Context()
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
				updates := make(chan func(), 1)
				state.Update = func(update func()) { updates <- update }
				tester := ui.NewTester(state.view, 1304, 891)
				for attempt := range 2 {
					coversMu.Lock()
					delete(covers, anime.Cover)
					coversMu.Unlock()
					state.loadCovers(context.Background(), []source.Anime{{ID: anime.ID, Cover: anime.Cover}})
					select {
					case update := <-updates:
						update()
					case <-time.After(5 * time.Second):
						t.Fatal("poster did not finish loading")
					}
					bitmap := covers[anime.Cover]
					if bitmap == nil {
						t.Fatal("poster was not decoded")
					}
					width, height := bitmap.Size()
					if width != 1086 || height != 1536 {
						t.Fatalf("poster was cropped before rendering: decoded %dx%d, want original 1086x1536 (reload %d)", width, height, attempt)
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
