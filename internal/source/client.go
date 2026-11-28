// Package source reads the public Mikan catalog, episode history and RSS feeds.
package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/steampixel007/Dipper/internal/proxy"
)

type Anime struct {
	ID, Title, Cover, Summary, Day string
}

type Group struct {
	ID, Name string
	// Count is -1 when the detail page only contains a truncated preview.
	Count int
	// Preview holds the resources parsed from the detail page table. When Count
	// is nonnegative it is the group's complete history; when Count is -1 it is
	// a truncated preview and must not be treated as complete history.
	Preview []Resource
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

// Client is safe for concurrent use. Reuse one Client to share transport
// admission across callers while pacing and server-requested cooldowns stay
// isolated per origin.
type Client struct {
	mu            sync.RWMutex
	base          *url.URL
	initErr       error
	http          *http.Client
	gate          chan struct{}
	coverGate     chan struct{}
	rateMu        sync.Mutex
	rates         map[string]*originRate
	interval      time.Duration
	coverInterval time.Duration
	timeout       time.Duration
	initialTake   int
	maxTake       int
}

type originRate struct {
	retryUntil   time.Time
	nextDocument time.Time
	coverRefill  time.Time
}

// DefaultOrigin is the public Mikan origin used unless the user picks a route.
const DefaultOrigin = "https://mikanani.me"

const MaxConcurrentCovers = 6

// New uses DefaultOrigin when baseURL is empty. Invalid origins surface as
// method errors, preserving a constructor that does not require error handling.
func New(baseURL string) *Client {
	return newClient(baseURL, proxy.Func)
}

func newClient(baseURL string, resolveProxy func(*http.Request) (*url.URL, error)) *Client {
	if baseURL == "" {
		baseURL = DefaultOrigin
	}
	u, err := httpURL(baseURL)
	c := &Client{
		base: u, initErr: err,
		http: &http.Client{
			Transport: &http.Transport{
				Proxy:               resolveProxy,
				ForceAttemptHTTP2:   true,
				MaxIdleConns:        2 * MaxConcurrentCovers,
				MaxIdleConnsPerHost: MaxConcurrentCovers + 2,
				IdleConnTimeout:     90 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("mikan: too many redirects")
				}
				_, err := httpURL(req.URL.String())
				return err
			},
		},
		gate: make(chan struct{}, 1), coverGate: make(chan struct{}, MaxConcurrentCovers),
		rates:    make(map[string]*originRate),
		interval: 1500 * time.Millisecond, coverInterval: 750 * time.Millisecond,
		timeout: 30 * time.Second, initialTake: 1000, maxTake: 128000,
	}
	return c
}

func (c *Client) target() (*url.URL, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.base, c.initErr
}

// Retarget points the client at another origin. Transport admission stays
// shared so switching cannot multiply concurrency, and each origin keeps its
// own pacing and cooldown state; a request already queued for the old origin
// keeps that origin's state until the caller cancels it.
func (c *Client) Retarget(baseURL string) error {
	u, err := httpURL(baseURL)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.base, c.initErr = u, nil
	c.mu.Unlock()
	return nil
}

// Origin reports the origin in use; empty when construction failed.
func (c *Client) Origin() string {
	u, err := c.target()
	if err != nil || u == nil {
		return ""
	}
	return u.String()
}

func httpURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("mikan: expected an absolute HTTP(S) URL")
	}
	return u, nil
}

func originKey(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return scheme + "://" + host
}

func (c *Client) rateFor(u *url.URL) *originRate {
	key := originKey(u)
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	rate := c.rates[key]
	if rate == nil {
		rate = &originRate{}
		c.rates[key] = rate
	}
	return rate
}

func (c *Client) currentRate() *originRate {
	u, err := c.target()
	if err != nil || u == nil {
		return &originRate{}
	}
	return c.rateFor(u)
}

func (c *Client) endpoint(path string, query url.Values) (string, error) {
	base, err := c.target()
	if err != nil {
		return "", err
	}
	u := base.ResolveReference(&url.URL{Path: path, RawQuery: query.Encode()})
	return u.String(), nil
}

func (c *Client) get(ctx context.Context, rawURL string, limit int64) ([]byte, *url.URL, error) {
	return c.getWithStart(ctx, rawURL, limit, false, nil)
}

func (c *Client) getWithStart(ctx context.Context, rawURL string, limit int64, cover bool, start func() bool) ([]byte, *url.URL, error) {
	if _, err := c.target(); err != nil {
		return nil, nil, err
	}
	u, err := httpURL(rawURL)
	if err != nil {
		return nil, nil, err
	}
	rate := c.rateFor(u)
	for attemptsLeft := 2; ; attemptsLeft-- {
		body, finalURL, err := c.getOnce(ctx, rate, rawURL, limit, cover, start)
		start = nil
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return body, finalURL, err
		}
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, nil, contextErr
		}
		if attemptsLeft == 1 {
			return body, finalURL, err
		}
	}
}

