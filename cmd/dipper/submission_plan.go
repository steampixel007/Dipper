package main

import "github.com/steampixel007/Dipper/internal/core"

type submissionPlan struct {
	Selected   []Resource
	New        []Resource
	IDs        []string
	Incomplete int
	Completed  int
	Failed     int
}

func (s *State) planSubmission() submissionPlan {
	var plan submissionPlan
	hashes := make(map[string]string, len(s.History))
	for _, resource := range coreResources(s.History) {
		if hash, err := core.ResourceHash(resource); err == nil {
			hashes[resource.ID] = hash
		}
	}
	byHash := make(map[string]Download, len(s.Downloads))
	byResource := make(map[string]Download, len(s.Downloads))
	for _, download := range s.Downloads {
		if download.ID != "" {
			byHash[download.ID] = download
		}
		if download.ResourceID != "" {
			byResource[download.ResourceID] = download
		}
	}
	seen := make(map[string]bool)
	for _, resource := range s.resources() {
		matches, _ := decide(resource.Title, s.Rule)
		if !matches || !s.Checked[resource.ID] {
			continue
		}
		plan.Selected = append(plan.Selected, resource)
		if s.Editing && !s.Backfill {
			continue
		}
		hash := s.resourceHashes[resource.ID]
		if hash == "" {
			hash = hashes[resource.ID]
		}
		identity := "resource:" + resource.ID
		if hash != "" {
			identity = "hash:" + hash
		}
		download, exists := byHash[hash]
		if !exists {
			download, exists = byResource[resource.ID]
		}
		if exists && download.ID != "" {
			identity = "hash:" + download.ID
		}
		if seen[identity] {
			continue
		}
		seen[identity] = true
		if exists {
			switch {
			case download.Failed:
				plan.Failed++
			case download.Progress >= 1:
				plan.Completed++
			default:
				plan.Incomplete++
			}
			continue
		}
		plan.New = append(plan.New, resource)
		plan.IDs = append(plan.IDs, resource.ID)
	}
	return plan
}
