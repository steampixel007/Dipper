package source

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestRetargetRejectsInvalidOrigin(t *testing.T) {
	c := New("")
	before := c.Origin()
	for _, raw := range []string{"", "ftp://example.com", "/relative", "file:///etc"} {
		if err := c.Retarget(raw); err == nil {
			t.Fatalf("Retarget(%q) accepted", raw)
		}
		if got := c.Origin(); got != before {
			t.Fatalf("failed Retarget(%q) changed origin to %q", raw, got)
		}
	}
}

func TestRetargetConcurrentWithRequests(t *testing.T) {
	// The catalog markup is the smallest shape Catalog accepts; requests run
	// against one test origin while Retarget flips between two spellings of it,
	// so endpoint resolution and mutation race only through the client lock.
	markup := []byte(`<div class="sk-bangumi" data-dayofweek="1"><ul class="an-ul"><li><span data-bangumiid="1" data-src="/cover.png"></span><a class="an-text" href="/Home/Bangumi/1">节目</a></li></ul></div>`)
	var plain, trailing atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(markup) }))
	defer server.Close()
	plain.Store(server.URL)
	trailing.Store(server.URL + "/")
	c := New(server.URL)
	c.interval = 0
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if i%4 == 0 {
					_ = c.Retarget(plain.Load().(string))
				} else {
					_ = c.Retarget(trailing.Load().(string))
				}
			} else {
				_, _ = c.Catalog(t.Context(), 2026, "春")
			}
		}(i)
	}
	wg.Wait()
}

func TestRetargetIsolatesCooldownWithQueuedRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const oldOrigin = "http://127.0.0.1:8001"
		const newOrigin = "http://127.0.0.1:8002"
		client := clockedClient(oldOrigin, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Host == "127.0.0.1:8001" {
				w.Header().Set("Retry-After", "90")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		var status *HTTPError
		if _, err := client.Cover(t.Context(), oldOrigin+"/poster"); !errors.As(err, &status) || status.RetryAfter != 90*time.Second {
			t.Fatalf("old cover did not establish cooldown: %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		queued := make(chan error, 1)
		go func() { queued <- client.Probe(ctx) }()
		synctest.Wait()
		if err := client.Retarget(newOrigin); err != nil {
			t.Fatal(err)
		}
		begin := time.Now()
		if err := client.Probe(t.Context()); err != nil || time.Since(begin) != 0 {
			t.Fatalf("new origin probe waited %v: %v", time.Since(begin), err)
		}
		cancel()
		if err := <-queued; !errors.Is(err, context.Canceled) {
			t.Fatalf("old queued request = %v, want caller cancellation", err)
		}
	})
}

func TestRetargetPreservesPreviouslyUsedDestinationCooldown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const original = "http://127.0.0.1:8001"
		client := clockedClient(original, cooldownOnce("90"))
		assertProbeCooldown(t, client)
		for _, target := range []string{"http://127.0.0.1:8002", original} {
			if err := client.Retarget(target); err != nil {
				t.Fatal(err)
			}
		}
		begin := time.Now()
		if err := client.Probe(t.Context()); err != nil || time.Since(begin) != 90*time.Second {
			t.Fatalf("return to previous origin waited %v: %v", time.Since(begin), err)
		}
	})
}

func TestRetargetPreservesPacingForEquivalentAndInvalidOrigins(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const origin = "http://localhost"
		client := clockedClient(origin, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		if err := client.Probe(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := client.Retarget("http://LOCALHOST:80/"); err != nil {
			t.Fatal(err)
		}
		begin := time.Now()
		if err := client.Probe(t.Context()); err != nil || time.Since(begin) != 1500*time.Millisecond {
			t.Fatalf("equivalent origin probe waited %v: %v", time.Since(begin), err)
		}
		if err := client.Retarget("file:///invalid"); err == nil {
			t.Fatal("invalid Retarget succeeded")
		}
		begin = time.Now()
		if err := client.Probe(t.Context()); err != nil || time.Since(begin) != 1500*time.Millisecond {
			t.Fatalf("probe after invalid Retarget waited %v: %v", time.Since(begin), err)
		}
	})
}
