package downloader

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/anacrolix/torrent"
)

func TestAppendBuiltinTrackersAddsOneTierWithoutDuplicates(t *testing.T) {
	spec := torrent.TorrentSpec{Trackers: [][]string{{"http://127.0.0.1:9/announce"}, {"udp://tracker.opentrackr.org:1337/announce"}}}
	appendBuiltinTrackers(&spec)
	tiers := spec.Trackers
	if len(tiers) != 3 {
		t.Fatalf("expected the existing two tiers plus one built-in tier, got %d: %v", len(tiers), tiers)
	}
	seen := map[string]bool{}
	for _, tier := range tiers {
		for _, tracker := range tier {
			if seen[tracker] {
				t.Fatalf("duplicate tracker %q", tracker)
			}
			seen[tracker] = true
		}
	}
	for _, tracker := range builtinTrackers {
		if !seen[tracker] {
			t.Fatalf("missing built-in tracker %q: %v", tracker, tiers)
		}
	}
}

func TestEngineAppendsBuiltinTrackersToTorrentAndMetadata(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if !hasAll(announce, builtinTrackers) {
		t.Fatalf("engine torrent lacks built-in trackers: %v", announce)
	}
	data, err := os.ReadFile(e.metadataPath(task.ID))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeSpec(data)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAll(saved.Trackers, builtinTrackers) {
		t.Fatalf("persisted metadata lacks built-in trackers: %v", saved.Trackers)
	}
}

func hasAll(tiers [][]string, want []string) bool {
	seen := map[string]bool{}
	for _, tier := range tiers {
		for _, tracker := range tier {
			seen[tracker] = true
		}
	}
	for _, tracker := range want {
		if !seen[tracker] {
			return false
		}
	}
	return true
}
