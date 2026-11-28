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

type titleSample struct {
	Resource
	count    int
	episodes int
}

var (
	sampleLeadingGroup  = regexp.MustCompile(`^\s*(?:\[[^\]]+\]|【[^】]+】)\s*`)
	sampleBracketTokens = regexp.MustCompile(`(?:\[([^\]]+)\]|【([^】]+)】|\(([^)]+)\))`)
	sampleEpisodeStrip  = regexp.MustCompile(`(?i)(?:第\s*\d+(?:\.\d+)?\s*[集话話]|\bs\d{1,2}[\s._-]*e\s*p?\s*\d+(?:\.\d+)?(?:v\d+)?\b|\bep?\s*\d+(?:\.\d+)?(?:v\d+)?\b|(?:\s-\s|\[|【)\d{1,4}(?:\.\d+)?(?:v\d+)?(?:\s|\]|】|$))`)
	samplePureNumber    = regexp.MustCompile(`(?i)^(?:s\d{1,2}e)?\d{1,4}(?:\.\d+)?(?:v\d+)?$|^ep?\d+(?:v\d+)?$`)
)

func titlePatternKey(title string) string {
	text := core.Normalize(title)
	text = sampleLeadingGroup.ReplaceAllString(text, "")
	text = sampleEpisodeStrip.ReplaceAllString(text, " # ")
	return strings.Join(strings.Fields(text), " ")
}

func (s *State) titleSamples() []titleSample {
	var samples []titleSample
	indexByKey := map[string]int{}
	var episodeSets []map[string]bool
	for _, r := range s.resources() {
		key := titlePatternKey(r.Title)
		index, exists := indexByKey[key]
		if !exists {
			index = len(samples)
			indexByKey[key] = index
			samples = append(samples, titleSample{Resource: r})
			episodeSets = append(episodeSets, map[string]bool{})
		}
		samples[index].count++
		if r.Episode != "" {
			episodeSets[index][r.Episode] = true
		}
		if r.Published > samples[index].Published {
			samples[index].Resource = r
		}
	}
	for index := range samples {
		samples[index].episodes = len(episodeSets[index])
	}
	slices.SortStableFunc(samples, func(a, b titleSample) int {
		if order := cmp.Compare(b.episodes, a.episodes); order != 0 {
			return order
		}
		return cmp.Compare(b.count, a.count)
	})
	return samples
}

func sampleCandidateTokens(title string) []string {
	body := sampleLeadingGroup.ReplaceAllString(title, "")
	if body == "" {
		body = title
	}
	var out []string
	seen := map[string]bool{}
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		norm := normalize(raw)
		if raw == "" || len([]rune(raw)) < 2 || seen[norm] || samplePureNumber.MatchString(norm) {
			return
		}
		if strings.HasPrefix(raw, "第") && (strings.HasSuffix(raw, "集") || strings.HasSuffix(raw, "话") || strings.HasSuffix(raw, "話")) {
			return
		}
		seen[norm] = true
		out = append(out, raw)
	}
	for _, tag := range core.ReleaseTags(body) {
		add(tag)
	}
	for _, match := range sampleBracketTokens.FindAllStringSubmatch(body, -1) {
		content := strings.TrimSpace(match[1] + match[2] + match[3])
		if content == "" {
			continue
		}
		if len([]rune(content)) <= 16 && !strings.Contains(content, " ") {
			add(content)
		}
		for _, word := range titleWordPattern.FindAllString(content, -1) {
			add(word)
		}
	}
	for _, word := range titleWordPattern.FindAllString(body, -1) {
		if core.ReleaseWordMeaning(word) != "含义未识别" {
			add(word)
		}
	}
	return out
}

