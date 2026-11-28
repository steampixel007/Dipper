package source

import (
	"net/http"
	"strings"
	"testing"
)

func TestDetailSummaryFromCapturedPages(t *testing.T) {
	for _, animeID := range []string{"227", "4063"} {
		t.Run(animeID, func(t *testing.T) {
			body := fixture(t, "bangumi-"+animeID+".html")
			client := testClient(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/Home/Bangumi/"+animeID {
					t.Errorf("unexpected detail request: %s", request.URL)
				}
				writer.Write(body)
			})
			detail, err := client.Detail(t.Context(), animeID)
			if err != nil {
				t.Fatal(err)
			}
			if animeID == "227" && (!strings.HasPrefix(detail.Anime.Summary, "主角工藤新一") || !strings.HasSuffix(detail.Anime.Summary, "等情节。")) {
				t.Fatalf("captured anime synopsis was lost or replaced by release text: %q", detail.Anime.Summary)
			}
			if animeID == "4063" && detail.Anime.Summary != "" {
				t.Fatalf("a page without an anime synopsis borrowed release text: %q", detail.Anime.Summary)
			}
			if len(detail.Groups) == 0 {
				t.Fatal("optional synopsis prevented subtitle group loading")
			}
		})
	}
}

func TestDetailMetadataFromCapturedPages(t *testing.T) {
	for _, want := range []struct {
		animeID, broadcastStart, officialURL, bangumiURL string
	}{
		{
			animeID:        "227",
			broadcastStart: "1/8/1996",
			officialURL:    "http://www.ytv.co.jp/conan/",
			bangumiURL:     "http://bgm.tv/subject/899",
		},
		{
			animeID:        "4063",
			broadcastStart: "10/5/2026",
			officialURL:    "https://mahoiku.com/anime/restart",
			bangumiURL:     "https://bgm.tv/subject/418229",
		},
		{
			animeID:        "4071",
			broadcastStart: "10/3/2026",
			officialURL:    "https://vertexforce.jp/",
			bangumiURL:     "https://bgm.tv/subject/636512",
		},
	} {
		t.Run(want.animeID, func(t *testing.T) {
			body := fixture(t, "bangumi-"+want.animeID+".html")
			client := testClient(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/Home/Bangumi/"+want.animeID {
					t.Errorf("unexpected detail request: %s", request.URL)
				}
				writer.Write(body)
			})
			detail, err := client.Detail(t.Context(), want.animeID)
			if err != nil {
				t.Fatal(err)
			}
			if detail.Anime.BroadcastStart != want.broadcastStart {
				t.Errorf("BroadcastStart = %q, want %q", detail.Anime.BroadcastStart, want.broadcastStart)
			}
			if detail.Anime.OfficialURL != want.officialURL {
				t.Errorf("OfficialURL = %q, want %q", detail.Anime.OfficialURL, want.officialURL)
			}
			if detail.Anime.BangumiURL != want.bangumiURL {
				t.Errorf("BangumiURL = %q, want %q", detail.Anime.BangumiURL, want.bangumiURL)
			}
		})
	}
}

func TestDetailIgnoresUnsafeExternalLinks(t *testing.T) {
	body := string(fixture(t, "bangumi-4071.html"))
	body = strings.Replace(body, `https://vertexforce.jp/`, `javascript:alert(1)`, 1)
	body = strings.Replace(body, `https://bgm.tv/subject/636512`, `https://user:pass@bgm.tv/subject/636512`, 1)
	client := testClient(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(body))
	})
	detail, err := client.Detail(t.Context(), "4071")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Anime.OfficialURL != "" {
		t.Fatalf("unsafe official link was accepted: %q", detail.Anime.OfficialURL)
	}
	if detail.Anime.BangumiURL != "" {
		t.Fatalf("credentialed bangumi link was accepted: %q", detail.Anime.BangumiURL)
	}
	if len(detail.Groups) == 0 {
		t.Fatal("unsafe optional links blocked subtitle group loading")
	}
}
