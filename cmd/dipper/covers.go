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
		if a.Cover != "" && covers[a.Cover] == nil {
			pending = append(pending, a)
		}
	}
	go func() {
		for _, a := range pending {
			if ctx.Err() != nil {
				return
			}
			key := fmt.Sprintf("%x", sha256.Sum256([]byte(a.Cover)))
			path := filepath.Join(s.Dir, "covers", key)
			data, _ := os.ReadFile(path)
			if len(data) == 0 {
				var err error
				data, err = s.Source.Cover(ctx, a.Cover)
				if err != nil {
					continue
				}
			}
			// Decode on the UI thread; its bitmap cache is also owned by that thread.
			s.Update(func() {
				if bitmap, err := ui.DecodeBitmap(data); err == nil {
					covers[a.Cover] = bitmap
				}
			})
			if s.Dir != "" {
				if os.MkdirAll(filepath.Dir(path), 0755) == nil {
					_ = os.WriteFile(path, data, 0644)
				}
			}
		}
	}()
}
