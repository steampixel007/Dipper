package main

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
)

type versionPriority int

const (
	preferQuality versionPriority = iota
	preferCompatibility
	preferSize
)

type versionOption struct {
	core.ReleaseVersion
	count        int
	typicalBytes int64
}

func (s *State) setupVersions() {
	s.VersionSetup = !s.Backfill && !s.ViewResources
	if !s.VersionSetup {
		s.Rule.Version = nil
		return
	}
	s.Versions = nil
	indexByVersion := map[core.ReleaseVersion]int{}
	var sizes [][]int64
	for _, resource := range s.resources() {
		version := resource.releaseVersion()
		if version.Ambiguous || version.Collection || version == (core.ReleaseVersion{}) {
			continue
		}
		index, found := indexByVersion[version]
		if !found {
			index = len(s.Versions)
			indexByVersion[version] = index
			s.Versions = append(s.Versions, versionOption{ReleaseVersion: version})
			sizes = append(sizes, nil)
		}
		s.Versions[index].count++
		if resource.Bytes > 0 {
			sizes[index] = append(sizes[index], resource.Bytes)
		}
	}
	for index, bytes := range sizes {
		if len(bytes) > 0 {
			slices.Sort(bytes)
			s.Versions[index].typicalBytes = bytes[len(bytes)/2]
		}
	}
	if s.EditID != "" {
		s.TitleSetup = s.Rule.Version == nil
		s.OnlyMatches = !s.TitleSetup
		return
	}
	if !s.TitleSetup {
		s.applyRecommendedVersion()
	}
}

func supportsPriority(option versionOption, priority versionPriority) bool {
	switch priority {
	case preferCompatibility:
		return option.Video == "AVC" && option.Container == "MP4" &&
			(option.Audio == "" || option.Audio == "AAC") &&
			option.BitDepth <= 8 && !option.HDR && option.Subtitles != "外挂" && option.Subtitles != "内封"
	case preferSize:
		return option.typicalBytes > 0
	default:
		return option.Height > 0
	}
}

func compareVersionQuality(left, right versionOption) int {
	if order := cmp.Compare(left.Height, right.Height); order != 0 {
		return order
	}
	rank := func(source string) int {
		switch source {
		case "BD":
			return 4
		case "WEB-DL":
			return 3
		case "WEB", "WEBRip":
			return 2
		case "TV", "DVD":
			return 1
		default:
			return 0
		}
	}
	return cmp.Compare(rank(left.Source), rank(right.Source))
}

func (s *State) recommendedVersion() int {
	best := -1
	for index, option := range s.Versions {
		if s.VersionLanguage != core.SubtitleUnknown && option.Language&s.VersionLanguage == 0 {
			continue
		}
		if !supportsPriority(option, s.VersionPriority) {
			continue
		}
		if best < 0 {
			best = index
			continue
		}
		current := s.Versions[best]
		order := compareVersionQuality(option, current)
		if s.VersionPriority == preferSize && option.typicalBytes != current.typicalBytes {
			order = cmp.Compare(current.typicalBytes, option.typicalBytes)
		}
		if order > 0 || (order == 0 && option.count > current.count) {
			best = index
		}
	}
	return best
}

func (s *State) applyRecommendedVersion() {
	index := s.recommendedVersion()
	if index < 0 {
		s.Rule = Rule{}
		s.Checked = map[string]bool{}
		return
	}
	s.pickVersion(s.Versions[index].ReleaseVersion)
}

func (s *State) pickVersion(version core.ReleaseVersion) {
	s.Rule = Rule{Version: &version}
	s.OnlyMatches = true
	s.Checked = map[string]bool{}
	s.initSelection()
}

func (s *State) canSaveDetailRule() bool {
	if !s.detailReady() {
		return false
	}
	if !s.VersionSetup || s.Rule.Version != nil || (s.TitleSetup && hasTitleConditions(s.Rule)) {
		return true
	}
	sub, saved := s.subscription(s.EditID)
	return s.TitleSetup && s.Editing && saved && sub.Rule.Version == nil && !hasTitleConditions(sub.Rule)
}

func hasTitleConditions(rule Rule) bool {
	return len(rule.All)+len(rule.None)+len(rule.Any) > 0
}

func languageLabel(language core.SubtitleLanguage) string {
	switch language {
	case core.SubtitleSimplified:
		return "简体中文"
	case core.SubtitleTraditional:
		return "繁体中文"
	case core.SubtitleSimplified | core.SubtitleTraditional:
		return "简繁中文"
	default:
		return "字幕语言未知"
	}
}

