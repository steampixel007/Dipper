package main

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/source"
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
	episodes     int
	typicalBytes int64
}

func (s *State) focusedVersionSetup() bool {
	return s.VersionSetup && !s.Editing && !s.Backfill && !s.ViewResources
}

func collectVersions(resources []Resource) ([]versionOption, int, int) {
	var versions []versionOption
	indexByVersion := map[core.ReleaseVersion]int{}
	var sizes [][]int64
	var episodeSets []map[string]bool
	collections, unclassified := 0, 0
	for _, resource := range resources {
		version := resource.releaseVersion()
		if version.Collection {
			collections++
			continue
		}
		if version.Ambiguous || version == (core.ReleaseVersion{}) {
			unclassified++
			continue
		}
		index, found := indexByVersion[version]
		if !found {
			index = len(versions)
			indexByVersion[version] = index
			versions = append(versions, versionOption{ReleaseVersion: version})
			sizes = append(sizes, nil)
			episodeSets = append(episodeSets, map[string]bool{})
		}
		versions[index].count++
		if resource.Episode != "" {
			episodeSets[index][resource.Episode] = true
		}
		if bytes := sizeBytes(resource.Bytes, resource.MB); bytes > 0 {
			sizes[index] = append(sizes[index], bytes)
		}
	}
	for index := range versions {
		versions[index].episodes = len(episodeSets[index])
		if len(sizes[index]) > 0 {
			slices.Sort(sizes[index])
			versions[index].typicalBytes = sizes[index][len(sizes[index])/2]
		}
	}
	return versions, collections, unclassified
}

func (s *State) setupVersions() {
	s.VersionSetup = !s.Backfill && !s.ViewResources
	if !s.VersionSetup {
		s.Rule.Version = nil
		s.Versions = nil
		s.CollectionCount, s.UnclassifiedCount = 0, 0
		return
	}
	s.Versions, s.CollectionCount, s.UnclassifiedCount = collectVersions(s.resources())
	s.sortVersions()
	if s.EditID != "" {
		s.TitleSetup = s.Rule.Version == nil
		s.OnlyMatches = !s.TitleSetup
		return
	}
	if len(s.Versions) == 0 && len(s.resources()) > 0 && !s.HistoryPreviewOnly {
		s.TitleSetup = true
		s.Rule = Rule{}
		s.OnlyMatches = true
		s.Checked = map[string]bool{}
		return
	}
	if !s.TitleSetup {
		if s.ManualVersion && s.Rule.Version != nil && slices.ContainsFunc(s.Versions, func(option versionOption) bool {
			return option.ReleaseVersion == *s.Rule.Version
		}) {
			s.OnlyMatches = true
			s.Checked = map[string]bool{}
			s.initSelection()
			return
		}
		s.applyRecommendedVersion()
	}
}

func versionMatchesLanguage(option versionOption, language core.SubtitleLanguage) bool {
	return language == core.SubtitleUnknown || option.Language&language != 0
}

func mainstreamEpisodeThreshold(versions []versionOption, language core.SubtitleLanguage) int {
	maxEpisodes := 0
	for _, option := range versions {
		if !versionMatchesLanguage(option, language) {
			continue
		}
		if option.episodes > maxEpisodes {
			maxEpisodes = option.episodes
		}
	}
	if maxEpisodes < 3 {
		return 0
	}
	return max(2, (maxEpisodes+1)/2)
}

func compatibilityScore(option versionOption) int {
	score := 0
	switch option.Container {
	case "MP4":
		score += 40
	case "":
		score += 15
	case "MKV":
		score += 5
	}
	switch option.Video {
	case "AVC":
		score += 30
	case "":
		score += 15
	case "HEVC":
		score += 5
	}
	switch option.Subtitles {
	case "内嵌":
		score += 20
	case "":
		score += 10
	case "内封":
		score += 5
	}
	if option.BitDepth <= 8 {
		score += 15
	}
	if !option.HDR {
		score += 15
	}
	if option.Audio == "" || option.Audio == "AAC" {
		score += 10
	} else {
		score += 5
	}
	return score
}

