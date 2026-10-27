package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

func (s *State) loadCovers(ctx context.Context, rows []source.Anime) {
	pending := make([]source.Anime, 0, len(rows))
	for _, a := range rows {
		if a.Cover == "" {
			continue
		}
		coversMu.Lock()
		loadedOrLoading := covers[a.Cover] != nil || loadingCovers[a.Cover]
		if !loadedOrLoading {
			loadingCovers[a.Cover] = true
		}
		coversMu.Unlock()
		if !loadedOrLoading {
			pending = append(pending, a)
		}
	}
	if len(pending) == 0 {
		return
	}
	go func() {
		for i, a := range pending {
			if ctx.Err() != nil {
				rest := make([]string, 0, len(pending)-i)
				for _, p := range pending[i:] {
					rest = append(rest, p.Cover)
				}
				s.Update(func() {
					for _, u := range rest {
						coversMu.Lock()
						delete(loadingCovers, u)
						coversMu.Unlock()
					}
				})
				return
			}
			key := fmt.Sprintf("%x", sha256.Sum256([]byte(a.Cover)))
			var data []byte
			path := ""
			if s.CoverDir != "" {
				path = filepath.Join(s.CoverDir, key)
				data, _ = os.ReadFile(path)
			}
			if len(data) == 0 {
				var err error
				data, err = s.Source.Cover(ctx, a.Cover)
				if err != nil {
					s.Update(func() {
						coversMu.Lock()
						delete(loadingCovers, a.Cover)
						coversMu.Unlock()
					})
					continue
				}
			}
			// Decode on the UI thread; its bitmap cache is also owned by that thread.
			s.Update(func() {
				coversMu.Lock()
				delete(loadingCovers, a.Cover)
				if bitmap, err := ui.DecodeBitmap(data); err == nil {
					covers[a.Cover] = bitmap
				}
				coversMu.Unlock()
			})
			if s.CoverDir != "" {
				if os.MkdirAll(filepath.Dir(path), 0755) == nil {
					_ = os.WriteFile(path, data, 0644)
				}
			}
		}
	}()
}
