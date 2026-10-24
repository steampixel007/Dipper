package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"

	"dipper.local/dipper/internal/proxy"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := New(server.URL)
	c.interval = 0
	return c
}

func TestClientSendsRequestsThroughConfiguredProxy(t *testing.T) {
	// The loopback guard keeps local test servers direct, so the wiring is
	// asserted through the transport's proxy function rather than by
	// sending a request to an address beyond this machine.
	c := New("")
	transport, ok := c.http.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client transport = %T, want *http.Transport", c.http.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("source client transport has no proxy function")
	}
	proxy.Configure(proxy.ModeCustom, "192.0.2.10:7897")
	t.Cleanup(func() { proxy.Configure(proxy.ModeAuto, "") })
	req, err := http.NewRequest(http.MethodGet, "https://mikanime.tv/", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := transport.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.String() != "http://192.0.2.10:7897" {
		t.Fatalf("proxy = %v, want the configured address", got)
	}
}

func TestCatalogRealSample(t *testing.T) {
	body := fixture(t, "catalog-source.html")
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Home/BangumiCoverFlowByDayOfWeek" || r.URL.Query().Get("year") != "2026" || r.URL.Query().Get("seasonStr") != "秋" {
			t.Errorf("unexpected catalog URL: %s", r.URL)
		}
		w.Write(body)
	})
	items, err := c.Catalog(t.Context(), 2026, "秋")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 83 {
		t.Fatalf("catalog has %d entries, want all 83 including unreleased titles", len(items))
	}
	if a := items[0]; a.ID != "4031" || a.Title != "天是红河岸" || a.Day != "2" || !strings.HasPrefix(a.Cover, c.base.String()+"/images/") {
		t.Fatalf("first anime: %+v", a)
	}
	var unreleased, movie bool
	for _, a := range items {
		if a.Day != "" && (len(a.Day) != 1 || a.Day[0] < '0' || a.Day[0] > '6') {
			t.Errorf("invalid UI day: %+v", a)
		}
		if a.ID == "3975" {
			unreleased = strings.Contains(a.Title, "角落小伙伴") && a.Day == ""
		}
		if a.ID == "4122" {
			movie = a.Day == ""
		}
	}
	if !unreleased || !movie {
		t.Fatal("unreleased/movie cards were lost")
	}
}

func TestCatalogDeduplicatesAcrossDays(t *testing.T) {
	body := `<div class="sk-bangumi" data-dayofweek="0"><ul class="an-ul"><li><span data-bangumiid="1" data-src="/cover.png"></span><a class="an-text" href="/Home/Bangumi/1">节目</a></li></ul></div>`
	body += strings.Replace(body, `data-dayofweek="0"`, `data-dayofweek="3"`, 1)
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
	items, err := c.Catalog(t.Context(), 2025, "winter")
	if err != nil || len(items) != 1 || items[0].Day != "0" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

func TestDetailRealSamples(t *testing.T) {
	for _, id := range []string{"4063", "227"} {
		t.Run(id, func(t *testing.T) {
			body := fixture(t, "bangumi-"+id+".html")
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/Home/Bangumi/"+id {
					t.Errorf("wrong detail path %s", r.URL)
				}
				w.Write(body)
			})
			detail, err := c.Detail(t.Context(), id)
			if err != nil {
				t.Fatal(err)
			}
			if detail.Anime.ID != id || detail.Anime.Title == "" || detail.Anime.Summary != "" {
				t.Fatalf("unexpected anime: %+v", detail.Anime)
			}
			found := false
			for _, group := range detail.Groups {
				if id == "4063" && group.ID == "370" {
					found = group.Name == "LoliHouse" && group.Count == 3 && detail.Anime.Day == "1" && len(detail.Groups) == 4
				}
				if id == "227" && group.ID == "161" {
					found = group.Name == "银色子弹字幕组" && group.Count == -1 && detail.Anime.Day == "0"
				}
			}
			if !found {
				t.Fatalf("expected group/count/day absent: %+v", detail)
			}
		})
	}
}

