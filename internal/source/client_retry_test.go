package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientRetriesInterruptedResponse(t *testing.T) {
	body := fixture(t, "rss-4063-370.xml")
	for _, phase := range []string{"headers", "body"} {
		for _, recover := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/recover=%t", phase, recover), func(t *testing.T) {
				var requests atomic.Int32
				client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					attempt := requests.Add(1)
					if attempt == 1 || !recover {
						if phase == "body" {
							w.Header().Set("Content-Length", strconv.Itoa(len(body)))
							w.Write(body[:len(body)/2])
							w.(http.Flusher).Flush()
						}
						connection, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						connection.Close()
						return
					}
					w.Write(body)
				})
				client.http.Transport.(*http.Transport).DisableKeepAlives = true
				items, err := client.Feed(t.Context(), "4063", "370")
				if recover {
					if err != nil || len(items) != 3 {
						t.Fatalf("interrupted %s did not recover: items=%d err=%v", phase, len(items), err)
					}
				} else if items != nil || (!errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF)) {
					t.Fatalf("persistent interruption must remain a failure: items=%v err=%v", items, err)
				}
				if requests.Load() != 2 {
					t.Fatalf("requests=%d, want exactly two attempts", requests.Load())
				}
			})
		}
	}
}

func TestClientInterruptedRetryRespectsCancellationAndRateLimit(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancelRequest), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var requests atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if cancelRequest {
					cancel()
				}
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				connection.Close()
			})
			client.http.Transport.(*http.Transport).DisableKeepAlives = true
			client.interval = time.Minute
			client.timeout = time.Second
			items, err := client.Feed(ctx, "4063", "370")
			expected := context.DeadlineExceeded
			if cancelRequest {
				expected = context.Canceled
			}
			if items != nil || !errors.Is(err, expected) || requests.Load() != 1 {
				t.Fatalf("retry ignored cancellation or rate limit: items=%v requests=%d err=%v", items, requests.Load(), err)
			}
		})
	}
}

func TestClientDoesNotRetryCompleteInvalidResponses(t *testing.T) {
	for _, response := range []struct {
		name   string
		status int
		body   string
	}{
		{"challenge", 200, "<html><title>Just a moment...</title></html>"},
		{"malformed", 200, "<rss>"},
		{"forbidden", 403, ""},
		{"rate-limited", 429, ""},
		{"unavailable", 503, ""},
	} {
		t.Run(response.name, func(t *testing.T) {
			var requests atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(response.status)
				io.WriteString(w, response.body)
			})
			items, err := client.Feed(t.Context(), "4063", "370")
			if items != nil || err == nil || requests.Load() != 1 {
				t.Fatalf("complete invalid response was retried or accepted: items=%v requests=%d err=%v", items, requests.Load(), err)
			}
		})
	}
}
