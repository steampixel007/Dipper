// Package trackers keeps a fresh copy of a curated public tracker list for
// the download engine. The list is advisory: every failure falls back to the
// cached copy, and the engine's built-in trackers cover the first run.
package trackers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"dipper.local/dipper/internal/core"
)

const (
	// DefaultURL is the primary source, a CDN mirror of the upstream raw
	// file so the list stays reachable where GitHub itself is slow.
	DefaultURL = "https://cdn.gh-proxy.org/https://raw.githubusercontent.com/XIU2/TrackersListCollection/master/best.txt"
	// FallbackURL is the upstream raw file.
	FallbackURL = "https://raw.githubusercontent.com/XIU2/TrackersListCollection/master/best.txt"
	// MaxBytes bounds one response; a bigger body means the source is not
	// the tracker list this package expects.
	MaxBytes = 64 << 10
	// MaxTrackers rejects a polluted response wholesale; the real list stays
	// far below it.
	MaxTrackers = 200
	// RefreshAfter is how long a saved list stays trusted.
	RefreshAfter = 24 * time.Hour
)

// List is the most recent good remote tracker list.
type List struct {
	URLs      []string  `json:"urls"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Load reads the cached list. A missing or unreadable cache is an empty
// list: the built-in trackers carry the first run.
func Load(path string) List {
	b, err := os.ReadFile(path)
	if err != nil {
		return List{}
	}
	var list List
	if json.Unmarshal(b, &list) != nil {
		return List{}
	}
	return list
}

// Save stores the list for the next start.
func (l List) Save(path string) error {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// Stale reports whether the list is old enough to refresh.
func (l List) Stale(now time.Time) bool {
	return l.FetchedAt.IsZero() || now.Sub(l.FetchedAt) >= RefreshAfter
}

// Fetcher downloads one URL and returns its body.
type Fetcher func(ctx context.Context, url string) ([]byte, error)

// Parse validates a fetched list body wholesale: every non-blank line must
// be an announce URL and the response must stay within bounds. One bad line
// rejects the response instead of trusting a polluted list.
func Parse(body []byte) ([]string, error) {
	if len(body) > MaxBytes {
		return nil, fmt.Errorf("tracker list is %d bytes, over the %d byte limit", len(body), MaxBytes)
	}
	if err := core.ValidateTrackers(string(body)); err != nil {
		return nil, fmt.Errorf("tracker list has an invalid line: %w", err)
	}
	urls := core.ParseTrackers(string(body))
	if len(urls) == 0 {
		return nil, fmt.Errorf("tracker list is empty")
	}
	if len(urls) > MaxTrackers {
		return nil, fmt.Errorf("tracker list has %d entries, over the %d limit", len(urls), MaxTrackers)
	}
	return urls, nil
}

// Refresh fetches the list when the cache is stale, trying each source URL
// in order, and stores the first good response. It returns the list now
// current: the old one when every source failed.
func Refresh(ctx context.Context, path string, now time.Time, fetch Fetcher) (List, error) {
	current := Load(path)
	if !current.Stale(now) {
		return current, nil
	}
	var lastErr error
	for _, url := range []string{DefaultURL, FallbackURL} {
		if ctx.Err() != nil {
			return current, ctx.Err()
		}
		body, err := fetch(ctx, url)
		if err != nil {
			lastErr = err
			continue
		}
		urls, err := Parse(body)
		if err != nil {
			lastErr = err
			continue
		}
		fresh := List{URLs: urls, FetchedAt: now}
		if err = fresh.Save(path); err != nil {
			return fresh, fmt.Errorf("save tracker list: %w", err)
		}
		return fresh, nil
	}
	return current, lastErr
}

// FetchHTTP downloads one URL with a bounded response.
func FetchHTTP(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker list source returned %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
}