func TestHistory1522AndRSS100(t *testing.T) {
	first := fixture(t, "probe-b227-g161-take1000.html")
	full := fixture(t, "probe-b227-g161-take1523.html")
	rss := fixture(t, "rss-227-161.xml")
	var takes []int
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Home/ExpandEpisodeTable":
			q := r.URL.Query()
			if q.Get("bangumiId") != "227" || q.Get("subtitleGroupId") != "161" || q.Get("offset") != "" || q.Get("page") != "" {
				t.Errorf("wrong history parameters: %s", r.URL)
			}
			take, _ := strconv.Atoi(q.Get("take"))
			takes = append(takes, take)
			if take == 1000 {
				w.Write(first)
			} else if take > 1522 {
				w.Write(full)
			} else {
				t.Errorf("unexpected take %d", take)
				w.WriteHeader(500)
			}
		case "/RSS/Bangumi":
			if r.URL.Query().Get("subgroupid") != "161" {
				t.Error("missing RSS subgroupid")
			}
			w.Write(rss)
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(404)
		}
	})
	history, err := c.History(t.Context(), "227", "161")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1522 || fmt.Sprint(takes) != "[1000 2000]" {
		t.Fatalf("history=%d takes=%v", len(history), takes)
	}
	if history[0].ID != "3039d91aff2ab0dd0f2946df88e4aa3499050b88" || history[0].Published != "2026/10/18 01:13" || history[0].Size != 380003942 {
		t.Fatalf("unexpected first episode %+v", history[0])
	}
	if !strings.HasPrefix(history[1521].Published, "2015/06/13") {
		t.Fatalf("oldest episode lost: %+v", history[1521])
	}
	feed, err := c.Feed(t.Context(), "227", "161")
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 100 {
		t.Fatalf("RSS count %d, want 100", len(feed))
	}
	for i, item := range feed {
		if item.ID != history[i].ID || item.Magnet != "" || item.TorrentURL == "" || history[i].Magnet == "" || item.Size <= 0 || strings.HasSuffix(item.Published, "Z") {
			t.Fatalf("RSS resource %d mismatch: %+v", i, item)
		}
	}
}

func TestHistoryExactTakeMustRequestMore(t *testing.T) {
	full := fixture(t, "probe-b227-g161-take1522.html")
	requests := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Write(full)
	})
	c.initialTake = 1522
	items, err := c.History(t.Context(), "227", "161")
	if err != nil || len(items) != 1522 || requests != 2 {
		t.Fatalf("len=%d requests=%d err=%v", len(items), requests, err)
	}
	c.maxTake = 1522
	items, err = c.History(t.Context(), "227", "161")
	if !errors.Is(err, ErrIncompleteHistory) || items != nil {
		t.Fatalf("limit must not return partial history: len=%d err=%v", len(items), err)
	}
}

func TestShortHistoryAndRSS(t *testing.T) {
	html := fixture(t, "probe-b4063-g370-take65.html")
	rss := fixture(t, "rss-4063-370.xml")
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/RSS/Bangumi" {
			w.Write(rss)
		} else {
			w.Write(html)
		}
	})
	history, err := c.History(t.Context(), "4063", "370")
	if err != nil {
		t.Fatal(err)
	}
	feed, err := c.Feed(t.Context(), "4063", "370")
	if err != nil || len(history) != 3 || len(feed) != 3 {
		t.Fatalf("history=%d feed=%d err=%v", len(history), len(feed), err)
	}
	if feed[0].Size != 512124512 || feed[0].Published != "2026-10-20T14:19:00" {
		t.Fatalf("bad RSS extension parsing: %+v", feed[0])
	}
	for i := range history {
		if history[i].ID != feed[i].ID || !strings.HasPrefix(history[i].TorrentURL, c.base.String()+"/Download/") {
			t.Fatalf("short history mismatch: %+v vs %+v", history[i], feed[i])
		}
	}
}

func TestEmptyHistoryIsVerified(t *testing.T) {
	empty := fixture(t, "probe-b227-g999999-take65.html")
	detail := fixture(t, "bangumi-227.html")
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/Home/Bangumi/") {
			w.Write(detail)
		} else {
			w.Write(empty)
		}
	})
	items, err := c.History(t.Context(), "227", "999999")
	if !errors.Is(err, ErrNotFound) || items != nil {
		t.Fatalf("unknown group: items=%v err=%v", items, err)
	}
	items, err = c.History(t.Context(), "227", "161")
	if !errors.Is(err, ErrIncompleteHistory) || items != nil {
		t.Fatalf("false empty group: items=%v err=%v", items, err)
	}
}

