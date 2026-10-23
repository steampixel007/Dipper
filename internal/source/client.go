// Package source reads the public Mikan catalog, episode history and RSS feeds.
package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Anime struct {
	ID, Title, Cover, Summary, Day string
}

type Group struct {
	ID, Name string
	// Count is -1 when the detail page only contains a truncated preview.
	Count int
}

type Resource struct {
	// ID is the lowercase source episode ID, not a verified torrent infohash.
	// Published retains the source's timezone-less text. RSS Magnet is empty;
	// HTML history preserves the magnet explicitly supplied by the source.
	ID, Title, Published, Magnet, TorrentURL string
	// Size is bytes (HTML sizes are approximate), or zero when the source omits it.
	Size int64
}

type Detail struct {
	Anime  Anime
	Groups []Group
}

var (
	ErrStructure         = errors.New("mikan: unrecognized or inconsistent response structure")
	ErrChallenge         = errors.New("mikan: access challenge")
	ErrIncompleteHistory = errors.New("mikan: cannot confirm complete history")
	ErrNotFound          = errors.New("mikan: anime or subtitle group not found")
	ErrResponseTooLarge  = errors.New("mikan: response exceeds size limit")
)

// HTTPError exposes failures, including a Retry-After hint, without retrying challenges.
type HTTPError struct {
	StatusCode int
	URL        string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("mikan: HTTP %d from %s", e.StatusCode, e.URL)
}

// Client is safe for concurrent use. Requests share a cancellable rate limit.
// Reuse one Client for catalog, subscriptions and covers to preserve that limit.
type Client struct {
	base        *url.URL
	initErr     error
	http        *http.Client
	gate        chan struct{}
	nextRequest time.Time
	interval    time.Duration
	timeout     time.Duration
	initialTake int
	maxTake     int
}

// New uses the public Mikan origin when baseURL is empty. Invalid origins surface
// as method errors, preserving a constructor that does not require error handling.
func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://mikanime.tv"
	}
	u, err := httpURL(baseURL)
	c := &Client{
		base: u, initErr: err,
		http: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("mikan: too many redirects")
				}
				_, err := httpURL(req.URL.String())
				return err
			},
		},
		gate: make(chan struct{}, 1), interval: 1500 * time.Millisecond,
		timeout: 30 * time.Second, initialTake: 1000, maxTake: 128000,
	}
	return c
}

func httpURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("mikan: expected an absolute HTTP(S) URL")
	}
	return u, nil
}

func (c *Client) endpoint(path string, query url.Values) (string, error) {
	if c.initErr != nil {
		return "", c.initErr
	}
	u := c.base.ResolveReference(&url.URL{Path: path, RawQuery: query.Encode()})
	return u.String(), nil
}

func (c *Client) get(ctx context.Context, rawURL string, limit int64) ([]byte, *url.URL, error) {
	if c.initErr != nil {
		return nil, nil, c.initErr
	}
	if _, err := httpURL(rawURL); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	if delay := time.Until(c.nextRequest); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Dipper/0.1 (Mikan public source reader)")
	req.Header.Set("Accept", "text/html,application/rss+xml,application/xml,image/*;q=0.9,*/*;q=0.5")
	c.nextRequest = time.Now().Add(c.interval)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("mikan: GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		if until := time.Now().Add(retryAfter); until.After(c.nextRequest) {
			c.nextRequest = until
		}
		return nil, nil, &HTTPError{StatusCode: resp.StatusCode, URL: resp.Request.URL.String(), RetryAfter: retryAfter}
	}
	if resp.ContentLength > limit {
		return nil, nil, ErrResponseTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, fmt.Errorf("mikan: read response: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, nil, ErrResponseTooLarge
	}
	if challenge(body) {
		return nil, nil, ErrChallenge
	}
	return body, resp.Request.URL, nil
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if until, err := http.ParseTime(value); err == nil && until.After(now) {
		return until.Sub(now)
	}
	return 0
}

func challenge(body []byte) bool {
	// Cloudflare analytics is present on ordinary pages and is not a challenge.
	lower := bytes.ToLower(body)
	for _, marker := range []string{"/cdn-cgi/challenge-platform/", "cf-chl-", "verify you are human", "<title>just a moment", "<title>access denied", "<title>attention required", "id=\"captcha\"", "class=\"g-recaptcha\"", "人机验证"} {
		if bytes.Contains(lower, []byte(marker)) {
			return true
		}
	}
	return false
}

// Cover returns encoded image bytes; decoding and caching belong to the caller.
// Image requests and redirects accept only HTTP(S). Bodies are limited to 5 MiB.
func (c *Client) Cover(ctx context.Context, imageURL string) ([]byte, error) {
	body, _, err := c.get(ctx, imageURL, 5<<20)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(http.DetectContentType(body), "image/") {
		return nil, fmt.Errorf("%w: expected an image", ErrStructure)
	}
	return body, nil
}
