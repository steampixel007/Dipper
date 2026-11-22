package main

import (
	"regexp"
	"strconv"

	"github.com/steampixel007/Dipper/internal/core"
)

var episodeRevisionPattern = regexp.MustCompile(`(?i)(?:\[|\s-\s|\bs\d{1,2}[\s._-]*e\s*p?\s*)(\d{1,4}(?:\.\d+)?)v(\d+)\b`)

func episodeRevision(resource Resource) int {
	episode, known := episodeNumber(resource.Episode)
	if known {
		for _, match := range episodeRevisionPattern.FindAllStringSubmatch(core.Normalize(resource.Title), -1) {
			if number, ok := episodeNumber(match[1]); ok && number == episode {
				if revision, err := strconv.Atoi(match[2]); err == nil {
					return revision
				}
			}
		}
	}
	return 1
}

func (s *State) selectsOnePerEpisode() bool {
	return s.VersionSetup && !s.Editing && !s.Backfill && s.Rule.Version != nil
}

func (s *State) episodeChoices(matches []Resource) ([]Resource, map[string]Resource) {
	if !s.selectsOnePerEpisode() {
		return matches, nil
	}
	byEpisode := make(map[string]Resource)
	for _, resource := range matches {
		if resource.Episode == "" {
			continue
		}
		current, exists := byEpisode[resource.Episode]
		picked, currentPicked := s.Checked[resource.ID], s.Checked[current.ID]
		if !exists || (picked && !currentPicked) ||
			(picked == currentPicked && preferredEpisodeRelease(resource, current)) {
			byEpisode[resource.Episode] = resource
		}
	}
	choices := make([]Resource, 0, len(byEpisode))
	for _, resource := range matches {
		if resource.Episode == "" || byEpisode[resource.Episode].ID == resource.ID {
			choices = append(choices, resource)
		}
	}
	return choices, byEpisode
}

func preferredEpisodeRelease(candidate, current Resource) bool {
	revision, currentRevision := episodeRevision(candidate), episodeRevision(current)
	if revision != currentRevision {
		return revision > currentRevision
	}
	return candidate.Published > current.Published
}

func (s *State) setResourceChecked(resource Resource, checked bool) {
	if checked && s.selectsOnePerEpisode() && resource.Episode != "" {
		for _, alternative := range s.resources() {
			if alternative.Episode == resource.Episode {
				s.Checked[alternative.ID] = false
			}
		}
		if s.EpisodeFilter == resource.Episode {
			s.EpisodeFilter = ""
			s.ResourceScroll.Y = s.EpisodeFilterScroll
		}
	}
	s.Checked[resource.ID] = checked
}
