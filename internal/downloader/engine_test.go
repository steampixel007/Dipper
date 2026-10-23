package downloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"
)

func localConfig(cfg *torrent.ClientConfig) {
	cfg.ListenHost = func(string) string { return "127.0.0.1" }
	cfg.ListenPort = 0
	cfg.NoDHT, cfg.DisableTrackers, cfg.DisablePEX = true, true, true
	cfg.DisableUTP, cfg.DisableIPv6 = true, true
	cfg.DisableWebtorrent, cfg.DisableWebseeds, cfg.NoDefaultPortForwarding = true, true, true
}

func testEngine(t *testing.T, config Config) *Engine {
	t.Helper()
	e, err := newEngine(config, localConfig)
	if err != nil {
		t.Fatal(err)
	}
	e.SetNetworkAvailable(true)
	t.Cleanup(func() {
		if err := e.Close(); err != nil {
			t.Error(err)
		}
	})
	return e
}

type fixture struct {
	mi      metainfo.MetaInfo
	files   map[string][]byte
	seed    *torrent.Client
	torrent *torrent.Torrent
	magnet  string
}

func newFixture(t *testing.T, size int) *fixture {
	t.Helper()
	dir := t.TempDir()
	files := map[string][]byte{
		"release/video.mkv":      make([]byte, size),
		"release/subs/字幕.ass":    []byte("Dipper local transfer test\n"),
		"release/subs/empty.txt": {},
	}
	for i := range files["release/video.mkv"] {
		files["release/video.mkv"][i] = byte((i*37 + i/17) % 251)
	}
	for name, b := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	info := metainfo.Info{PieceLength: 64 << 10}
	if err := info.BuildFromFilePath(filepath.Join(dir, "release")); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{mi: metainfo.MetaInfo{InfoBytes: infoBytes}, files: files}
	cfg := torrent.NewDefaultClientConfig()
	localConfig(cfg)
	cfg.Seed = true
	cfg.UploadRateLimiter = rate.NewLimiter(256<<10, 16<<10)
	var content []byte
	for _, file := range info.UpvertedFiles() {
		content = append(content, files[filepath.ToSlash(filepath.Join(info.Name, filepath.Join(file.BestPath()...)))]...)
	}
	cfg.DefaultStorage = memorySeed{content}
	f.seed, err = torrent.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.seed.Close() })
	f.torrent, _, err = f.seed.AddTorrentSpec(torrent.TorrentSpecFromMetaInfo(&f.mi))
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "seed verification", func() bool { return f.torrent.Complete().Bool() })
	f.magnet = "magnet:?xt=urn:btih:" + f.mi.HashInfoBytes().HexString() + "&x.pe=" + url.QueryEscape(f.seed.ListenAddrs()[0].String())
	return f
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if condition() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("timed out: " + what)
		case <-ticker.C:
		}
	}
}

func one(t *testing.T, e *Engine) Snapshot {
	t.Helper()
	s := e.Snapshot()
	if len(s) != 1 {
		t.Fatalf("expected one task, got %v", s)
	}
	if s[0].Error != "" {
		t.Fatalf("task error: %+v", s[0])
	}
	return s[0]
}

func checkFiles(t *testing.T, f *fixture, folder string) {
	t.Helper()
	for name, want := range f.files {
		got, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if sha256.Sum256(got) != sha256.Sum256(want) {
			t.Fatalf("content mismatch: %s", name)
		}
	}
}

