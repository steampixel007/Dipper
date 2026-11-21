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
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/source"
)

func TestStartupRestoresFollowingCover(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cached    bool
		failFirst bool
	}{{name: "cached", cached: true}, {name: "download"}, {name: "retry", failFirst: true}} {
		t.Run(tc.name, func(t *testing.T) {
			s := uiFixture(t, "following")
			resetCoverCache(t)
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
			s.Source = source.New(server.URL)
			url := server.URL + "/saved.jpg?width=400&height=560&format=webp"
			key := coverURL(url)
			path := filepath.Join(s.CoverDir, fmt.Sprintf("%x", sha256.Sum256([]byte(source.OriginalCoverURL(url)))))
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
			updates := coverUpdates(t, s)
			s.refreshCatalog()
			tt := ui.NewTester(s.view, 1320, 860)
			if !s.coverLoader.keep[key] {
				t.Fatal("first layout did not schedule the saved subscription cover")
			}
			s.refreshCatalog()
			if _, ok := tt.Find("正在加载封面"); !ok {
				t.Fatal("restored subscription has no loading indicator")
			}
			if tc.failFirst {
				awaitCoverUpdate(t, updates, func() bool { return s.coverLoader.stages[key] == coverMiss && !s.SourceLoading })
				tt.Frame()
				if _, ok := tt.Find("正在加载封面"); ok {
					t.Fatal("failed cover request still animates")
				}
				if cachedCover(key) != nil || len(s.coverLoader.fetching) != 0 || coverRequests.Load() != 1 {
					t.Fatal("an unavailable cover was loaded or retried during its backoff")
				}
				s.refreshCatalog()
				tt.Frame()
			}
			awaitCoverUpdate(t, updates, func() bool { return cachedCover(key) != nil && !s.SourceLoading })
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, poster) {
				t.Fatalf("saved cover missing from disk cache: %v", err)
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
