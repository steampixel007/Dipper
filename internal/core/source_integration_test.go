package core

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"dipper.local/dipper/internal/source"
)

func TestRealSourceFeedResolvesTorrentIdentity(t *testing.T) {
	sourceID := strings.Repeat("a", 40)
	info := []byte("d6:lengthi1e4:name5:a.mkve")
	hash := sha1.Sum(info)
	expected := hex.EncodeToString(hash[:])
	var torrentRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/RSS/Bangumi":
			w.Header().Set("Content-Type", "application/rss+xml")
			fmt.Fprintf(w, `<rss version="2.0"><channel><title>本地番剧字幕组</title><link>/RSS/Bangumi?bangumiId=1&amp;subgroupid=2</link><item><title>本地番剧 - 01 [1080p]</title><link>/Home/Episode/%s</link><pubDate>2026-10-23 08:00:00</pubDate><enclosure url="/Download/20261023/%s.torrent" length="1" type="application/x-bittorrent" /></item></channel></rss>`, sourceID, sourceID)
		case strings.HasSuffix(r.URL.Path, ".torrent"):
			torrentRequests.Add(1)
			w.Write(append(append([]byte("d4:info"), info...), 'e'))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := source.New(server.URL)
	feed := func(ctx context.Context, a, g string) ([]Resource, error) {
		resources, err := client.Feed(ctx, a, g)
		if err != nil {
			return nil, err
		}
		out := make([]Resource, len(resources))
		for i, r := range resources {
			out[i] = Resource{ID: r.ID, Title: r.Title, Published: r.Published, Size: r.Size, Magnet: r.Magnet, TorrentURL: r.TorrentURL}
		}
		return out, nil
	}
	d := &fakeDownloader{}
	c := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Feed: feed, Downloader: d})
	sub := create(t, c, Anime{ID: "1", Title: "本地番剧"}, Group{ID: "2", Name: "字幕组"}, Rule{}, nil, nil)
	if err := c.Check(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	tasks, err := c.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Task.Hash != expected || tasks[0].Task.Hash == sourceID || torrentRequests.Load() != 1 {
		t.Fatal(tasks, torrentRequests.Load())
	}
	if item := current(t, c, sub.ID).Items[sourceID]; item.Hash != expected || !item.Read {
		t.Fatal(item)
	}
}
