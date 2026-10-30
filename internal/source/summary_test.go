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
