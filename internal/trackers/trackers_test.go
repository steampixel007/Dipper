package trackers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseKeepsTheWholeGoodList(t *testing.T) {
	body := "udp://a.example.com:1337/announce\n\nhttp://b.example.com/announce\nudp://a.example.com:1337/announce\n"
	urls, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 2 || urls[0] != "udp://a.example.com:1337/announce" || urls[1] != "http://b.example.com/announce" {
		t.Fatalf("Parse = %v", urls)
	}
}

func TestParseRejectsOneBadLineWholesale(t *testing.T) {
	urls, err := Parse([]byte("udp://ok.example.com:1/announce\nftp://nope/announce"))
	if err == nil || urls != nil {
		t.Fatalf("Parse = %v, %v, want a wholesale rejection", urls, err)
	}
}

func TestParseRejectsEmptyOversizedAndTooLongResponses(t *testing.T) {
	if _, err := Parse(nil); err == nil {
		t.Fatal("an empty response was accepted")
	}
	if _, err := Parse([]byte(strings.Repeat("a", MaxBytes+1))); err == nil {
		t.Fatal("an oversized response was accepted")
	}
	var many strings.Builder
	for i := 0; i <= MaxTrackers; i++ {
		many.WriteString("udp://many-" + strconv.Itoa(i) + ".example.com:1337/announce\n")
	}
	if _, err := Parse([]byte(many.String())); err == nil {
		t.Fatal("a too long response was accepted")
	}
}

func TestListStale(t *testing.T) {
	now := time.Now()
	if !(List{}).Stale(now) {
		t.Fatal("a missing list must count as stale")
	}
	fresh := List{FetchedAt: now}
	if fresh.Stale(now) || fresh.Stale(now.Add(RefreshAfter-time.Minute)) {
		t.Fatal("a fresh list must not count as stale")
	}
	if !fresh.Stale(now.Add(RefreshAfter)) {
		t.Fatal("a list at the refresh age must count as stale")
	}
}

func TestLoadMissingAndCorruptCachesAreEmpty(t *testing.T) {
	if list := Load(filepath.Join(t.TempDir(), "absent.json")); len(list.URLs) != 0 {
		t.Fatalf("Load = %v, want an empty list", list.URLs)
	}
	path := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if list := Load(path); len(list.URLs) != 0 {
		t.Fatalf("Load = %v, want an empty list", list.URLs)
	}
}

func TestRefreshTriesTheFallbackWhenTheMirrorFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.json")
	var tried []string
	fetch := func(_ context.Context, url string) ([]byte, error) {
		tried = append(tried, url)
		if url == DefaultURL {
			return nil, errors.New("mirror down")
		}
		return []byte("udp://good.example.com:1337/announce\n"), nil
	}
	list, err := Refresh(context.Background(), path, time.Now(), fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(tried) != 2 || tried[0] != DefaultURL || tried[1] != FallbackURL {
		t.Fatalf("tried %v", tried)
	}
	if len(list.URLs) != 1 || list.URLs[0] != "udp://good.example.com:1337/announce" {
		t.Fatalf("Refresh = %v", list.URLs)
	}
	if again := Load(path); len(again.URLs) != 1 {
		t.Fatalf("cache was not saved: %v", again)
	}
}

func TestRefreshSkipsAFreshCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.json")
	now := time.Now()
	fetch := func(context.Context, string) ([]byte, error) {
		t.Fatal("a fresh cache must not be fetched")
		return nil, nil
	}
	if err := (List{URLs: []string{"udp://cached.example.com:1/announce"}, FetchedAt: now}).Save(path); err != nil {
		t.Fatal(err)
	}
	list, err := Refresh(context.Background(), path, now.Add(time.Minute), fetch)
	if err != nil || len(list.URLs) != 1 {
		t.Fatalf("Refresh = %v, %v", list.URLs, err)
	}
}

func TestRefreshKeepsTheOldListWhenEverySourceFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.json")
	old := time.Now().Add(-2 * RefreshAfter)
	if err := (List{URLs: []string{"udp://old.example.com:1/announce"}, FetchedAt: old}).Save(path); err != nil {
		t.Fatal(err)
	}
	fetch := func(context.Context, string) ([]byte, error) {
		return nil, errors.New("offline")
	}
	list, err := Refresh(context.Background(), path, time.Now(), fetch)
	if err == nil {
		t.Fatal("a total failure must be reported")
	}
	if len(list.URLs) != 1 || list.URLs[0] != "udp://old.example.com:1/announce" {
		t.Fatalf("Refresh = %v, want the cached list", list.URLs)
	}
}

func TestRefreshStopsWhenTheContextIsCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fetch := func(context.Context, string) ([]byte, error) {
		t.Fatal("a cancelled refresh must not fetch")
		return nil, nil
	}
	if _, err := Refresh(ctx, path, time.Now(), fetch); !errors.Is(err, context.Canceled) {
		t.Fatalf("Refresh = %v, want context.Canceled", err)
	}
}

func TestParseAcceptsARealShapedListBody(t *testing.T) {
	body := "udp://tracker.opentrackr.org:1337/announce\nhttps://tracker.tamersunion.org:443/announce\nudp://open.demonii.com:1337/announce\n"
	urls, err := Parse([]byte(body))
	if err != nil || len(urls) != 3 {
		t.Fatalf("Parse = %v, %v", urls, err)
	}
}
