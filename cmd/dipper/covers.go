package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

const (
	// coverCacheCap bounds the decoded bitmaps kept in memory; a browsing
	// session can afford to re-decode evicted covers from the disk cache.
	coverCacheCap = 300
	// coverDiskCap bounds the on-disk cover cache payload; evicted files
	// re-fetch from the source when a page needs them again.
	coverDiskCap = 96 << 20
)

// coverOrder lists cached cover URLs oldest first by last use.
var coverOrder []string

func (s *State) loadCovers(ctx context.Context, rows []source.Anime) {
	pending := make([]source.Anime, 0, len(rows))
	inUse := make(map[string]bool)
	for _, a := range rows {
		if a.Cover == "" {
			continue
		}
		touchCover(a.Cover)
		inUse[a.Cover] = true
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
	evictCovers(inUse)
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
			keep := make(map[string]bool)
			for _, p := range pending {
				if p.Cover != "" {
					keep[filepath.Join(s.CoverDir, fmt.Sprintf("%x", sha256.Sum256([]byte(p.Cover))))] = true
				}
			}
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
				evictCoverFiles(s.CoverDir, coverDiskCap, keep)
			}
		}
	}()
}

// touchCover marks a cover as recently used for eviction.
func touchCover(url string) {
	if url == "" {
		return
	}
	coversMu.Lock()
	defer coversMu.Unlock()
	for i, used := range coverOrder {
		if used == url {
			coverOrder = append(coverOrder[:i], coverOrder[i+1:]...)
			break
		}
	}
	coverOrder = append(coverOrder, url)
}

// evictCovers drops the least recently used decoded covers no current row is
// showing until the memory cache is back within its cap.
func evictCovers(inUse map[string]bool) {
	coversMu.Lock()
	defer coversMu.Unlock()
	for len(covers) > coverCacheCap {
		victim := ""
		for _, url := range coverOrder {
			if !inUse[url] {
				victim = url
				break
			}
		}
		if victim == "" {
			return
		}
		delete(covers, victim)
		for i, url := range coverOrder {
			if url == victim {
				coverOrder = append(coverOrder[:i], coverOrder[i+1:]...)
				break
			}
		}
	}
}

// evictCoverFiles keeps the on-disk cache within its byte cap by deleting the
// oldest files first; paths in keep stay for the rows just loaded.
func evictCoverFiles(dir string, capBytes int64, keep map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type file struct {
		path string
		size int64
		mod  time.Time
	}
	var files []file
	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, file{filepath.Join(dir, entry.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	if total <= capBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for _, f := range files {
		if total <= capBytes {
			return
		}
		if keep[f.path] {
			continue
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}