func (c *Client) waitForRate(ctx context.Context, rate *originRate, cover bool, start func() bool) error {
	for {
		c.rateMu.Lock()
		if err := ctx.Err(); err != nil {
			c.rateMu.Unlock()
			return err
		}
		next := rate.nextDocument
		if cover {
			next = rate.coverRefill.Add(-c.coverBurstWindow())
		}
		if rate.retryUntil.After(next) {
			next = rate.retryUntil
		}
		now := time.Now()
		delay := next.Sub(now)
		if delay <= 0 {
			if start != nil && !start() {
				c.rateMu.Unlock()
				return context.Canceled
			}
			if cover {
				if now.After(rate.coverRefill) {
					rate.coverRefill = now
				}
				rate.coverRefill = rate.coverRefill.Add(c.coverInterval)
			} else {
				rate.nextDocument = now.Add(c.interval)
			}
			c.rateMu.Unlock()
			return nil
		}
		c.rateMu.Unlock()
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

func (c *Client) coverBurstWindow() time.Duration {
	return (MaxConcurrentCovers - 1) * c.coverInterval
}

func (c *Client) deferRate(rate *originRate, now time.Time, retryAfter, coverBackoff time.Duration) {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	if until := now.Add(retryAfter); retryAfter > 0 && until.After(rate.retryUntil) {
		rate.retryUntil = until
	}
	if retryAfter > 0 || coverBackoff > 0 {
		resume := now.Add(coverBackoff)
		if rate.retryUntil.After(resume) {
			resume = rate.retryUntil
		}
		if refill := resume.Add(c.coverBurstWindow()); refill.After(rate.coverRefill) {
			rate.coverRefill = refill
		}
	}
}

func (c *Client) getOnce(ctx context.Context, rate *originRate, rawURL string, limit int64, cover bool, start func() bool) ([]byte, *url.URL, error) {
	if err := c.waitForRate(ctx, rate, cover, start); err != nil {
		return nil, nil, err
	}
	gate := c.gate
	if cover {
		gate = c.coverGate
	}
	select {
	case gate <- struct{}{}:
		defer func() { <-gate }()
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Dipper/0.1 (Mikan public source reader)")
	req.Header.Set("Accept", "text/html,application/rss+xml,application/xml,image/*;q=0.9,*/*;q=0.5")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("mikan: GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		now := time.Now()
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), now)
		status := &HTTPError{StatusCode: resp.StatusCode, URL: resp.Request.URL.String(), RetryAfter: retryAfter}
		var backoff time.Duration
		if cover {
			backoff = CoverRetryDelay(status)
		}
		c.deferRate(rate, now, retryAfter, backoff)
		return nil, nil, status
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
		if cover {
			c.deferRate(rate, time.Now(), 0, CoverRetryDelay(ErrChallenge))
		}
		return nil, nil, ErrChallenge
	}
	return body, resp.Request.URL, nil
}

func CoverRetryDelay(err error) time.Duration {
	var status *HTTPError
	if errors.As(err, &status) {
		if status.StatusCode == http.StatusTooManyRequests || status.StatusCode == http.StatusForbidden {
			return max(time.Minute, status.RetryAfter)
		}
		if status.StatusCode >= http.StatusInternalServerError {
			return max(2*time.Second, status.RetryAfter)
		}
		return status.RetryAfter
	}
	if errors.Is(err, ErrChallenge) {
		return time.Minute
	}
	return 0
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
	return c.CoverWhenReady(ctx, imageURL, nil)
}

// CoverWhenReady calls start after the shared limiter admits the first request.
// Returning false skips the request without spending a rate-limit interval.
// The callback must not block or call back into the client.
func (c *Client) CoverWhenReady(ctx context.Context, imageURL string, start func() bool) ([]byte, error) {
	body, _, err := c.getWithStart(ctx, OriginalCoverURL(imageURL), 5<<20, true, start)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(http.DetectContentType(body), "image/") {
		return nil, fmt.Errorf("%w: expected an image", ErrStructure)
	}
	return body, nil
}

// OriginalCoverURL removes the source's resize parameters. They crop portrait
// posters into square catalog thumbnails, so a smaller size is not a complete
// image.
func OriginalCoverURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := u.Query()
	query.Del("width")
	query.Del("height")
	u.RawQuery = query.Encode()
	return u.String()
}

// Probe checks the source origin the way a real request reaches it, rate
// limit and proxy included, without parsing the page it gets back.
func (c *Client) Probe(ctx context.Context) error {
	root, err := c.endpoint("/", nil)
	if err != nil {
		return err
	}
	_, _, err = c.get(ctx, root, 1<<20)
	return err
}

func (c *Client) ProbeWithProxy(ctx context.Context, mode, address string) error {
	origin, err := c.target()
	if err != nil {
		return err
	}
	probe := newClient(origin.String(), proxy.ForSettings(mode, address))
	defer probe.http.CloseIdleConnections()
	return probe.Probe(ctx)
}
