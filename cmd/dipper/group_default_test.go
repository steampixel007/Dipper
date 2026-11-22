package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/steampixel007/Dipper/internal/source"
)

func groupDefaultState(t *testing.T) *State {
	t.Helper()
	s := uiFixture(t, "detail")
	s.Page, s.AnimeID = "detail", "1"
	s.detailRevision = 1
	s.Group, s.ChoosingGroup, s.EditID = -1, false, ""
	s.GroupIDs = []string{"1", "2", "3"}
	return s
}

func TestDefaultGroupUsesLargestKnownCountAndKeepsListOrderOnTie(t *testing.T) {
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
			s.selectLargestGroup(context.Background(), 1, "1")
			if s.Group != test.want || s.GroupCountLoading {
				t.Fatalf("group=%d loading=%v, want group %d", s.Group, s.GroupCountLoading, test.want)
			}
		})
	}
}

func TestDefaultGroupFillsUnknownCountsBeforeChoosing(t *testing.T) {
	handlers := map[string][]versionTestRelease{
		"1": {{"[组一] 番剧 [01][1080p][简体][MP4]", 300}},
		"3": {
			{"[组三] 番剧 [01][1080p][简体][MP4]", 300},
			{"[组三] 番剧 [02][1080p][简体][MP4]", 300},
			{"[组三] 番剧 [03][1080p][简体][MP4]", 300},
			{"[组三] 番剧 [04][1080p][简体][MP4]", 300},
			{"[组三] 番剧 [05][1080p][简体][MP4]", 300},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rows, ok := handlers[req.URL.Query().Get("subtitleGroupId")]
		if req.URL.Path != "/Home/ExpandEpisodeTable" || !ok {
			http.NotFound(w, req)
			return
		}
		io.WriteString(w, versionTestTable(rows))
	}))
	defer server.Close()

	s := groupDefaultState(t)
	s.Source = source.New(server.URL)
	s.GroupCounts = []int{-1, 4, -1}
	updates := make(chan func(), 8)
	s.Update = func(update func()) { updates <- update }
	s.selectLargestGroup(context.Background(), 1, "1")
	awaitVersionState(t, updates, func() bool { return !s.GroupCountLoading })
	if s.Group != 2 || !slices.Equal(s.GroupCounts, []int{1, 4, 5}) || len(s.History) != 5 {
		t.Fatalf("group=%d counts=%v history=%d", s.Group, s.GroupCounts, len(s.History))
	}
}

func TestDefaultGroupStaysUnsetWhenNoCountCanBeLearned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	s := groupDefaultState(t)
	s.Source = source.New(server.URL)
	s.GroupCounts = []int{-1, -1, -1}
	updates := make(chan func(), 8)
	s.Update = func(update func()) { updates <- update }
	s.selectLargestGroup(context.Background(), 1, "1")
	awaitVersionState(t, updates, func() bool { return !s.GroupCountLoading })
	if s.Group != -1 {
		t.Fatalf("unknown counts selected group %d", s.Group)
	}
}
