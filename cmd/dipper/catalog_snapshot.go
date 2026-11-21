package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/steampixel007/Dipper/internal/source"
)

const catalogSnapshotLimit = 2 << 20

type catalogKey struct {
	Origin string
	Year   int
	Season string
}

type catalogSnapshot struct {
	Key  catalogKey
	Rows []source.Anime
}

type catalogDisk struct {
	mu sync.Mutex
}

func readCatalogSnapshot(dir string, key catalogKey) (catalogSnapshot, bool) {
	var snapshot catalogSnapshot
	if dir == "" || key.Origin == "" {
		return snapshot, false
	}
	file, err := os.Open(filepath.Join(dir, "catalog.json"))
	if err != nil {
		return snapshot, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, catalogSnapshotLimit+1))
	if err != nil || len(data) > catalogSnapshotLimit || json.Unmarshal(data, &snapshot) != nil {
		return snapshot, false
	}
	return snapshot, snapshot.Key == key && snapshot.Rows != nil
}

func (disk *catalogDisk) write(ctx context.Context, dir string, snapshot catalogSnapshot) {
	if dir == "" || snapshot.Key.Origin == "" || ctx.Err() != nil {
		return
	}
	removeStaleTemps(dir, ".catalog-", time.Hour)
	data, err := json.Marshal(snapshot)
	if err != nil || len(data) > catalogSnapshotLimit {
		return
	}
	file, err := os.CreateTemp(dir, ".catalog-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return
	}
	disk.mu.Lock()
	defer disk.mu.Unlock()
	if ctx.Err() == nil {
		_ = os.Rename(file.Name(), filepath.Join(dir, "catalog.json"))
	}
}