func TestHistoryRejectsRegressionAndDuplicates(t *testing.T) {
	body := fixture(t, "probe-b4063-g370-take65.html")
	for _, mode := range []string{"regression", "duplicate", "malformed", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			first, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			first.Find("tbody tr").Slice(2, 3).Remove()
			firstHTML, _ := first.Html()
			second, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
			switch mode {
			case "regression":
				second.Find("tbody tr").First().Remove()
			case "duplicate":
				second.Find("tbody").AppendSelection(second.Find("tbody tr").First().Clone())
			case "malformed":
				second.Find("tbody tr").Last().Find(".magnet-link-wrap").Remove()
			}
			secondHTML, _ := second.Html()
			if mode == "truncated" {
				secondHTML = strings.Split(secondHTML, "</tbody>")[0]
			}
			count := 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				count++
				if count == 1 {
					io.WriteString(w, firstHTML)
				} else {
					io.WriteString(w, secondHTML)
				}
			})
			c.initialTake = 2
			items, err := c.History(t.Context(), "4063", "370")
			if !errors.Is(err, ErrIncompleteHistory) || items != nil {
				t.Fatalf("%s must fail: len=%d err=%v", mode, len(items), err)
			}
		})
	}
}

func TestResponseErrorsNeverLookEmpty(t *testing.T) {
	for _, body := range []string{"", "<html><title>Login</title></html>", "<html><title>Just a moment...</title></html>", `<script src="/cdn-cgi/challenge-platform/a"></script>`, "<rss version=\"2.0\"><channel></channel></rss>"} {
		t.Run(fmt.Sprintf("body-%d", len(body)), func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
			if items, err := c.Catalog(t.Context(), 2026, "秋"); err == nil || items != nil {
				t.Fatalf("catalog accepted %q", body)
			}
			if _, err := c.Detail(t.Context(), "227"); err == nil {
				t.Fatalf("detail accepted %q", body)
			}
			if items, err := c.History(t.Context(), "227", "161"); err == nil || items != nil {
				t.Fatalf("history accepted %q", body)
			}
			if items, err := c.Feed(t.Context(), "227", "161"); err == nil || items != nil {
				t.Fatalf("feed accepted %q", body)
			}
		})
	}
}

func TestFeedRejectsMalformedFields(t *testing.T) {
	original := string(fixture(t, "rss-4063-370.xml"))
	for name, body := range map[string]string{
		"namespace-alias":  strings.ReplaceAll(original, "https://mikanani.me/0.1/", "https://mikanime.tv/0.1/"),
		"torrent-mismatch": strings.Replace(original, "d4ecfc4906ffc64283b9c8d4d7085532a329c7d5.torrent", strings.Repeat("a", 40)+".torrent", 1),
		"size-mismatch":    strings.Replace(original, `length="512124512"`, `length="1"`, 1),
		"empty-title":      strings.Replace(original, "<title>[LoliHouse]", "<title></title><ignored>[LoliHouse]", 1),
		"wrong-group":      strings.Replace(original, "subgroupid=370", "subgroupid=999", 1),
		"truncated":        strings.TrimSuffix(strings.TrimSpace(original), "</channel></rss>"),
		"trailer":          original + "<html/>",
	} {
		t.Run(name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
			items, err := c.Feed(t.Context(), "4063", "370")
			if name == "namespace-alias" {
				if err != nil || len(items) != 3 {
					t.Fatalf("namespace should work: %v", err)
				}
			} else if !errors.Is(err, ErrStructure) || items != nil {
				t.Fatalf("bad feed accepted: items=%v err=%v", items, err)
			}
		})
	}
}

func TestHTTPFailuresAndRetryAfter(t *testing.T) {
	for _, status := range []int{403, 404, 429, 500, 503} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var requests atomic.Int32
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(status)
			})
			_, err := c.Feed(t.Context(), "227", "161")
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != status || httpErr.RetryAfter != time.Minute {
				t.Fatalf("wrong HTTP error: %v", err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Millisecond)
			defer cancel()
			_, err = c.Feed(ctx, "227", "161")
			if !errors.Is(err, context.DeadlineExceeded) || requests.Load() != 1 {
				t.Fatalf("Retry-After not respected: requests=%d err=%v", requests.Load(), err)
			}
		})
	}
	now := time.Now().Truncate(time.Second)
	if parseRetryAfter(now.Add(time.Minute).UTC().Format(http.TimeFormat), now) != time.Minute || parseRetryAfter("-1", now) != 0 {
		t.Fatal("bad Retry-After date handling")
	}
}

