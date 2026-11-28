package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/source"
)

func groupDefaultState(t *testing.T) *State {
	t.Helper()
	s := uiFixture(t, "detail")
	s.Page, s.AnimeID = "detail", "1"
	s.detailRevision = 1
	s.Group, s.ChoosingGroup, s.EditID = -1, false, ""
	s.GroupIDs = []string{"1", "2", "3"}
	s.GroupPreviews = nil
	return s
}

func TestDefaultGroupUsesLargestKnownCountFallbackWithoutPreviews(t *testing.T) {
	tests := []struct {
		name   string
		counts []int
		want   int
	}{
		{"largest", []int{2, 9, 4}, 1},
		{"tie keeps the earlier group", []int{7, 3, 7}, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := groupDefaultState(t)
			s.GroupCounts = test.counts
			s.selectDefaultGroup()
			if s.Group != test.want || s.GroupCountLoading {
				t.Fatalf("group=%d loading=%v, want group %d", s.Group, s.GroupCountLoading, test.want)
			}
		})
	}
}

func TestDefaultGroupPrefersChineseSubtitlesOverTruncatedRawGroup(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "bangumi-4071.html"))
	if err != nil {
		t.Fatal(err)
	}
	historyCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/Home/Bangumi/4071":
			w.Write(body)
		case "/Home/ExpandEpisodeTable":
			historyCalls++
			http.NotFound(w, req)
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	s := uiFixture(t, "detail")
	s.Source, s.Ctx = source.New(server.URL), context.Background()
	updates := make(chan func(), 16)
	s.Update = func(update func()) { updates <- update }
	s.openAnime("4071", "browse")
	awaitVersionState(t, updates, func() bool { return !s.DetailLoading })

	if !s.hasGroup() {
		t.Fatalf("no default group selected; groups=%v", s.Groups)
	}
	chosen := s.Groups[s.Group]
	if chosen == "Kirara Fantasia" || chosen == "生肉/不明字幕" {
		t.Fatalf("selected non-Chinese/raw group %q instead of a Chinese-subtitled group", chosen)
	}
	if s.Rule.Version == nil || s.Rule.Version.Language == core.SubtitleUnknown {
		t.Fatalf("default group %q did not recommend a Chinese-subtitled version: %+v", chosen, s.Rule.Version)
	}
	if historyCalls != 0 {
		t.Fatalf("non-truncated default group triggered %d unnecessary History requests", historyCalls)
	}
	if !s.detailReady() || len(s.History) == 0 {
		t.Fatalf("non-truncated default group was not immediately ready from Detail preview: ready=%v history=%d", s.detailReady(), len(s.History))
	}
}

func TestDefaultGroupRespectsSavedLanguageAndPriority(t *testing.T) {
	s := groupDefaultState(t)
	s.Groups = []string{"RawGroup", "HEVCGroup", "MP4Group"}
	s.GroupCounts = []int{12, 12, 12}
	s.GroupPreviews = [][]source.Resource{
		{
			{ID: "r1", Title: "[RawGroup] Show - 01 [1080p][AVC][MKV]", Size: 400e6},
			{ID: "r2", Title: "[RawGroup] Show - 02 [1080p][AVC][MKV]", Size: 400e6},
			{ID: "r3", Title: "[RawGroup] Show - 03 [1080p][AVC][MKV]", Size: 400e6},
		},
		{
			{ID: "h1", Title: "[HEVCGroup] Show - 01 [1080p][简体][HEVC][MKV]", Size: 250e6},
			{ID: "h2", Title: "[HEVCGroup] Show - 02 [1080p][简体][HEVC][MKV]", Size: 250e6},
			{ID: "h3", Title: "[HEVCGroup] Show - 03 [1080p][简体][HEVC][MKV]", Size: 250e6},
		},
		{
			{ID: "m1", Title: "[MP4Group] Show - 01 [1080p][简体][MP4]", Size: 500e6},
			{ID: "m2", Title: "[MP4Group] Show - 02 [1080p][简体][MP4]", Size: 500e6},
			{ID: "m3", Title: "[MP4Group] Show - 03 [1080p][简体][MP4]", Size: 500e6},
		},
	}

	s.VersionLanguage = core.SubtitleSimplified
	s.VersionPriority = preferCompatibility
	if got := s.bestDefaultGroup(); got != 2 {
		t.Fatalf("compatibility priority selected group %d (%s), want 2 (MP4Group)", got, s.Groups[got])
	}

	s.VersionPriority = preferSize
	if got := s.bestDefaultGroup(); got != 1 {
		t.Fatalf("size priority selected group %d (%s), want 1 (HEVCGroup)", got, s.Groups[got])
	}
}

func TestDefaultGroupStaysUnsetWhenNoCountCanBeLearned(t *testing.T) {
	s := groupDefaultState(t)
	s.GroupCounts = []int{-1, -1, -1}
	s.selectDefaultGroup()
	if s.Group != -1 {
		t.Fatalf("unknown counts without previews selected group %d", s.Group)
	}
}