func isStrictlyCompatible(option versionOption) bool {
	return (option.Container == "MP4" || option.Container == "") &&
		(option.Video == "AVC" || option.Video == "") &&
		(option.Audio == "" || option.Audio == "AAC") &&
		option.BitDepth <= 8 && !option.HDR &&
		option.Subtitles != "外挂" && option.Subtitles != "内封"
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

func codecCompressionRank(video string) int {
	switch video {
	case "AV1":
		return 3
	case "HEVC":
		return 2
	case "AVC":
		return 1
	default:
		return 0
	}
}

func compareVersionOptions(left, right versionOption, language core.SubtitleLanguage, priority versionPriority, mainstreamMin int) int {
	if mainstreamMin > 0 {
		leftMain := left.episodes >= mainstreamMin
		rightMain := right.episodes >= mainstreamMin
		if leftMain != rightMain {
			if leftMain {
				return 1
			}
			return -1
		}
	}
	if language == core.SubtitleUnknown {
		leftChinese := left.Language != core.SubtitleUnknown
		rightChinese := right.Language != core.SubtitleUnknown
		if leftChinese != rightChinese {
			if leftChinese {
				return 1
			}
			return -1
		}
	}
	switch priority {
	case preferCompatibility:
		if order := cmp.Compare(compatibilityScore(left), compatibilityScore(right)); order != 0 {
			return order
		}
		if order := compareVersionQuality(left, right); order != 0 {
			return order
		}
	case preferSize:
		leftKnown, rightKnown := left.typicalBytes > 0, right.typicalBytes > 0
		if leftKnown != rightKnown {
			if leftKnown {
				return 1
			}
			return -1
		}
		if leftKnown {
			if order := cmp.Compare(right.typicalBytes, left.typicalBytes); order != 0 {
				return order
			}
		} else {
			if left.Height > 0 && right.Height > 0 && left.Height != right.Height {
				return cmp.Compare(right.Height, left.Height)
			}
			if order := cmp.Compare(codecCompressionRank(left.Video), codecCompressionRank(right.Video)); order != 0 {
				return order
			}
		}
	default:
		if order := compareVersionQuality(left, right); order != 0 {
			return order
		}
	}
	if order := cmp.Compare(left.episodes, right.episodes); order != 0 {
		return order
	}
	return cmp.Compare(left.count, right.count)
}

func recommendedVersionIn(versions []versionOption, language core.SubtitleLanguage, priority versionPriority) int {
	mainstreamMin := mainstreamEpisodeThreshold(versions, language)
	best := -1
	for index, option := range versions {
		if !versionMatchesLanguage(option, language) {
			continue
		}
		if best < 0 || compareVersionOptions(option, versions[best], language, priority, mainstreamMin) > 0 {
			best = index
		}
	}
	return best
}

func (s *State) sortVersions() {
	if len(s.Versions) <= 1 {
		return
	}
	language, priority := s.VersionLanguage, s.VersionPriority
	mainstreamMin := mainstreamEpisodeThreshold(s.Versions, language)
	slices.SortStableFunc(s.Versions, func(left, right versionOption) int {
		leftMatch := versionMatchesLanguage(left, language)
		rightMatch := versionMatchesLanguage(right, language)
		if leftMatch != rightMatch {
			if leftMatch {
				return -1
			}
			return 1
		}
		return -compareVersionOptions(left, right, language, priority, mainstreamMin)
	})
}

func (s *State) recommendedVersion() int {
	return recommendedVersionIn(s.Versions, s.VersionLanguage, s.VersionPriority)
}

func (s *State) applyRecommendedVersion() {
	s.ManualVersion = false
	s.sortVersions()
	index := s.recommendedVersion()
	if index < 0 {
		s.Rule = Rule{}
		s.Checked = map[string]bool{}
		return
	}
	s.pickVersion(s.Versions[index].ReleaseVersion, false)
}

func (s *State) pickVersion(version core.ReleaseVersion, manual bool) {
	s.Rule = Rule{Version: &version}
	s.ManualVersion = manual
	s.OnlyMatches = true
	s.EpisodeFilter = ""
	s.Checked = map[string]bool{}
	s.initSelection()
}

type groupCandidate struct {
	index        int
	nonEmpty     bool
	langTier     int
	rawPenalty   bool
	hasVersions  bool
	bestEpisodes int
	bestOption   versionOption
	truncated    bool
	count        int
}

func isRawGroupName(name string) bool {
	return strings.Contains(name, "生肉") || strings.Contains(name, "不明字幕")
}

func (s *State) evaluateGroupCandidate(index int) groupCandidate {
	cand := groupCandidate{index: index}
	if index < len(s.GroupCounts) {
		cand.count = s.GroupCounts[index]
		cand.truncated = cand.count == -1
	}
	var preview []source.Resource
	if index < len(s.GroupPreviews) {
		preview = s.GroupPreviews[index]
	}
	cand.nonEmpty = len(preview) > 0 || cand.count > 0 || cand.truncated
	if len(preview) == 0 {
		return cand
	}
	resources := buildResources(preview)
	versions, _, _ := collectVersions(resources)
	cand.hasVersions = len(versions) > 0
	for _, option := range versions {
		if s.VersionLanguage != core.SubtitleUnknown && option.Language&s.VersionLanguage != 0 {
			cand.langTier = 2
			break
		}
		if option.Language != core.SubtitleUnknown && cand.langTier < 1 {
			cand.langTier = 1
		}
	}
	if cand.langTier == 0 && index < len(s.Groups) && isRawGroupName(s.Groups[index]) {
		cand.rawPenalty = true
	}
	if cand.hasVersions {
		lang := s.VersionLanguage
		if recommendedVersionIn(versions, lang, s.VersionPriority) < 0 {
			lang = core.SubtitleUnknown
		}
		if best := recommendedVersionIn(versions, lang, s.VersionPriority); best >= 0 {
			cand.bestOption = versions[best]
			cand.bestEpisodes = cand.bestOption.episodes
		}
	}
	if cand.bestEpisodes == 0 {
		eps := map[string]bool{}
		for _, r := range resources {
			if r.Episode != "" {
				eps[r.Episode] = true
			}
		}
		cand.bestEpisodes = len(eps)
	}
	return cand
}

func compareGroupCandidates(left, right groupCandidate, language core.SubtitleLanguage, priority versionPriority) int {
	if left.nonEmpty != right.nonEmpty {
		if left.nonEmpty {
			return 1
		}
		return -1
	}
	if order := cmp.Compare(left.langTier, right.langTier); order != 0 {
		return order
	}
	if left.rawPenalty != right.rawPenalty {
		if !left.rawPenalty {
			return 1
		}
		return -1
	}
	if left.hasVersions != right.hasVersions {
		if left.hasVersions {
			return 1
		}
		return -1
	}
	maxEp := max(left.bestEpisodes, right.bestEpisodes)
	if maxEp >= 3 {
		threshold := max(2, (maxEp+1)/2)
		leftMain, rightMain := left.bestEpisodes >= threshold, right.bestEpisodes >= threshold
		if leftMain != rightMain {
			if leftMain {
				return 1
			}
			return -1
		}
	}
	if left.hasVersions && right.hasVersions {
		if order := compareVersionOptions(left.bestOption, right.bestOption, language, priority, 0); order != 0 {
			return order
		}
	}
	if order := cmp.Compare(left.bestEpisodes, right.bestEpisodes); order != 0 {
		return order
	}
	if left.truncated != right.truncated {
		if left.truncated {
			return 1
		}
		return -1
	}
	return cmp.Compare(left.count, right.count)
}

func (s *State) bestDefaultGroup() int {
	if len(s.Groups) == 0 || len(s.GroupIDs) == 0 {
		return -1
	}
	hasPreviews := slices.ContainsFunc(s.GroupPreviews, func(rows []source.Resource) bool { return len(rows) > 0 })
	if !hasPreviews {
		best := -1
		for index, count := range s.GroupCounts {
			if index < len(s.GroupIDs) && s.GroupIDs[index] != "" && count >= 0 && (best < 0 || count > s.GroupCounts[best]) {
				best = index
			}
		}
		return best
	}
	bestIndex := -1
	var bestCand groupCandidate
	for index := range s.Groups {
		if index >= len(s.GroupIDs) || s.GroupIDs[index] == "" {
			continue
		}
		cand := s.evaluateGroupCandidate(index)
		if bestIndex < 0 || compareGroupCandidates(cand, bestCand, s.VersionLanguage, s.VersionPriority) > 0 {
			bestIndex = index
			bestCand = cand
		}
	}
	return bestIndex
}

func (s *State) alternativeGroupWithLanguage(language core.SubtitleLanguage) int {
	if language == core.SubtitleUnknown {
		return -1
	}
	bestIndex := -1
	var bestCand groupCandidate
	for index := range s.Groups {
		if index == s.Group || index >= len(s.GroupIDs) || s.GroupIDs[index] == "" {
			continue
		}
		cand := s.evaluateGroupCandidate(index)
		if cand.langTier < 2 {
			continue
		}
		if bestIndex < 0 || compareGroupCandidates(cand, bestCand, language, s.VersionPriority) > 0 {
			bestIndex = index
			bestCand = cand
		}
	}
	return bestIndex
}

func (s *State) groupPreviewSummary(index int) string {
	if index < 0 || index >= len(s.GroupPreviews) || len(s.GroupPreviews[index]) == 0 {
		return ""
	}
	versions, _, _ := collectVersions(buildResources(s.GroupPreviews[index]))
	if len(versions) == 0 {
		return ""
	}
	var heights []int
	var combinedLang core.SubtitleLanguage
	containers := map[string]bool{}
	for _, option := range versions {
		if option.Height > 0 && !slices.Contains(heights, option.Height) {
			heights = append(heights, option.Height)
		}
		combinedLang |= option.Language
		if option.Container != "" {
			containers[option.Container] = true
		}
	}
	slices.SortFunc(heights, func(a, b int) int { return cmp.Compare(b, a) })
	var parts []string
	if len(heights) > 0 {
		limit := min(2, len(heights))
		resParts := make([]string, 0, limit)
		for _, h := range heights[:limit] {
			resParts = append(resParts, fmt.Sprintf("%dp", h))
		}
		parts = append(parts, strings.Join(resParts, "/"))
	}
	if combinedLang != core.SubtitleUnknown {
		parts = append(parts, languageLabel(combinedLang))
	}
	if len(containers) == 1 {
		for c := range containers {
			parts = append(parts, c)
		}
	}
	return strings.Join(parts, " · ")
}

func collisionEpisodes(matches []Resource) int {
	byEpisode := map[string][]Resource{}
	for _, r := range matches {
		if r.Episode == "" {
			continue
		}
		byEpisode[r.Episode] = append(byEpisode[r.Episode], r)
	}
	collisions := 0
	for _, list := range byEpisode {
		if len(list) <= 1 {
			continue
		}
		titlesByRev := map[int]string{}
		for _, r := range list {
			rev := episodeRevision(r)
			norm := normalize(r.Title)
			if prev, exists := titlesByRev[rev]; exists && prev != norm {
				collisions++
				break
			}
			titlesByRev[rev] = norm
		}
	}
	return collisions
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
		return "字幕未标"
	}
}

