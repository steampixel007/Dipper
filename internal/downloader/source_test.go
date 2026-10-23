package downloader

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestSourceMetadataPriorityFallbackAndIdentity(t *testing.T) {
	f := newFixture(t, 64<<10)
	other := newFixture(t, 65<<10)
	f.mi.Announce = "http://127.0.0.1:9/announce"
	var good, wrong bytes.Buffer
	if err := f.mi.Write(&good); err != nil {
		t.Fatal(err)
	}
	if err := other.mi.Write(&wrong); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good":
			w.Write(good.Bytes())
		case "/wrong":
			w.Write(wrong.Bytes())
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	magnet := f.magnet + "&tr=http%3A%2F%2F127.0.0.1%3A8%2Fannounce"
	spec, err := sourceSpec(context.Background(), Task{Magnet: magnet, TorrentURL: server.URL + "/good"})
	if err != nil || len(spec.InfoBytes) == 0 || len(spec.Trackers) < 2 || len(spec.PeerAddrs) == 0 {
		t.Fatalf("HTTP metadata/merged discovery missing: spec=%+v err=%v", spec, err)
	}
	spec, err = sourceSpec(context.Background(), Task{Magnet: magnet, TorrentURL: server.URL + "/failed"})
	if err != nil || len(spec.InfoBytes) != 0 || spec.InfoHash != f.mi.HashInfoBytes() {
		t.Fatalf("HTTP failure did not fall back to magnet: spec=%+v err=%v", spec, err)
	}
	if _, err := sourceSpec(context.Background(), Task{Magnet: magnet, TorrentURL: server.URL + "/wrong"}); err == nil {
		t.Fatal("hash mismatch silently fell back to magnet")
	}
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	id := f.mi.HashInfoBytes().HexString()
	task := Task{ID: id, Magnet: magnet, TorrentURL: server.URL + "/good", Folder: filepath.Join(root, "files")}
	wrongID := task
	wrongID.ID = other.mi.HashInfoBytes().HexString()
	if err := e.Ensure(wrongID); err == nil || len(e.Snapshot()) != 0 {
		t.Fatalf("incorrect task identity admitted: %v", err)
	}
	wrongURL := wrongID
	wrongURL.Magnet = ""
	if err := e.Ensure(wrongURL); err == nil || len(e.Snapshot()) != 0 {
		t.Fatalf("incorrect HTTP-only identity admitted: %v", err)
	}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	task.Magnet = f.magnet + "&tr=http%3A%2F%2F127.0.0.1%3A7%2Fannounce"
	task.TorrentURL = server.URL + "/new-url"
	if err := e.Ensure(task); err != nil {
		t.Fatalf("same hash/folder rejected discovery changes: %v", err)
	}
	task.Magnet = other.magnet
	if err := e.Ensure(task); err == nil {
		t.Fatal("existing task accepted mismatched magnet identity")
	}
}

func TestCloseCancelsInFlightEnsure(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	result := make(chan error, 1)
	go func() {
		result <- e.Ensure(Task{ID: "inflight", TorrentURL: server.URL, Folder: filepath.Join(root, "files")})
	}()
	<-started
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) || len(e.Snapshot()) != 0 {
			t.Fatalf("in-flight admission survived Close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not cancel HTTP fetch")
	}
}
