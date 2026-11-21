package main

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
	"github.com/steampixel007/Dipper/internal/source"
)

func TestPreferencesUseConfigDirectory(t *testing.T) {
	root := t.TempDir()
	s := newState(filepath.Join(root, "data"))
	s.ConfigDir = filepath.Join(root, "config")
	if err := os.MkdirAll(s.ConfigDir, 0700); err != nil {
		t.Fatal(err)
	}
	s.CloseBehavior = "quit"
	if err := s.savePreferences(); err != nil {
		t.Fatal(err)
	}
	pref, err := readPreferences(s.ConfigDir)
	if err != nil || pref.CloseBehavior != "quit" {
		t.Fatalf("config preferences = %+v, %v", pref, err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "preferences.json")); !os.IsNotExist(err) {
		t.Fatalf("preferences unexpectedly written to data directory: %v", err)
	}
}

func TestCoverUsesCacheDirectory(t *testing.T) {
	resetCoverCache(t)
	root := t.TempDir()
	s := newState(filepath.Join(root, "data"))
	s.CoverDir = filepath.Join(root, "cache", "covers")
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(encoded.Bytes())
	}))
	defer server.Close()
	s.Source = source.New(server.URL)
	updates := coverUpdates(t, s)
	url := server.URL + "/cover.png"
	key := coverCacheKey(url)
	s.coverLoader.visible = []string{url}
	s.syncCovers()
	awaitCoverUpdate(t, updates, func() bool { return cachedCover(url) != nil })
	path := filepath.Join(s.CoverDir, key)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cover not cached at %s: %v", path, err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "covers", key)); !os.IsNotExist(err) {
		t.Fatalf("cover unexpectedly cached in data directory: %v", err)
	}
}

func TestStartupStateHasDownloader(t *testing.T) {
	engine := &downloader.Engine{}
	s := newAppState(t.Context(), t.TempDir(), source.New("http://127.0.0.1"), &core.Core{}, engine)
	if s.Engine == nil {
		t.Fatal("startup state has no downloader: loading saved settings would crash")
	}
	if s.Engine != engine {
		t.Fatal("startup state must use the running downloader for settings and task actions")
	}
	if snapshots := s.Engine.Snapshot(); len(snapshots) != 0 {
		t.Fatalf("new downloader has unexpected tasks: %v", snapshots)
	}
}