func versionCoreLabel(version core.ReleaseVersion) string {
	var parts []string
	if version.Height > 0 {
		parts = append(parts, fmt.Sprintf("%dp", version.Height))
	}
	parts = append(parts, languageLabel(version.Language))
	return strings.Join(parts, " · ")
}

func versionTechLabel(version core.ReleaseVersion) string {
	var parts []string
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

func versionLabel(version core.ReleaseVersion) string {
	coreText := versionCoreLabel(version)
	techText := versionTechLabel(version)
	if techText == "" {
		return coreText
	}
	return coreText + " · " + techText
}

func versionMetricsLabel(option versionOption, previewOnly bool) string {
	var parts []string
	switch {
	case option.episodes > 0 && previewOnly:
		parts = append(parts, fmt.Sprintf("已预览 %d 集", option.episodes))
	case option.episodes > 0:
		parts = append(parts, fmt.Sprintf("共 %d 集", option.episodes))
	case previewOnly:
		parts = append(parts, fmt.Sprintf("已预览 %d 条", option.count))
	default:
		parts = append(parts, fmt.Sprintf("共 %d 条", option.count))
	}
	if option.typicalBytes > 0 {
		parts = append(parts, "约 "+byteSizeText(option.typicalBytes)+"/集")
	}
	return strings.Join(parts, " · ")
}

func (s *State) versionRecommendationNote(option versionOption) string {
	switch s.VersionPriority {
	case preferCompatibility:
		if isStrictlyCompatible(option) {
			return "优先推荐播放兼容性更好的版本。"
		}
		return "该字幕组未提供更通用的 MP4 版本，已推荐相对更易播放的版本。"
	case preferSize:
		if option.typicalBytes > 0 {
			return "按主力版本里单集体积较小推荐，约 " + byteSizeText(option.typicalBytes) + "/集。"
		}
		return "来源未提供文件大小，按预计体积较小的版本推荐。"
	default:
		if option.Height > 0 {
			reason := fmt.Sprintf("按画质与覆盖集数推荐，优先 %dp", option.Height)
			if option.Source != "" {
				reason += " · " + option.Source
			}
			return reason + "。"
		}
		return "按当前字幕组主力发布版本推荐。"
	}
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
	s.OnlyMatches = true
	s.EpisodeFilter = ""
	s.ShowEpisodeDetails = false
	s.DownloadExistingChoice = true
	s.ResourceScroll = ui.ScrollState{}
	if manual {
		s.Rule = cloneRule(s.TitleRuleDraft)
		if !hasTitleConditions(s.Rule) {
			s.TitleTemplateID = ""
			s.TitleManualInput = false
		}
		s.initSelection()
	} else {
		s.applyRecommendedVersion()
	}
}

func setupChoice(c *ui.Context, key, label string, active bool, action func()) {
	pal := colors(c)
	button := compactSecondary(c, key, label, action)
	if active {
		button.Background(pal.accentWash).BorderColor(pal.accent).TextColor(pal.accentInk)
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

func radioDot(c *ui.Context, active bool) {
	pal := colors(c)
	border := pal.outline
	if active {
		border = pal.accent
	}
	ui.Box(c).Size(16, 16).Radius(8).Border(1.5, border).Center().Children(func() {
		if active {
			ui.Box(c).Size(8, 8).Radius(4).Background(pal.accent)
		}
	})
}

func (s *State) setVersionLanguage(lang core.SubtitleLanguage) {
	if s.VersionLanguage == lang {
		return
	}
	s.VersionLanguage = lang
	s.ShowAllVersions = false
	s.saveVersionPreferences()
	s.applyRecommendedVersion()
}

func (s *State) setVersionPriority(priority versionPriority) {
	s.VersionPriority = priority
	s.ManualVersion = false
	s.ShowAllVersions = false
	s.saveVersionPreferences()
	s.applyRecommendedVersion()
}

func (s *State) setDownloadExisting(download bool) {
	s.DownloadExistingChoice = download
	choices, _ := s.episodeChoices(s.matchingResources())
	for _, r := range s.resources() {
		s.Checked[r.ID] = false
	}
	if download {
		for _, r := range choices {
			s.Checked[r.ID] = true
		}
	}
}

func (s *State) detailPreviewReady() bool {
	return s.hasGroup() && !s.DetailLoading && s.DetailError == "" && s.HistoryError == "" && (!s.HistoryLoading || s.HistoryPreviewOnly)
}

func (s *State) focusedDetail(c *ui.Context, a Anime) {
	embeddedFooter := textScale(c) > 1
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		ui.Scroll(c.Key("focused-detail-scroll")).TrackScroll(&s.ResourceScroll).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(16, pageInsetX(c), 28).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(760).Gap(20).Children(func() {
					s.focusedDetailHeader(c, a)
					switch {
					case s.DetailLoading:
						s.focusedCard(c, func() { pageLoading(c, "正在加载字幕组…") })
					case s.DetailError != "":
						s.focusedCard(c, func() { s.empty(c, "字幕组加载失败", s.DetailError, "重试加载", s.refreshDetail) })
					case len(s.Groups) == 0:
						s.focusedCard(c, func() {
							s.empty(c, "暂无字幕组", "来源尚未提供这部番剧的字幕组信息。", "重新加载", s.refreshDetail)
						})
					case !s.hasGroup() || s.ChoosingGroup:
						s.focusedCard(c, func() {
							ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
								heading(c, "选择字幕组")
								ui.Spacer(c)
								if s.hasGroup() {
									compactQuiet(c, "cancel-choose-group", "返回", func() { s.ChoosingGroup = false }).TextColor(colors(c).accentInk)
								}
							})
							s.groupPicker(c)
						})
					case s.HistoryLoading && !s.HistoryPreviewOnly:
						s.focusedCard(c, func() { pageLoading(c, "正在加载发布记录…") })
					case s.HistoryError != "":
						s.focusedCard(c, func() { s.empty(c, "发布记录加载失败", s.HistoryError, "重试加载资源", s.refreshHistory) })
					case s.TitleSetup:
						s.focusedTitleSetup(c)
					default:
						s.focusedVersionCard(c)
						if s.Rule.Version != nil {
							s.focusedEpisodesCard(c)
						}
					}
					if embeddedFooter {
						s.detailFooter(c, true)
					}
				})
			})
		})
		if !embeddedFooter {
			s.detailFooter(c, false)
		}
	})
}

