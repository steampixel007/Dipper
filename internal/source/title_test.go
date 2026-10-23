package source

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestResourceTitlesPreserveInternalWhitespace(t *testing.T) {
	const title = "[字幕组]  保留　全角空格  ＡVC"
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(fixture(t, "probe-b4063-g370-take65.html"))))
	if err != nil {
		t.Fatal(err)
	}
	doc.Find("a.magnet-link-wrap").First().SetText(title)
	html, err := doc.Html()
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("1", 40)
	rss := fmt.Sprintf(`<rss version="2.0"><channel><title>蜜柑</title><link>https://example.com/RSS/Bangumi?bangumiId=4063&amp;subgroupid=370</link><item><title>%s</title><link>/Home/Episode/%s</link><pubDate>2026-10-23</pubDate><enclosure url="/Download/20261023/%s.torrent" length="100" type="application/x-bittorrent"/></item></channel></rss>`, title, hash, hash)
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/RSS/Bangumi" {
			fmt.Fprint(w, rss)
		} else {
			fmt.Fprint(w, html)
		}
	})
	history, err := c.History(t.Context(), "4063", "370")
	if err != nil {
		t.Fatal(err)
	}
	feed, err := c.Feed(t.Context(), "4063", "370")
	if err != nil {
		t.Fatal(err)
	}
	if history[0].Title != title || feed[0].Title != title {
		t.Fatalf("original title changed: history=%q RSS=%q", history[0].Title, feed[0].Title)
	}
}
