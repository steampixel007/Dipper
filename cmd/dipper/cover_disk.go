package main

import (
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type coverDiskFile struct {
	size int64
	used time.Time
}

type coverDisk struct {
	mu    sync.Mutex
	dir   string
	files map[string]coverDiskFile
	total int64
	limit int64
}

func (disk *coverDisk) read(dir, key string) ([]byte, bool) {
	if dir == "" {
		return nil, false
	}
	path := filepath.Join(dir, key)
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	info, statErr := file.Stat()
	data, err := io.ReadAll(io.LimitReader(file, coverFileCap+1))
	file.Close()
	if err != nil || len(data) > coverFileCap {
		return nil, false
	}
	now := time.Now()
	disk.mu.Lock()
	defer disk.mu.Unlock()
	if disk.dir == dir {
		if entry, ok := disk.files[key]; ok {
			entry.used = now
			disk.files[key] = entry
		}
	}
	if statErr == nil && now.Sub(info.ModTime()) >= time.Hour {
		_ = os.Chtimes(path, now, now)
	}
	return data, true
}

func (disk *coverDisk) write(dir, key string, data []byte, keep ...string) {
	if dir == "" || os.MkdirAll(dir, 0755) != nil {
		return
	}
	disk.mu.Lock()
	defer disk.mu.Unlock()
	if disk.dir != dir {
		disk.index(dir)
	}
	path := filepath.Join(dir, key)
	file, err := os.CreateTemp(dir, ".cover-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil || closeErr != nil || os.Rename(file.Name(), path) != nil {
		return
	}
	disk.total += int64(len(data)) - disk.files[key].size
	disk.files[key] = coverDiskFile{size: int64(len(data)), used: time.Now()}
	keys := make([]string, 0, len(keep)+1)
	keys = append(keys, key)
	for _, cover := range keep {
		keys = append(keys, coverCacheKey(cover))
	}
	disk.evict(disk.capacity(), keys...)
}

func (disk *coverDisk) capacity() int64 {
	if disk.limit > 0 {
		return disk.limit
	}
	return coverDiskCap
}

func (disk *coverDisk) index(dir string) {
	disk.dir = dir
	disk.files = make(map[string]coverDiskFile)
	disk.total = 0
	removeStaleTemps(dir, ".cover-", time.Hour)
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		key := entry.Name()
		if entry.Type() != 0 || len(key) != 64 {
			continue
		}
		if _, err := hex.DecodeString(key); err != nil {
			continue
		}
		if info, err := entry.Info(); err == nil {
			disk.files[key] = coverDiskFile{size: info.Size(), used: info.ModTime()}
			disk.total += info.Size()
		}
	}
}

func (disk *coverDisk) evict(limit int64, keep ...string) {
	if disk.total <= limit {
		return
	}
	protected := make(map[string]bool, len(keep))
	for _, key := range keep {
		protected[key] = true
	}
	keys := make([]string, 0, len(disk.files))
	for key := range disk.files {
		if !protected[key] {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(left, right int) bool {
		return disk.files[keys[left]].used.Before(disk.files[keys[right]].used)
	})
	for _, key := range keys {
		if disk.total <= limit {
			break
		}
		if err := os.Remove(filepath.Join(disk.dir, key)); err == nil || os.IsNotExist(err) {
			disk.total -= disk.files[key].size
			delete(disk.files, key)
		}
	}
}

func removeStaleTemps(dir, prefix string, age time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		info, err := entry.Info()
		if err == nil && now.Sub(info.ModTime()) > age {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}