func (s *State) focusedCard(c *ui.Context, body func()) {
	pal := colors(c)
	ui.Column(c).FillWidth().Padding(20, 24).Gap(14).Radius(radiusPanel).Background(pal.surface).Border(1, pal.line).Children(body)
}

func (s *State) focusedDetailHeader(c *ui.Context, a Anime) {
	pal := colors(c)
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			backLabel := "返回番剧列表"
			backPage := "browse"
			if s.DetailReturn == "following" {
				backLabel = "返回我的追番"
				backPage = "following"
			} else if s.DetailReturn == "downloads" {
				backLabel = "返回下载"
				backPage = "downloads"
			} else if s.DetailReturn == browseDetailReturn {
				backLabel = "返回番剧详情"
			}
			back := ui.ButtonBase(c.Key("back-browse")).Label(backLabel).
				Padding(8, 12, 8, 4).Gap(8).AlignItems(ui.Center).Radius(radiusSmall).
				FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.muted).Cursor(ui.CursorPointer)
			if back.Hovered() {
				back.Background(pal.chrome).TextColor(pal.ink)
			}
			tactile(back).Children(func() {
				icon(c, "back")
				ui.Text(c, backLabel)
			}).OnClick(func() { s.enterPage(backPage) })
		})
		ui.Row(c).Wrap().Gap(16).AlignItems(ui.Center).Children(func() {
			s.featuredCover(c, a, 52, 72)
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				pageTitle(c, a.Title)
				caption(c, dayName(a.Day)+" · 来源：蜜柑计划")
			})
		})
	})
}