// extractSampleKeywords selects a compact set of literal title terms from
// sample that separate it from sibling releases while staying broad enough to
// match the other episodes of the same release series.
func extractSampleKeywords(sample Resource, all []Resource) []string {
	candidates := sampleCandidateTokens(sample.Title)
	if len(candidates) == 0 {
		return nil
	}
	type tokenStats struct {
		word       string
		norm       string
		df         int
		episodes   int
		sameEpDiff int
		recognized bool
	}
	sameEpOthers := 0
	for _, r := range all {
		if sample.Episode != "" && r.Episode == sample.Episode && r.ID != sample.ID {
			sameEpOthers++
		}
	}
	var stats []tokenStats
	for _, word := range candidates {
		norm := normalize(word)
		df := 0
		eps := map[string]bool{}
		sameEpMiss := 0
		for _, r := range all {
			has := strings.Contains(normalize(r.Title), norm)
			if has {
				df++
				if r.Episode != "" {
					eps[r.Episode] = true
				}
			} else if sample.Episode != "" && r.Episode == sample.Episode && r.ID != sample.ID {
				sameEpMiss++
			}
		}
		if len(all) > 1 && df == len(all) {
			continue
		}
		stats = append(stats, tokenStats{
			word:       word,
			norm:       norm,
			df:         df,
			episodes:   len(eps),
			sameEpDiff: sameEpMiss,
			recognized: core.ReleaseWordMeaning(word) != "含义未识别",
		})
	}
	if len(stats) == 0 {
		limit := min(2, len(candidates))
		return candidates[:limit]
	}

	slices.SortStableFunc(stats, func(a, b tokenStats) int {
		aMulti, bMulti := a.episodes > 1, b.episodes > 1
		if aMulti != bMulti {
			if aMulti {
				return -1
			}
			return 1
		}
		if order := cmp.Compare(b.sameEpDiff, a.sameEpDiff); order != 0 {
			return order
		}
		if a.recognized != b.recognized {
			if a.recognized {
				return -1
			}
			return 1
		}
		if order := cmp.Compare(b.episodes, a.episodes); order != 0 {
			return order
		}
		return cmp.Compare(b.df, a.df)
	})

	var chosen []string
	var chosenNorms []string
	overlaps := func(norm string) bool {
		for _, existing := range chosenNorms {
			if strings.Contains(existing, norm) || strings.Contains(norm, existing) {
				return true
			}
		}
		return false
	}

	// First eliminate same-episode competing releases using greedy set cover.
	var competing []Resource
	for _, r := range all {
		if sample.Episode != "" && r.Episode == sample.Episode && r.ID != sample.ID {
			competing = append(competing, r)
		}
	}
	for len(competing) > 0 && len(chosen) < 3 {
		bestIdx := -1
		bestElim := 0
		for i, st := range stats {
			if overlaps(st.norm) {
				continue
			}
			elim := 0
			for _, r := range competing {
				if !strings.Contains(normalize(r.Title), st.norm) {
					elim++
				}
			}
			if elim > bestElim || (elim == bestElim && elim > 0 && bestIdx >= 0 && st.episodes > stats[bestIdx].episodes) {
				bestElim = elim
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			break
		}
		picked := stats[bestIdx]
		chosen = append(chosen, picked.word)
		chosenNorms = append(chosenNorms, picked.norm)
		var still []Resource
		for _, r := range competing {
			if strings.Contains(normalize(r.Title), picked.norm) {
				still = append(still, r)
			}
		}
		competing = still
	}

	// Add up to 2 distinguishing terms total when no same-episode siblings existed
	// (or to anchor resolution/language alongside a custom discriminator).
	targetCount := 2
	if sameEpOthers > 0 && len(chosen) > 0 {
		targetCount = min(3, max(2, len(chosen)))
	}
	for _, st := range stats {
		if len(chosen) >= targetCount {
			break
		}
		if overlaps(st.norm) {
			continue
		}
		if len(chosen) > 0 && !st.recognized && st.sameEpDiff == 0 {
			continue
		}
		chosen = append(chosen, st.word)
		chosenNorms = append(chosenNorms, st.norm)
	}
	return chosen
}

func (s *State) pickTitleSample(sample Resource) {
	keywords := extractSampleKeywords(sample, s.resources())
	s.TitleTemplateID = sample.ID
	s.TitleManualInput = false
	s.OnlyMatches = true
	s.EpisodeFilter = ""
	s.DownloadExistingChoice = true
	s.Checked = map[string]bool{}
	s.applyRule(Rule{All: keywords}, "按样本生成条件")
}

func (s *State) titleTemplateResource() (Resource, bool) {
	if s.TitleTemplateID == "" {
		return Resource{}, false
	}
	for _, r := range s.resources() {
		if r.ID == s.TitleTemplateID {
			return r, true
		}
	}
	return Resource{}, false
}

func (s *State) focusedTitleSetup(c *ui.Context) {
	pal := colors(c)
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		if len(s.Versions) > 0 {
			compactQuiet(c, "switch-recommendation", "← 返回版本推荐", func() { s.switchVersionMode(false) }).TextColor(pal.accentInk)
		}
		ui.Spacer(c)
		s.focusedGroupStrip(c)
	})

	stepOne := s.TitleTemplateID == "" && !hasTitleConditions(s.Rule) && !s.TitleManualInput && len(s.resources()) > 0
	if stepOne {
		s.focusedTitleSamplePicker(c)
		return
	}
	s.focusedTitleRuleCard(c)
	if hasTitleConditions(s.Rule) {
		s.focusedEpisodesCard(c)
	}
}