func versionLabel(version core.ReleaseVersion) string {
	parts := []string{fmt.Sprintf("%dp", version.Height), languageLabel(version.Language)}
	if version.Height == 0 {
		parts = parts[1:]
	}
	for _, label := range []string{version.Source, version.Video, version.Audio, version.Container, version.Subtitles, version.Edition} {
		if label != "" {
			parts = append(parts, label)
		}
	}
	if version.BitDepth > 0 {
		parts = append(parts, fmt.Sprintf("%d-bit", version.BitDepth))
	}
	if version.HDR {
		parts = append(parts, "HDR")
	}
	return strings.Join(parts, " · ")
}

func (s *State) switchVersionMode(manual bool) {
	if !manual {
		s.TitleRuleDraft = cloneRule(s.Rule)
	}
	s.TitleSetup = manual
	s.Checked = map[string]bool{}
	s.UndoFilters, s.RedoFilters = nil, nil
	s.ActiveTitleWord, s.ActiveTitleRow = "", ""
	s.FilterFeedback = ""
	s.WordOpen = false
	s.OnlyMatches = !manual
	s.ResourceScroll = ui.ScrollState{}
	if manual {
		s.Rule = cloneRule(s.TitleRuleDraft)
	} else {
		s.applyRecommendedVersion()
	}
}

func setupRadio(c *ui.Context, button ui.Element, label string, active bool, action func()) {
	pal := colors(c)
	button.Label(label).Padding(compactPaddingY, compactPaddingX).Radius(radiusSmall).Border(1, pal.outline).
		Background(pal.surface).TextColor(pal.ink).Cursor(ui.CursorPointer)
	if active {
		button.Background(pal.accentWash).BorderColor(pal.accent).TextColor(pal.accentInk)
	} else if button.Hovered() {
		button.Background(pal.chrome)
	}
	bindRadio(button, active, action)
	tactile(button).Children(func() {
		ui.Text(c, label).FontSize(textSize(c, textControl)).FontWeight(500)
	})
}

func bindRadio(radio ui.Element, active bool, action func()) {
	if active {
		radio.OnClick(action)
	} else {
		radio.OnChange(action)
	}
}

func setupSegments(c *ui.Context, key, label string, selected int, labels []string, choose func(int)) {
	active := selected
	parts := ui.SegmentedBase(c.Key(key), &selected, len(labels))
	parts.Track.Label(label).Wrap().Gap(8).Children(func() {
		for index, text := range labels {
			segment := parts.Segment(index)
			filterSegment(c, segment, text, text, index == active)
			if index == active {
				segment.OnClick(func() { choose(index) })
			}
		}
	}).OnChange(func() { choose(selected) })
}

func (s *State) versionOptions(c *ui.Context) {
	pal := colors(c)
	if s.TitleSetup {
		heading(c, "按原标题追番")
		compactQuiet(c, "switch-recommendation", "返回版本推荐", func() { s.switchVersionMode(false) }).TextColor(pal.accentInk).AlignSelf(ui.Start)
		s.ruleEditor(c)
		return
	}
	heading(c, "选字幕语言")
	languages := []core.SubtitleLanguage{core.SubtitleUnknown, core.SubtitleSimplified, core.SubtitleTraditional}
	setupSegments(c, "version-language", "选字幕语言", slices.Index(languages, s.VersionLanguage), []string{"不限", "简体", "繁体"}, func(index int) {
		s.VersionLanguage = languages[index]
		s.applyRecommendedVersion()
	})
	heading(c, "优先考虑")
	setupSegments(c, "version-priority", "优先考虑", int(s.VersionPriority), []string{"画质", "兼容性", "省空间"}, func(index int) {
		s.VersionPriority = versionPriority(index)
		s.applyRecommendedVersion()
	})
	compactQuiet(c, "switch-title-rules", "按原标题追番", func() { s.switchVersionMode(true) }).TextColor(pal.accentInk).AlignSelf(ui.Start)
}

