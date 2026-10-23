package downloader

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRealSharedUploadBudgetAndCloseStopsPayload(t *testing.T) {
	fixtures := []*fixture{newFixture(t, 512<<10), newFixture(t, (512<<10)+1)}
	metainfo := make([][]byte, len(fixtures))
	for i, f := range fixtures {
		var b bytes.Buffer
		if err := f.mi.Write(&b); err != nil {
			t.Fatal(err)
		}
		metainfo[i] = b.Bytes()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/0" {
			w.Write(metainfo[0])
		} else {
			w.Write(metainfo[1])
		}
	}))
	defer server.Close()
	root := t.TempDir()
	const limit = 128 << 10
	source := testEngine(t, Config{DataDir: filepath.Join(root, "source-state"), UploadLimit: limit})
	consumer := testEngine(t, Config{DataDir: filepath.Join(root, "consumer-state")})
	for i, f := range fixtures {
		folder := filepath.Join(root, fmt.Sprintf("import-%d", i))
		for name, b := range f.files {
			path := filepath.Join(folder, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		id := f.mi.HashInfoBytes().HexString()
		if err := source.Ensure(Task{ID: id, TorrentURL: fmt.Sprintf("%s/%d", server.URL, i), Folder: folder}); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "imported files verified", func() bool {
		s := source.Snapshot()
		return len(s) == 2 && s[0].Complete && s[1].Complete
	})
	for i, f := range fixtures {
		id := f.mi.HashInfoBytes().HexString()
		magnet := "magnet:?xt=urn:btih:" + id + "&x.pe=" + url.QueryEscape(source.client.ListenAddrs()[0].String())
		if err := consumer.Ensure(Task{ID: id, Magnet: magnet, Folder: filepath.Join(root, fmt.Sprintf("consume-%d", i)), Context: context.Background()}); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "both resources upload through one client", func() bool {
		s := source.Snapshot()
		return s[0].UploadedBytes > 0 && s[1].UploadedBytes > 0
	})
	before := source.client.Stats()
	start := time.Now()
	time.Sleep(2 * time.Second)
	after := source.client.Stats()
	elapsed := time.Since(start).Seconds()
	delta := after.BytesWrittenData.Int64() - before.BytesWrittenData.Int64()
	if delta < limit/2 || float64(delta) > float64(limit)*elapsed+64<<10 {
		t.Fatalf("shared upload budget: sent %d bytes over %.2fs with %d B/s limit", delta, elapsed, limit)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	stats := consumer.client.Stats()
	baseline := stats.BytesReadData.Int64()
	time.Sleep(550 * time.Millisecond)
	stats = consumer.client.Stats()
	if extra := stats.BytesReadData.Int64() - baseline; extra != 0 {
		t.Fatalf("Close left payload transmission running: %d", extra)
	}
	for _, s := range source.Snapshot() {
		if s.Paused || s.ManuallyStopped || s.UploadSpeed != 0 || s.DownloadSpeed != 0 {
			t.Fatalf("Close changed user intent or left nonzero speed: %+v", s)
		}
	}
	t.Logf("shared source payload %d B over %.2fs (limit %d B/s); zero payload after Close", delta, elapsed, limit)
}
