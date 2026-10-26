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

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

func TestStartupRestoresFollowingCover(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cached    bool
		failFirst bool
	}{{name: "cached", cached: true}, {name: "download"}, {name: "retry", failFirst: true}} {
		t.Run(tc.name, func(t *testing.T) {
			s := uiFixture(t, "following")
			poster, err := assets.ReadFile("assets/" + s.Catalog[0].Cover)
			if err != nil {
				t.Fatal(err)
			}
			var coverRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/saved.jpg" {
					if coverRequests.Add(1) == 1 && tc.failFirst {
						http.Error(w, "cover unavailable", http.StatusServiceUnavailable)
						return
					}
					w.Header().Set("Content-Type", "image/jpeg")
					_, _ = w.Write(poster)
					return
				}
				http.Error(w, "catalog unavailable", http.StatusServiceUnavailable)
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.Ctx = ctx
			s.Source = source.New(server.URL)
			url := server.URL + "/saved.jpg?width=400&height=560&format=webp"
			t.Cleanup(func() {
				delete(covers, url)
				delete(loadingCovers, url)
			})
			path := filepath.Join(s.CoverDir, fmt.Sprintf("%x", sha256.Sum256([]byte(url))))
			if tc.cached {
				if err := os.MkdirAll(s.CoverDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, poster, 0644); err != nil {
					t.Fatal(err)
				}
			}
			state := core.State{
				Anime:         []core.AnimeRecord{{Anime: core.Anime{ID: "4075", Title: "战车椅子", Cover: url}}},
				Subscriptions: []core.Subscription{{ID: "saved", Anime: core.Anime{ID: "4075"}, Group: core.Group{ID: "370", Name: "字幕组"}}},
			}
			s.project(state, nil)
			updates := make(chan func())
			s.Update = func(fn func()) {
				select {
				case updates <- fn:
				case <-ctx.Done():
				}
			}
			s.refreshCatalog()
			if !loadingCovers[url] {
				t.Fatal("startup did not schedule the saved subscription cover")
			}
			// Changing seasons cancels the catalog request, not a saved cover.
			s.refreshCatalog()
			tt := ui.NewTester(s.view, 1320, 860)
			if _, ok := tt.Find("正在加载封面"); !ok {
				t.Fatal("restored subscription has no loading indicator")
			}
			deadline := time.NewTimer(10 * time.Second)
			defer deadline.Stop()
			poll := time.NewTicker(10 * time.Millisecond)
			defer poll.Stop()
			if tc.failFirst {
				for loadingCovers[url] || s.SourceLoading {
					select {
					case fn := <-updates:
						fn()
					case <-deadline.C:
						t.Fatal("failed cover request did not finish")
					}
				}
				tt.Frame()
				if _, ok := tt.Find("正在加载封面"); !ok {
					t.Fatal("failed cover request replaced the loading indicator with the bird")
				}
				if covers[url] != nil {
					t.Fatal("failed request unexpectedly produced a cover")
				}
				s.refreshCatalog()
			}
			for {
				data, err := os.ReadFile(path)
				if covers[url] != nil && !s.SourceLoading && err == nil && bytes.Equal(data, poster) {
					break
				}
				select {
				case fn := <-updates:
					fn()
				case <-poll.C:
				case <-deadline.C:
					t.Fatalf("saved cover not restored: bitmap=%v catalogLoading=%v catalogError=%q cacheError=%v", covers[url] != nil, s.SourceLoading, s.SourceError, err)
				}
			}
			if s.SourceError == "" {
				t.Error("test did not exercise an unavailable catalog")
			}
			wantRequests := int32(1)
			if tc.cached {
				wantRequests = 0
			} else if tc.failFirst {
				wantRequests = 2
			}
			if got := coverRequests.Load(); got != wantRequests {
				t.Errorf("cover requests = %d, want %d", got, wantRequests)
			}
			s.project(state, nil)
			tt.Frame()
			if _, ok := tt.Find("正在加载封面"); ok {
				t.Error("restored cover still shows loading after the next state snapshot")
			}
		})
	}
}