func (s *State) focusedTitleSamplePicker(c *ui.Context) {
	pal := colors(c)
	samples := s.titleSamples()
	s.focusedCard(c, func() {
		heading(c, "第 1 步：点一条你想要的资源")
		caption(c, "系统会照着你选的这条，自动帮你整理追番规则。")
		list := samples
		if s.TitleShowAllSamples {
			list = nil
			for _, r := range s.previewResources() {
				list = append(list, titleSample{Resource: r, count: 1})
			}
		}
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			for i, sample := range list {
				item := sample
				btn := ui.ButtonBase(c.Key(fmt.Sprintf("title-sample-%d-%s", i, item.ID))).
					Label("选择模板 "+item.Title).
					Column().FillWidth().AlignItems(ui.Start).Gap(6).Padding(12, 16).Radius(radiusControl).
					Border(1, pal.line).Background(pal.canvas).Cursor(ui.CursorPointer)
				if btn.Hovered() {
					btn.BorderColor(pal.accent).Background(pal.accentWash)
				}
				tactile(btn).Children(func() {
					ui.Text(c, item.Title).FontSize(textSize(c, textBody)).LineHeight(1.5).TextColor(pal.ink)
					ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
						if !s.TitleShowAllSamples && item.episodes > 1 {
							caption(c, fmt.Sprintf("同类约 %d 集", item.episodes))
						} else if !s.TitleShowAllSamples && item.count > 1 {
							caption(c, fmt.Sprintf("同类 %d 条", item.count))
						} else if item.Episode != "" {
							caption(c, episodeLabel(item.Episode))
						}
						if bytes := sizeBytes(item.Bytes, item.MB); bytes > 0 {
							caption(c, byteSizeText(bytes))
						}
						ui.Spacer(c)
						ui.Text(c, "照这条追番 ›").FontSize(textSize(c, textControl)).FontWeight(500).TextColor(pal.accentInk)
					})
				}).OnClick(func() { s.pickTitleSample(item.Resource) })
			}
		})
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if len(s.resources()) > len(samples) {
				toggleLabel := fmt.Sprintf("查看全部 %d 条发布", len(s.resources()))
				if s.TitleShowAllSamples {
					toggleLabel = fmt.Sprintf("只看代表样本（%d 类）", len(samples))
				}
				compactQuiet(c, "toggle-all-samples", toggleLabel, func() { s.TitleShowAllSamples = !s.TitleShowAllSamples }).TextColor(pal.muted)
			}
			ui.Spacer(c)
			compactQuiet(c, "manual-title-input", "没有想要的？手动输入关键词 ›", func() { s.TitleManualInput = true }).TextColor(pal.accentInk)
		})
	})
}

