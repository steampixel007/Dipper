package main

import (
	"testing"

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
	"dipper.local/dipper/internal/source"
)

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