func (s *State) focusedGroupStrip(c *ui.Context) {
	pal := colors(c)
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		caption(c, "字幕组："+s.Groups[s.Group])
		if len(s.Groups) > 1 {
			compactSecondary(c, "change-group", "更换", func() { s.ChoosingGroup = true }).TextColor(pal.accentInk).Disabled(s.Busy)
		}
	})
}

func (s *State) focusedVersionCard(c *ui.Context) {
	pal := colors(c)
	recommended := s.recommendedVersion()
	var matchingIndices []int
	for index, option := range s.Versions {
		if versionMatchesLanguage(option, s.VersionLanguage) {
			matchingIndices = append(matchingIndices, index)
		}
	}
	singleUnified := len(s.Versions) == 1 && len(matchingIndices) == 1 && s.VersionLanguage == core.SubtitleUnknown

	s.focusedCard(c, func() {
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			heading(c, "1. 观看版本")
			ui.Spacer(c)
			s.focusedGroupStrip(c)
		})
		if s.HistoryPreviewOnly {
			caption(c, "已基于最新记录生成推荐，正在后台加载完整剧集…")
		}
		if !singleUnified && len(s.Versions) > 0 {
			ui.Row(c).Wrap().Gap(16).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
					caption(c, "字幕")
					for _, choice := range []struct {
						value core.SubtitleLanguage
						label string
					}{{0, "不限"}, {core.SubtitleSimplified, "简体"}, {core.SubtitleTraditional, "繁体"}} {
						setupChoice(c, fmt.Sprintf("language-%d", choice.value), choice.label, s.VersionLanguage == choice.value, func() {
							s.setVersionLanguage(choice.value)
						})
					}
				})
				if len(matchingIndices) > 1 {
					ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
						caption(c, "偏好")
						for index, label := range []string{"画质", "兼容性", "省空间"} {
							priority := versionPriority(index)
							setupChoice(c, fmt.Sprintf("priority-%d", index), label, !s.ManualVersion && s.VersionPriority == priority, func() {
								s.setVersionPriority(priority)
							})
						}
					})
				}
			})
		}

		switch {
		case len(s.Versions) == 0:
			caption(c, "这个字幕组的版本暂未自动识别，可以直接按原标题自选。")
			primary(c, "fallback-title-setup", "按原标题自选", func() { s.switchVersionMode(true) }).AlignSelf(ui.Start)
		case len(matchingIndices) == 0:
			langText := languageLabel(s.VersionLanguage)
			caption(c, fmt.Sprintf("当前字幕组没有标注「%s」的版本。", langText))
			ui.Row(c).Wrap().Gap(8).Children(func() {
				compactSecondary(c, "reset-language-any", "查看全部语言", func() { s.setVersionLanguage(core.SubtitleUnknown) })
				if alt := s.alternativeGroupWithLanguage(s.VersionLanguage); alt >= 0 {
					compactSecondary(c, "switch-alt-group", fmt.Sprintf("切到「%s」（含%s）", s.Groups[alt], langText), func() { s.chooseGroup(alt) }).TextColor(pal.accentInk).BorderColor(pal.accent).Background(pal.accentWash)
				}
				if len(s.Groups) > 1 {
					compactQuiet(c, "open-group-picker-empty", "更换字幕组", func() { s.ChoosingGroup = true }).TextColor(pal.accentInk)
				}
			})
		default:
			if singleUnified {
				caption(c, "该字幕组提供统一版本，直接确认即可追番。")
			} else if recommended >= 0 {
				caption(c, s.versionRecommendationNote(s.Versions[recommended]))
			}
			visibleCount := len(matchingIndices)
			const initialVersionLimit = 4
			if !s.ShowAllVersions && visibleCount > initialVersionLimit {
				visibleCount = initialVersionLimit
			}
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				for _, index := range matchingIndices[:visibleCount] {
					option := s.Versions[index]
					active := s.Rule.Version != nil && *s.Rule.Version == option.ReleaseVersion
					isRec := index == recommended
					s.versionChoiceRow(c, index, option, active, isRec)
				}
			})
			if len(matchingIndices) > initialVersionLimit {
				toggleLabel := fmt.Sprintf("展开更多版本（另有 %d 个）", len(matchingIndices)-initialVersionLimit)
				if s.ShowAllVersions {
					toggleLabel = "收起更多版本"
				}
				compactQuiet(c, "toggle-all-versions", toggleLabel, func() { s.ShowAllVersions = !s.ShowAllVersions }).TextColor(pal.accentInk).AlignSelf(ui.Start)
			}
			if s.Rule.Version != nil {
				if collisions := collisionEpisodes(s.matchingResources()); collisions > 0 {
					ui.Column(c).FillWidth().Padding(12, 16).Gap(8).Radius(radiusControl).Background(pal.warningWash).Children(func() {
						ui.Textf(c, "这个版本下有 %d 集存在多条不同发布，可能包含未识别的版本差异。", collisions).FontSize(textSize(c, textMeta)).TextColor(pal.warningInk)
						ui.Row(c).Wrap().Gap(8).Children(func() {
							compactQuiet(c, "collision-show-details", "查看明细", func() { s.ShowEpisodeDetails = true }).TextColor(pal.accentInk)
							compactQuiet(c, "collision-title-setup", "按原标题精确区分", func() { s.switchVersionMode(true) }).TextColor(pal.accentInk)
						})
					})
				}
			}
		}

		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if s.CollectionCount > 0 || s.UnclassifiedCount > 0 {
				var extras []string
				if s.CollectionCount > 0 {
					extras = append(extras, fmt.Sprintf("%d 条合集", s.CollectionCount))
				}
				if s.UnclassifiedCount > 0 {
					extras = append(extras, fmt.Sprintf("%d 条未归类资源", s.UnclassifiedCount))
				}
				caption(c, "另有 "+strings.Join(extras, "、")+"，可在明细或按原标题自选中查看。")
			}
			ui.Spacer(c)
			if len(s.Versions) > 0 {
				compactQuiet(c, "switch-title-rules", "都不是想要的？按原标题自选", func() { s.switchVersionMode(true) }).TextColor(pal.accentInk)
			}
		})
	})
}