func TestCancellationTimeoutAndRateLimit(t *testing.T) {
	var requests atomic.Int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.Feed(ctx, "227", "161"); !errors.Is(err, context.Canceled) || requests.Load() != 0 {
		t.Fatalf("cancel before request: requests=%d err=%v", requests.Load(), err)
	}
	c.timeout = 25 * time.Millisecond
	if _, err := c.Feed(t.Context(), "227", "161"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("HTTP timeout: %v", err)
	}
	var mu sync.Mutex
	var starts []time.Time
	limited := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		w.Write([]byte("\x89PNG\r\n\x1a\nimage"))
	})
	limited.interval = 35 * time.Millisecond
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := limited.Cover(t.Context(), limited.base.String()+"/poster"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(starts) != 3 || starts[2].Sub(starts[0]) < 60*time.Millisecond {
		t.Fatalf("requests not rate limited: %v", starts)
	}
	limited.nextRequest = time.Now().Add(time.Minute)
	ctx, cancel = context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if _, err := limited.Cover(ctx, limited.base.String()+"/poster"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("rate wait not cancelled: %v", err)
	}
}

func TestCoverValidationAndLimit(t *testing.T) {
	image := []byte("\x89PNG\r\n\x1a\nimage")
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/image":
			w.Write(image)
		case "/large":
			w.Header().Set("Content-Length", strconv.Itoa((5<<20)+1))
		case "/large-chunked":
			w.(http.Flusher).Flush()
			io.Copy(w, io.LimitReader(repeatingReader{}, (5<<20)+1))
		case "/redirect":
			http.Redirect(w, r, "/image", http.StatusFound)
		case "/unsafe-redirect":
			http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
		default:
			io.WriteString(w, "<html>login</html>")
		}
	})
	for _, path := range []string{"/image", "/redirect"} {
		got, err := c.Cover(t.Context(), c.base.String()+path)
		if err != nil || !bytes.Equal(got, image) {
			t.Fatalf("image %s: %q %v", path, got, err)
		}
	}
	for _, path := range []string{"/large", "/large-chunked"} {
		if _, err := c.Cover(t.Context(), c.base.String()+path); !errors.Is(err, ErrResponseTooLarge) {
			t.Fatalf("cover size limit %s: %v", path, err)
		}
	}
	for _, raw := range []string{"file:///image", "data:image/png;base64,AAAA", "/relative", "ftp://example.com/image", c.base.String() + "/login", c.base.String() + "/unsafe-redirect"} {
		if _, err := c.Cover(t.Context(), raw); err == nil {
			t.Fatalf("invalid image URL/body accepted: %q", raw)
		}
	}
}

type repeatingReader struct{}

func (repeatingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestRedirectedOriginResolvesResourceURLs(t *testing.T) {
	body := fixture(t, "probe-b4063-g370-take65.html")
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer destination.Close()
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL+r.URL.RequestURI(), http.StatusFound)
	})
	items, err := c.History(t.Context(), "4063", "370")
	if err != nil || len(items) != 3 || !strings.HasPrefix(items[0].TorrentURL, destination.URL+"/Download/") {
		t.Fatalf("redirected resource URLs: %+v err=%v", items, err)
	}
}

func TestInvalidInputsDoNotRequest(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid input caused HTTP request") })
	if _, err := c.Catalog(t.Context(), 0, "秋"); err == nil {
		t.Error("invalid year accepted")
	}
	if _, err := c.Catalog(t.Context(), 2026, "invalid"); err == nil {
		t.Error("invalid season accepted")
	}
	if _, err := c.Detail(t.Context(), "../1"); err == nil {
		t.Error("invalid anime ID accepted")
	}
	if _, err := c.History(t.Context(), "227", "0"); err == nil {
		t.Error("invalid group ID accepted")
	}
	if _, err := c.Feed(t.Context(), "227", "../1"); err == nil {
		t.Error("invalid RSS group ID accepted")
	}
	if _, err := New("file:///invalid").Catalog(t.Context(), 2026, "秋"); err == nil {
		t.Error("invalid origin accepted")
	}
}
