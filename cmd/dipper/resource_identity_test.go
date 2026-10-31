package main

import (
	"testing"

	"dipper.local/dipper/internal/core"
)

func TestDownloadForResourceUsesStableIdentity(t *testing.T) {
	s := &State{resourceHashes: map[string]string{}}
	s.Downloads = []Download{
		{ID: "hash-a", ResourceID: "ra", Title: "Show - 01 1080p"},
		{ID: "hash-b", ResourceID: "rb", Title: "Show - 01 1080p"},
	}
	if d, _, ok := s.downloadForResource(Resource{ID: "rb", Title: "Show - 01 1080p"}); !ok || d.ID != "hash-b" {
		t.Fatalf("resource rb must match its own download, got %q ok=%v", d.ID, ok)
	}
}

func TestDownloadForResourceResolvesHashThroughCoreItems(t *testing.T) {
	s := &State{resourceHashes: map[string]string{"rc": "hash-b"}}
	s.Downloads = []Download{
		{ID: "hash-a", ResourceID: "ra", Title: "Show - 01 1080p"},
		{ID: "hash-b", ResourceID: "ra", Title: "Show - 01 1080p"},
	}
	if d, _, ok := s.downloadForResource(Resource{ID: "rc", Title: "Show - 01 1080p"}); !ok || d.ID != "hash-b" {
		t.Fatalf("resource rc shares hash-b content and must match that download, got %q ok=%v", d.ID, ok)
	}
	if _, _, ok := s.downloadForResource(Resource{ID: "rd", Title: "Show - 01 1080p"}); ok {
		t.Fatal("an unrelated resource with the same title must not match any download")
	}
}

func TestProjectRecordsResourceIdentity(t *testing.T) {
	s := &State{}
	state := core.State{
		Subscriptions: []core.Subscription{{
			ID: "sub-a",
			Items: map[string]core.Item{
				"ra": {Hash: "hash-a"},
				"rb": {Hash: "hash-b"},
			},
		}},
		Tasks: []core.TaskRecord{
			{Task: core.DownloadTask{Hash: "hash-a", Resource: core.Resource{ID: "ra", Title: "Show - 01 1080p"}, Directory: "C:\\data\\a"}},
			{Task: core.DownloadTask{Hash: "hash-b", Resource: core.Resource{ID: "rb", Title: "Show - 01 1080p"}, Directory: "C:\\data\\b"}},
		},
	}
	s.project(state, nil)
	if s.resourceHashes["ra"] != "hash-a" || s.resourceHashes["rb"] != "hash-b" {
		t.Fatalf("projection lost the resource-to-hash bridge: %v", s.resourceHashes)
	}
	if len(s.Downloads) != 2 {
		t.Fatalf("expected two projected downloads, got %d", len(s.Downloads))
	}
	if s.Downloads[0].ResourceID != "ra" || s.Downloads[1].ResourceID != "rb" {
		t.Fatalf("projected downloads lost their resource identity: %q %q", s.Downloads[0].ResourceID, s.Downloads[1].ResourceID)
	}
}