func (s *State) versionChoiceRow(c *ui.Context, index int, option versionOption, active, recommended bool) {
	pal := colors(c)
	coreText := versionCoreLabel(option.ReleaseVersion)
	techText := versionTechLabel(option.ReleaseVersion)
	metrics := versionMetricsLabel(option, s.HistoryPreviewOnly)
	accLabel := coreText + " · " + metrics
	if techText != "" {
		accLabel = versionLabel(option.ReleaseVersion) + " · " + metrics
	}
	if recommended {
		accLabel = "推荐 · " + accLabel
	}
	btn := ui.ButtonBase(c.Key(fmt.Sprintf("version-%d", index))).Label(accLabel).
		Row().FillWidth().Gap(12).AlignItems(ui.Start).Padding(12, 16).Radius(radiusControl).
		Border(1, pal.line).Background(pal.canvas).Cursor(ui.CursorPointer)
	if active {
		btn.BorderColor(pal.accent).Background(pal.accentWash)
	} else if btn.Hovered() {
		btn.BorderColor(pal.hoverLine)
	}
	tactile(btn).Children(func() {
		ui.Box(c).Padding(3, 0, 0, 0).Children(func() { radioDot(c, active) })
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				if recommended {
					statusPill(c, "推荐", pal.accentInk, pal.surface)
				}
				ui.Text(c, coreText).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(pal.ink)
				ui.Spacer(c)
				caption(c, metrics)
			})
			if techText != "" {
				caption(c, techText)
			}
		})
	}).OnClick(func() {
		s.pickVersion(option.ReleaseVersion, !recommended)
	})
}