func (s *State) versionSummary(c *ui.Context) {
	recommended := s.recommendedVersion()
	ui.Column(c).Gap(12).Children(func() {
		heading(c, "选择版本")
		if recommended < 0 && len(s.Versions) == 0 {
			caption(c, "这个字幕组的版本暂未识别，可以按原标题追番。")
		} else if recommended < 0 && s.VersionLanguage != 0 && !slices.ContainsFunc(s.Versions, func(option versionOption) bool {
			return option.Language&s.VersionLanguage != 0
		}) {
			caption(c, "没有标注这种字幕语言的版本。可以更换语言、字幕组，或查看原标题。")
		} else if recommended < 0 {
			caption(c, "没有足够信息推荐。可以改选下方版本，或按原标题追番。")
		} else {
			option := s.Versions[recommended]
			switch s.VersionPriority {
			case preferCompatibility:
				caption(c, "推荐 AVC + MP4")
			case preferSize:
				caption(c, "推荐单集约 "+byteSizeText(option.typicalBytes))
			default:
				reason := fmt.Sprintf("推荐 %dp", option.Height)
				if option.Source != "" {
					reason += " · " + option.Source
				}
				caption(c, reason)
			}
		}
		if s.Rule.Version != nil {
			caption(c, "已选 "+versionLabel(*s.Rule.Version))
		}
		selected := core.ReleaseVersion{}
		if s.Rule.Version != nil {
			selected = *s.Rule.Version
		}
		ui.Scroll(c.Key("version-choices")).MaxHeight(144).Children(func() {
			ui.RadioGroup(c, func() {
				for index, option := range s.Versions {
					if s.VersionLanguage != 0 && option.Language&s.VersionLanguage == 0 {
						continue
					}
					label := fmt.Sprintf("%s · %d 条", versionLabel(option.ReleaseVersion), option.count)
					if index == recommended {
						label = "推荐 · " + label
					}
					radio := ui.RadioBase(c.Key(fmt.Sprintf("version-%d", index)), &selected, option.ReleaseVersion)
					setupRadio(c, radio, label, selected == option.ReleaseVersion, func() {
						s.pickVersion(option.ReleaseVersion)
					})
				}
			}).Label("可用版本").Row().Wrap().Gap(8)
		})
	})
}

var titleWordPattern = regexp.MustCompile(`[\p{Latin}\p{N}]+(?:[-.+×][\p{Latin}\p{N}]+)*|[\p{L}]+`)

func (s *State) originalTitle(c *ui.Context, resource Resource) {
	pal := colors(c)
	ui.Row(c).Wrap().Gap(0).Children(func() {
		offset := 0
		for index, span := range titleWordPattern.FindAllStringIndex(resource.Title, -1) {
			if span[0] > offset {
				ui.Text(c, resource.Title[offset:span[0]]).FontSize(textSize(c, textBody)).LineHeight(1.5).TextColor(pal.ink)
			}
			word := resource.Title[span[0]:span[1]]
			active := s.ActiveTitleRow == resource.ID && s.ActiveTitleWord == word
			button := ui.ButtonBase(c.Key(fmt.Sprintf("title-word-%d", index))).
				Label("查看标题词「"+word+"」").
				Padding(4, 4).Radius(radiusSmall).Cursor(ui.CursorPointer).
				FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.accentInk)
			tactile(button)
			if active {
				button.Background(pal.accentWash)
			} else if button.Hovered() {
				button.Background(pal.chrome)
			}
			button.Children(func() { ui.Text(c, word) }).OnClick(func() {
				if s.ActiveTitleRow == resource.ID && s.ActiveTitleWord == word {
					s.ActiveTitleWord, s.ActiveTitleRow = "", ""
					return
				}
				s.ActiveTitleWord, s.ActiveTitleRow = word, resource.ID
			})
			offset = span[1]
		}
		if offset < len(resource.Title) {
			ui.Text(c, resource.Title[offset:]).FontSize(textSize(c, textBody)).LineHeight(1.5).TextColor(pal.ink)
		}
	})
	if s.ActiveTitleRow != resource.ID || s.ActiveTitleWord == "" {
		return
	}
	word := s.ActiveTitleWord
	ui.Column(c).Padding(12, 16).Gap(8).Radius(radiusControl).Background(pal.sunken).Children(func() {
		caption(c, word+" · "+core.ReleaseWordMeaning(word))
		selected := s.wordChoice(word)
		ui.RadioGroup(c, func() {
			for _, option := range []struct {
				label  string
				choice wordChoice
			}{{"包含", wordWant}, {"不包含", wordAvoid}} {
				count := s.countRuleMatches(ruleWithWord(s.Rule, word, option.choice))
				label := fmt.Sprintf("%s「%s」 · 剩 %d 条", option.label, word, count)
				radio := ui.RadioBase(c.Key(fmt.Sprintf("title-rule-%d", option.choice)), &selected, option.choice)
				setupRadio(c, radio, label, selected == option.choice, func() {
					s.setWord(word, option.choice)
				})
			}
		}).Label("标题词条件").Row().Wrap().Gap(8)
	})
}
