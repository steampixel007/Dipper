package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
	"dipper.local/dipper/internal/source"
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
	s.Ctx = context.Background()
	s.Update = func(fn func()) { fn() }
	url := server.URL + "/cover.png"
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(url)))
	s.loadCovers(s.Ctx, []source.Anime{{Cover: url}})
	path := filepath.Join(s.CoverDir, key)
	deadline := time.After(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("cover not cached at %s", path)
		case <-time.After(10 * time.Millisecond):
		}
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