func (s *State) focusedTitleRuleCard(c *ui.Context) {
	pal := colors(c)
	template, hasTemplate := s.titleTemplateResource()
	s.focusedCard(c, func() {
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if hasTemplate {
				heading(c, "1. 已按样本生成追番条件")
			} else {
				heading(c, "1. 按标题词追番")
			}
			ui.Spacer(c)
			if len(s.UndoFilters) > 0 {
				compactQuiet(c, "undo-filter", "撤销", s.undoFilter).TextColor(pal.accentInk)
			}
			if len(s.RedoFilters) > 0 {
				compactQuiet(c, "redo-filter", "重做", s.redoFilter).TextColor(pal.accentInk)
			}
			if len(s.resources()) > 0 {
				compactQuiet(c, "repick-title-sample", "重选一条", func() {
					s.TitleTemplateID = ""
					s.TitleManualInput = false
					s.Rule = Rule{}
					s.Checked = map[string]bool{}
					s.ActiveTitleWord, s.ActiveTitleRow = "", ""
					s.FilterFeedback = ""
				}).TextColor(pal.accentInk)
			}
		})

		if hasTemplate {
			ui.Column(c).FillWidth().Padding(12, 16).Gap(6).Radius(radiusControl).Background(pal.canvas).Children(func() {
				if s.TitleManualInput {
					caption(c, "点击下面标题里的任意词，可查看含义或加入包含 / 排除条件：")
					s.originalTitle(c, template)
				} else {
					ui.Text(c, template.Title).FontSize(textSize(c, textMeta)).LineHeight(1.5).TextColor(pal.muted)
				}
			})
		}

		if hasTitleConditions(s.Rule) {
			ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				for i, word := range s.Rule.All {
					w := word
					key := fmt.Sprintf("want-chip-%d", i)
					ui.Row(c.Key(key)).Gap(0).AlignItems(ui.Center).Children(func() {
						setupChoice(c, key+"-btn", "只要 "+w+" ×", true, func() { s.setWord(w, wordUnrestricted) })
					})
				}
				for i, word := range s.Rule.None {
					w := word
					key := fmt.Sprintf("avoid-chip-%d", i)
					compactSecondary(c, key+"-btn", "不要 "+w+" ×", func() { s.setWord(w, wordUnrestricted) }).Background(pal.sunken).TextColor(pal.muted)
				}
				for idx, g := range s.Rule.Any {
					groupIndex := idx
					compactSecondary(c, fmt.Sprintf("any-chip-%d", idx), "任一："+strings.Join(g, " / ")+" ×", func() {
						rule := cloneRule(s.Rule)
						rule.Any = slices.Delete(rule.Any, groupIndex, groupIndex+1)
						s.applyRule(rule, "移除任意词组")
					}).Background(pal.accentWash).BorderColor(pal.accent).TextColor(pal.accentInk)
				}
			})
		} else {
			caption(c, "添加至少一个标题词后即可追番。支持提前设置尚未发布的新番条件。")
		}

		if s.FilterFeedback != "" {
			caption(c, s.FilterFeedback)
		}

		tuneLabel := "▸ 微调条件 / 排除词 / 手动输入"
		if s.TitleManualInput {
			tuneLabel = "▾ 收起微调"
		}
		compactQuiet(c, "toggle-title-tune", tuneLabel, func() { s.TitleManualInput = !s.TitleManualInput }).TextColor(pal.accentInk).AlignSelf(ui.Start)

		if s.TitleManualInput || !hasTitleConditions(s.Rule) {
			ui.Divider(c).Background(pal.line)
			if !hasTemplate && len(s.resources()) > 0 {
				caption(c, "点击下面标题里的任意词，可查看含义或设置包含 / 不包含：")
				sampleRows := s.titleSamples()
				limit := min(8, len(sampleRows))
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					for _, sample := range sampleRows[:limit] {
						ui.Column(c).FillWidth().Padding(10, 12).Radius(radiusSmall).Background(pal.canvas).Children(func() {
							s.originalTitle(c, sample.Resource)
						})
					}
				})
			}
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				ui.Row(c).Wrap().Gap(controlGap).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c.Key("keyword"), &s.Keyword).Label("标题关键词").Placeholder("输入标题中的词").Grow(1).MinWidth(200).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface)
					compactSecondary(c, "want-custom", "只要这个词", func() { s.setWord(s.Keyword, wordWant); s.Keyword = "" }).TextColor(pal.accentInk).Disabled(strings.TrimSpace(s.Keyword) == "")
					compactSecondary(c, "avoid-custom", "不要这个词", func() { s.setWord(s.Keyword, wordAvoid); s.Keyword = "" }).TextColor(pal.muted).Disabled(strings.TrimSpace(s.Keyword) == "")
				})
				ui.Row(c).Wrap().Gap(12).Children(func() {
					compactQuiet(c, "toggle-aliases", "添加同义写法", func() { s.ShowAliases = !s.ShowAliases }).TextColor(pal.accentInk)
					if hasTitleConditions(s.Rule) {
						compactQuiet(c, "clear-rules", "重置全部条件", func() { s.applyRule(Rule{}, "重置全部条件"); s.WordOpen = false }).TextColor(pal.muted)
					}
				})
				if s.ShowAliases {
					caption(c, "用 / 分隔同一类标签的不同写法，例如：简日内嵌 / CHS。")
					ui.Row(c).Wrap().Gap(controlGap).AlignItems(ui.Center).Children(func() {
						ui.TextInput(c.Key("aliases"), &s.AnyDraft).Label("同义写法").Placeholder("简日内嵌 / CHS").Grow(1).MinWidth(200).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface)
						compactSecondary(c, "add-alias-group", "添加这组写法", s.addAliases).Disabled(strings.TrimSpace(s.AnyDraft) == "")
					})
				}
			})
		}
	})
}
