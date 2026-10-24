package source

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
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