func TestRealMagnetPauseRestoreAndManualStop(t *testing.T) {
	f := newFixture(t, 2<<20)
	root := t.TempDir()
	config := Config{DataDir: filepath.Join(root, "state"), UploadLimit: 128 << 10}
	task := Task{ID: "episode/hash", Magnet: f.magnet, Folder: filepath.Join(root, "content")}
	e := testEngine(t, config)
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "real payload and measured download speed", func() bool {
		s := one(t, e)
		return s.DownloadedBytes >= 128<<10 && s.DownloadSpeed > 0 && s.CompletedBytes > 0 && !s.Complete
	})
	if err := e.Pause(task.ID); err != nil {
		t.Fatal(err)
	}
	paused := one(t, e)
	if paused.Status != StatusPaused || paused.DownloadSpeed != 0 || paused.UploadSpeed != 0 {
		t.Fatalf("pause snapshot: %+v", paused)
	}
	time.Sleep(150 * time.Millisecond)
	stats := f.torrent.Stats()
	up := stats.BytesWrittenData.Int64()
	time.Sleep(550 * time.Millisecond)
	stats = f.torrent.Stats()
	if delta := stats.BytesWrittenData.Int64() - up; delta != 0 {
		t.Fatalf("payload continued after pause: %d", delta)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	e = testEngine(t, config)
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	restored := one(t, e)
	if !restored.Paused || restored.DownloadedBytes != paused.DownloadedBytes || restored.UploadedBytes != paused.UploadedBytes {
		t.Fatalf("intent/counters did not survive: before=%+v after=%+v", paused, restored)
	}
	// Metadata must be immediately reconstructible without contacting any peer.
	f.seed.Close()
	if err := e.Resume(task.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "local metadata and verified partial data", func() bool {
		s := one(t, e)
		return s.TotalBytes > 0 && s.CompletedBytes >= 64<<10
	})
	if got := one(t, e).DownloadedBytes; got != paused.DownloadedBytes {
		t.Fatalf("metadata recovery redownloaded data: %d != %d", got, paused.DownloadedBytes)
	}
	fresh := newFixture(t, 2<<20)
	e.mu.Lock()
	e.tasks[task.ID].torrent.AddPeers([]torrent.PeerInfo{{Addr: torrent.StringAddr(fresh.seed.ListenAddrs()[0].String()), Source: torrent.PeerSourceDirect, Trusted: true}})
	e.mu.Unlock()
	waitFor(t, "complete download", func() bool { return one(t, e).Complete })
	complete := one(t, e)
	if complete.Progress != 1 || complete.Status != StatusSeeding || complete.DownloadedBytes < complete.TotalBytes {
		t.Fatalf("completion snapshot: %+v", complete)
	}
	checkFiles(t, f, task.Folder)
	if err := e.StopSeeding(task.ID); err != nil {
		t.Fatal(err)
	}
	stopped := one(t, e)
	if !stopped.ManuallyStopped || stopped.Status != StatusStopped {
		t.Fatalf("manual stop: %+v", stopped)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	e = testEngine(t, config)
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if got := one(t, e); !got.ManuallyStopped || got.Status != StatusStopped || got.UploadedBytes != stopped.UploadedBytes {
		t.Fatalf("manual stop did not survive restart: %+v", got)
	}
	e.mu.Lock()
	active := e.tasks[task.ID].torrent != nil
	e.mu.Unlock()
	if active {
		t.Fatal("manual stop reattached a torrent")
	}
	if err := e.Resume(task.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "explicit resume after manual stop", func() bool { return one(t, e).Status == StatusSeeding })
	beforeClose := one(t, e)
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	e = testEngine(t, config)
	waitFor(t, "exit preserves running intent", func() bool { return one(t, e).Status == StatusSeeding })
	if got := one(t, e); got.Paused || got.ManuallyStopped || got.DownloadedBytes != beforeClose.DownloadedBytes {
		t.Fatalf("close changed intent or counter epoch: %+v", got)
	}
	t.Logf("verified %d bytes across pause/restart; payload down=%d up=%d", complete.TotalBytes, complete.DownloadedBytes, complete.UploadedBytes)
}

func TestRealUploadRatioStopsAndCloseStopsPayload(t *testing.T) {
	f := newFixture(t, 512<<10)
	root := t.TempDir()
	config := Config{DataDir: filepath.Join(root, "state"), UploadLimit: 128 << 10}
	e := testEngine(t, config)
	task := Task{ID: "seed", Magnet: f.magnet, Folder: filepath.Join(root, "files")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "engine downloaded seed", func() bool { return one(t, e).Complete })
	// An independent library client consumes this engine's data; the original seed is not added.
	cfg := torrent.NewDefaultClientConfig()
	localConfig(cfg)
	cfg.DataDir = t.TempDir()
	consumer, err := torrent.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { consumer.Close() })
	spec := torrent.TorrentSpecFromMetaInfo(&f.mi)
	spec.PeerAddrs = []string{e.client.ListenAddrs()[0].String()}
	leecher, _, err := consumer.AddTorrentSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	leecher.DownloadAll()
	waitFor(t, "measured upload payload", func() bool {
		s := one(t, e)
		return s.UploadedBytes > 0 && s.UploadSpeed > 0
	})
	waitFor(t, "ratio policy stop", func() bool { return one(t, e).AutoStopped })
	s := one(t, e)
	if s.UploadedBytes < s.DownloadedBytes || s.UploadSpeed != 0 || s.Status != StatusCompleted {
		t.Fatalf("ratio stop: %+v", s)
	}
	checkFiles(t, f, task.Folder)
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	e = testEngine(t, config)
	if got := one(t, e); !got.AutoStopped || got.UploadedBytes != s.UploadedBytes {
		t.Fatalf("automatic stop did not survive: %+v", got)
	}
}