func (s *State) focusedEpisodesCard(c *ui.Context) {
	matches := s.matchingResources()
	choices, byEpisode := s.episodeChoices(matches)
	selected, _ := s.selected()
	totalSize := resourceSizes(choices)
	selectedSize := resourceSizes(selected)
	allSelected := len(choices) > 0 && len(selected) == len(choices)
	noneSelected := len(selected) == 0
	pal := colors(c)

	recognizedCount := 0
	for _, r := range choices {
		if r.Episode != "" {
			recognizedCount++
		}
	}
	unit := "集"
	sectionHeading := fmt.Sprintf("2. 已播出的 %d 集", len(choices))
	if recognizedCount < len(choices) {
		unit = "个资源"
		sectionHeading = fmt.Sprintf("2. 匹配的 %d 个资源", len(choices))
	}

	s.focusedCard(c, func() {
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if len(choices) > 0 {
				heading(c, sectionHeading)
			} else {
				heading(c, "2. 追番方式")
			}
			ui.Spacer(c)
			if len(choices) > 0 {
				caption(c, totalSize)
			}
		})

		if len(choices) == 0 {
			message := "当前没有已播出的匹配资源，开启后将按条件自动下载后续更新。"
			if len(s.resources()) == 0 {
				message = "这组暂无已发布资源，开启追番后将按条件检查后续更新。"
			}
			ui.Text(c, message).FontSize(textSize(c, textMeta)).TextColor(pal.warningInk).Padding(12).Background(pal.warningWash).Radius(radiusControl)
			return
		}

		ui.Column(c).FillWidth().Gap(8).Children(func() {
			downloadBtn := ui.ButtonBase(c.Key("download-existing-all")).
				Label(fmt.Sprintf("立即下载全部 %d %s，并自动追后续更新", len(choices), unit)).
				Row().FillWidth().Gap(12).AlignItems(ui.Center).Padding(12, 16).Radius(radiusControl).
				Border(1, pal.line).Background(pal.canvas).Cursor(ui.CursorPointer)
			if allSelected {
				downloadBtn.BorderColor(pal.accent).Background(pal.accentWash)
			} else if downloadBtn.Hovered() {
				downloadBtn.BorderColor(pal.hoverLine)
			}
			tactile(downloadBtn).Children(func() {
				radioDot(c, allSelected)
				ui.Textf(c, "立即下载全部 %d %s（%s），并自动追后续更新", len(choices), unit, totalSize).FontSize(textSize(c, textBody)).TextColor(pal.ink).Grow(1).MinWidth(0)
			}).OnClick(func() { s.setDownloadExisting(true) })

			futureBtn := ui.ButtonBase(c.Key("download-future-only")).
				Label("暂不下载已有集数，只自动追后续更新").
				Row().FillWidth().Gap(12).AlignItems(ui.Center).Padding(12, 16).Radius(radiusControl).
				Border(1, pal.line).Background(pal.canvas).Cursor(ui.CursorPointer)
			if noneSelected {
				futureBtn.BorderColor(pal.accent).Background(pal.accentWash)
			} else if futureBtn.Hovered() {
				futureBtn.BorderColor(pal.hoverLine)
			}
			tactile(futureBtn).Children(func() {
				radioDot(c, noneSelected)
				ui.Text(c, "暂不下载已有集数，只自动追后续更新").FontSize(textSize(c, textBody)).TextColor(pal.ink).Grow(1).MinWidth(0)
			}).OnClick(func() { s.setDownloadExisting(false) })
		})

		if !allSelected && !noneSelected {
			caption(c, fmt.Sprintf("已自定义选择 %d / %d %s（%s）", len(selected), len(choices), unit, selectedSize))
		}

		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if s.EpisodeFilter != "" {
				statusPill(c, "只看"+episodeLabel(s.EpisodeFilter), pal.accentInk, pal.accentWash)
				compactQuiet(c, "clear-episode-filter", "返回全部集数", func() { s.toggleEpisodeFilter(s.EpisodeFilter) }).TextColor(pal.accentInk)
			}
			ui.Spacer(c)
			toggleText := fmt.Sprintf("自选集数 / 查看明细 (%d) ▾", len(choices))
			if s.ShowEpisodeDetails {
				toggleText = "收起明细 ▴"
			}
			compactQuiet(c, "toggle-episode-details", toggleText, func() { s.ShowEpisodeDetails = !s.ShowEpisodeDetails }).TextColor(pal.accentInk)
		})

		if s.ShowEpisodeDetails {
			ui.Divider(c).Background(pal.line)
			s.focusedEpisodesDetailList(c, choices, byEpisode)
		}
	})
}

func (s *State) hasAnyDownloadForResources() bool {
	for _, r := range s.resources() {
		if _, _, found := s.downloadForResource(r); found {
			return true
		}
	}
	return false
}

func (s *State) focusedEpisodesDetailList(c *ui.Context, choices []Resource, byEpisode map[string]Resource) {
	selected, _ := s.selected()
	all := len(choices) > 0 && len(selected) == len(choices)
	ui.Row(c).Wrap().Gap(controlGap).AlignItems(ui.Center).Children(func() {
		ui.Checkbox(c.Key("select-all"), &all, "全选符合条件的集数").Disabled(len(choices) == 0).OnChange(func() {
			s.setDownloadExisting(all)
		})
		compactQuiet(c, "select-none", "清空选择", func() { s.setDownloadExisting(false) }).TextColor(colors(c).muted)
		ui.Spacer(c)
		ui.Checkbox(c.Key("only-matches"), &s.OnlyMatches, "只看匹配")
		if !s.TitleSetup {
			ui.Checkbox(c.Key("show-original-titles"), &s.ShowOriginalTitles, "显示原标题")
		}
		if s.hasAnyDownloadForResources() {
			ui.Checkbox(c.Key("only-downloaded"), &s.OnlyDownloaded, "只看已有任务")
		}
	})

	ui.Column(c).FillWidth().Gap(0).Children(func() {
		visible := 0
		for _, r := range s.previewResources() {
			res := r
			ok, _ := matchResource(res, s.Rule)
			if s.OnlyMatches && !ok {
				continue
			}
			if s.EpisodeFilter != "" && res.Episode != s.EpisodeFilter {
				continue
			}
			if choice, exists := byEpisode[res.Episode]; exists && s.EpisodeFilter == "" &&
				!s.OnlyDownloaded && choice.ID != res.ID {
				continue
			}
			if s.OnlyDownloaded {
				if _, _, found := s.downloadForResource(res); !found {
					continue
				}
			}
			visible++
			s.focusedResourceRow(c, res, byEpisode[res.Episode].ID == res.ID)
		}
		if visible == 0 && len(s.previewResources()) > 0 {
			s.empty(c, "没有符合筛选的资源", "试试清除筛选，看看这个字幕组的全部资源。", "清除筛选", s.clearPreviewFilters)
		}
	})
}

func (s *State) focusedResourceRow(c *ui.Context, r Resource, episodeChoice bool) {
	ok, reason := matchResource(r, s.Rule)
	pal := colors(c)
	showTitle := s.TitleSetup || s.ShowOriginalTitles || r.Episode == "" || s.EpisodeFilter != "" || !s.OnlyMatches
	ui.Column(c.Key("resource-"+r.ID)).FillWidth().Padding(10, 0).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(pal.line).Transition(rowTransition).Children(func() {
		ui.Row(c).Wrap().Gap(controlGap).AlignItems(ui.Center).Children(func() {
			on := s.Checked[r.ID] && ok
			ui.Checkbox(c.Key("pick-"+r.ID), &on, "").Label("选择 " + r.ID).Disabled(!ok).OnChange(func() {
				s.setResourceChecked(r, on)
				selected, _ := s.selected()
				s.DownloadExistingChoice = len(selected) > 0
			})
			ui.Text(c, episodeLabel(r.Episode)).FontSize(textSize(c, textBody)).FontWeight(600).TextColor(pal.ink)
			if count := s.duplicates(r.Episode); count > 1 && ok && episodeChoice {
				ep := r.Episode
				fg, bg := pal.warningInk, pal.warningWash
				label := fmt.Sprintf("同集 %d 条 · 更换", count)
				if rev := episodeRevision(r); rev > 1 {
					label = fmt.Sprintf("已选 v%d · 更换发布 (%d)", rev, count)
				}
				if s.EpisodeFilter == ep {
					fg, bg = pal.accentInk, pal.accentWash
					label = "返回全部集数"
				}
				compactQuiet(c, "filter-episode-"+r.ID, label, func() { s.toggleEpisodeFilter(ep) }).TextColor(fg).Background(bg).Padding(4, 8)
			}
			if d, idx, found := s.downloadForResource(r); found {
				s.resourceDownloadBadges(c, r, d, idx)
			}
			ui.Spacer(c)
			if r.Published != "" {
				caption(c, r.Published)
			}
			if bytes := sizeBytes(r.Bytes, r.MB); bytes > 0 {
				caption(c, byteSizeText(bytes))
			} else {
				caption(c, "大小未知")
			}
		})
		if showTitle {
			if s.TitleSetup {
				s.originalTitle(c, r)
			} else {
				ui.Text(c, r.Title).FontSize(textSize(c, textMeta)).LineHeight(1.5).Selectable().TextColor(pal.muted)
			}
		}
		if !ok {
			caption(c, reason)
		}
	})
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
		s.setVersionLanguage(languages[index])
	})
	heading(c, "优先考虑")
	activePriority := int(s.VersionPriority)
	if s.ManualVersion {
		activePriority = -1
	}
	setupSegments(c, "version-priority", "优先考虑", activePriority, []string{"画质", "兼容性", "省空间"}, func(index int) {
		s.setVersionPriority(versionPriority(index))
	})
	compactQuiet(c, "switch-title-rules", "按原标题追番", func() { s.switchVersionMode(true) }).TextColor(pal.accentInk).AlignSelf(ui.Start)
}

func (s *State) versionSummary(c *ui.Context) {
	recommended := s.recommendedVersion()
	ui.Column(c).Gap(12).Children(func() {
		heading(c, "选择版本")
		if recommended < 0 && len(s.Versions) == 0 {
			caption(c, "这个字幕组的版本暂未识别，可以按原标题追番。")
		} else if recommended < 0 && s.VersionLanguage != 0 {
			caption(c, "没有标注这种字幕语言的版本。可以更换语言、字幕组，或查看原标题。")
		} else if recommended >= 0 {
			caption(c, s.versionRecommendationNote(s.Versions[recommended]))
		}
		selected := core.ReleaseVersion{}
		if s.Rule.Version != nil {
			selected = *s.Rule.Version
		}
		ui.Scroll(c.Key("version-choices")).MaxHeight(144).Children(func() {
			ui.RadioGroup(c, func() {
				for index, option := range s.Versions {
					if !versionMatchesLanguage(option, s.VersionLanguage) {
						continue
					}
					label := fmt.Sprintf("%s · %s", versionLabel(option.ReleaseVersion), versionMetricsLabel(option, s.HistoryPreviewOnly))
					if index == recommended {
						label = "推荐 · " + label
					}
					radio := ui.RadioBase(c.Key(fmt.Sprintf("version-%d", index)), &selected, option.ReleaseVersion)
					setupRadio(c, radio, label, s.Rule.Version != nil && selected == option.ReleaseVersion, func() {
						s.pickVersion(option.ReleaseVersion, index != recommended)
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
				ui.Text(c, resource.Title[offset:span[0]]).FontSize(textSize(c, textBody)).LineHeight(1.5).TextColor(pal.muted)
			}
			word := resource.Title[span[0]:span[1]]
			active := s.ActiveTitleRow == resource.ID && s.ActiveTitleWord == word
			choice := s.wordChoice(word)
			button := ui.ButtonBase(c.Key(fmt.Sprintf("title-word-%d", index))).
				Label("查看标题词「"+word+"」").
				Padding(3, 4).Radius(radiusSmall).Cursor(ui.CursorPointer).
				FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink)
			tactile(button)
			if active || choice == wordWant {
				button.Background(pal.accentWash).TextColor(pal.accentInk)
			} else if choice == wordAvoid {
				button.Background(pal.sunken).TextColor(pal.muted)
			} else if button.Hovered() {
				button.Background(pal.accentWash).TextColor(pal.accentInk)
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
			ui.Text(c, resource.Title[offset:]).FontSize(textSize(c, textBody)).LineHeight(1.5).TextColor(pal.muted)
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
		if s.wordChoice(word) != wordUnrestricted {
			compactQuiet(c, "title-rule-clear", "不再限制", func() { s.setWord(word, wordUnrestricted) }).TextColor(pal.muted).AlignSelf(ui.Start)
		}
	})
}