func TestEnsureCancellationAndBadSources(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- e.Ensure(Task{ID: "cancel", TorrentURL: server.URL, Folder: filepath.Join(root, "data"), Context: ctx})
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("wrong cancellation error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Ensure ignored cancellation")
	}
	if len(e.Snapshot()) != 0 {
		t.Fatal("canceled task was admitted")
	}
	for _, source := range []Task{{Magnet: "magnet:?dn=bad"}, {TorrentURL: "file:///tmp/a.torrent"}, {Magnet: "not-a-magnet"}} {
		source.ID, source.Folder = "bad", filepath.Join(root, "bad")
		if err := e.Ensure(source); err == nil {
			t.Fatalf("invalid source accepted: %+v", source)
		}
	}
}

func TestTorrentURLPersistsMetadataAndRejectsCollisions(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	config := Config{DataDir: filepath.Join(root, "state")}
	e := testEngine(t, config)
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(e.metadataPath(task.ID)); err != nil {
		t.Fatal("missing persisted metadata:", err)
	}
	for _, other := range []Task{
		{ID: "duplicate", TorrentURL: server.URL, Folder: filepath.Join(root, "other")},
		{ID: "overlap", TorrentURL: server.URL, Folder: filepath.Join(task.Folder, "child")},
		{ID: "state", TorrentURL: server.URL, Folder: config.DataDir},
	} {
		if err := e.Ensure(other); err == nil {
			t.Fatalf("collision accepted: %+v", other)
		}
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	server.Close()
	e = testEngine(t, config)
	waitFor(t, "URL metadata restored with server offline", func() bool { return one(t, e).TotalBytes > 0 })
	if err := e.SetUploadLimit(64 << 10); err != nil || e.limiter.Limit() != rate.Limit(64<<10) {
		t.Fatalf("global limiter not updated: %v", err)
	}
	if err := e.SetUploadLimit(-1); err != nil || e.limiter.Limit() != rate.Inf {
		t.Fatalf("unlimited not applied: %v", err)
	}
}

func TestConcurrentEnsureAndClose(t *testing.T) {
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	task := Task{ID: "same", Magnet: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567", Folder: filepath.Join(root, "files")}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := e.Ensure(task); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if len(e.Snapshot()) != 1 {
		t.Fatal("concurrent Ensure duplicated task")
	}
	for range 4 {
		wg.Go(func() {
			if err := e.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if err := e.Ensure(task); !errors.Is(err, ErrClosed) {
		t.Fatalf("Ensure after Close: %v", err)
	}
}

func TestCorruptCheckpointFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tasks.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if e, err := newEngine(Config{DataDir: dir}, localConfig); err == nil {
		e.Close()
		t.Fatal("corrupt checkpoint silently reset")
	}
}

func ExampleEngine_Ensure() {
	// Hash is core's stable BT identity; Directory is its isolated resource folder.
	_ = Task{ID: "bt-content-hash", Magnet: "magnet:?xt=urn:btih:...", Folder: "isolated-resource-folder", Context: context.Background()}
	fmt.Println("map core Hash, Resource, Directory and Context into downloader.Task")
	// Output: map core Hash, Resource, Directory and Context into downloader.Task
}

var _ io.Closer = (*Engine)(nil)

type memorySeed struct{ data []byte }

func (s memorySeed) OpenTorrent(_ context.Context, _ *metainfo.Info, _ metainfo.Hash) (storage.TorrentImpl, error) {
	return storage.TorrentImpl{Piece: func(p metainfo.Piece) storage.PieceImpl {
		return seedPiece{bytes.NewReader(s.data[p.Offset() : p.Offset()+p.Length()])}
	}, Close: func() error { return nil }}, nil
}

type seedPiece struct{ *bytes.Reader }

func (seedPiece) WriteAt([]byte, int64) (int, error) { return 0, errors.New("read-only fixture") }
func (seedPiece) MarkComplete() error                { return nil }
func (seedPiece) MarkNotComplete() error             { return nil }
func (seedPiece) Completion() storage.Completion     { return storage.Completion{Ok: true, Complete: true} }
