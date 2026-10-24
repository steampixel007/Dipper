package main

import (
	"context"
	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
	"dipper.local/dipper/internal/source"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

type Anime struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Day    int    `json:"day"`
	Cover  string `json:"cover"`
	Source string `json:"source"`
}
type Rule struct {
	All  []string   `json:"all,omitempty"`
	None []string   `json:"none,omitempty"`
	Any  [][]string `json:"any,omitempty"`
}
type FilterEdit struct {
	Rule   Rule
	Action string
}
type Resource struct {
	ID, Title, Episode, Published string
	MB                            int
	Tokens                        []string
}
type Subscription struct {
	ID, GroupID, GroupName string
	AnimeID                string
	Group                  int
	Rule                   Rule
	Folder                 string
	Paused                 bool
	Skipped                []string
	Read                   []string
	LastCheck              string
	CheckFailed            bool
}
type Download struct {
	Error                               string
	DownSpeed, UpSpeed                  float64
	ID, AnimeID, Title, Episode, Folder string
	MB                                  int
	Progress                            float64
	Paused, Failed, SeedStopped         bool
	SeedSeconds                         int
	UploadedMB                          float64
}
type Saved struct {
	Root          string
	Upload        string
	SourceRoute   string
	Subscriptions []Subscription
	Downloads     []Download
	IntroHidden   bool
	ReduceEffects bool
}
type State struct {
	Core                                               *core.Core
	Engine                                             *downloader.Engine
	Source                                             *source.Client
	Ctx                                                context.Context
	Update                                             func(func())
	Groups, GroupIDs                                   []string
	GroupCounts                                        []int
	History                                            []source.Resource
	ResourceList                                       []Resource
	KnownAnime                                         map[string]Anime
	SourceLoading, DetailLoading, HistoryLoading, Busy bool
	SourceError, DetailError, HistoryError             string
	EditID                                             string
	ProposedFolder                                     string
	Backfill                                           bool
	catalogRevision, detailRevision, historyRevision   uint64
	catalogCancel, detailCancel, historyCancel         context.CancelFunc

	Saved
	Catalog                    []Anime
	Page                       string
	Year                       string
	Season                     string
	Day                        int
	Search                     string
	AnimeID                    string
	Group                      int
	ChoosingGroup              bool
	Rule                       Rule
	Checked                    map[string]bool
	Editing                    bool
	Keyword, AnyDraft          string
	WordOpen, ShowAliases      bool
	WordAnchor                 string
	UndoFilters, RedoFilters   []FilterEdit
	UndoSaved                  *Subscription
	FilterFeedback             string
	Confirm, Help, Suggestions bool
	SuggestedResource          Resource
	OnlyMatches                bool
	Notice, SaveError          string
	SourceTesting              bool
	SourceRouteError           string
	SourceRoutes               []sourceRoute
	SourceFailed               bool
	CatalogScroll              ui.ScrollState
	ResourceScroll             ui.ScrollState
	Dir                        string
	PickFolder                 func()
	OpenFolder                 func(string)
	Quit                       func()
}

var seasons = []string{"冬", "春", "夏", "秋"}

func (s *State) anime(id string) Anime {
	if a, ok := s.KnownAnime[id]; ok {
		return a
	}
	for _, a := range s.Catalog {
		if a.ID == id {
			return a
		}
	}
	return Anime{ID: id, Title: "未知番剧"}
}

func cloneRule(r Rule) Rule {
	b, _ := json.Marshal(r)
	var copy Rule
	_ = json.Unmarshal(b, &copy)
	return copy
}

func normalize(text string) string {
	return core.Normalize(text)
}
func decide(title string, r Rule) (bool, string) {
	result := core.Match(title, coreRule(r))
	return result.Matches, result.Reason
}

func (s *State) initSelection() {
	for _, r := range s.resources() {
		if _, seen := s.Checked[r.ID]; !seen {
			ok, _ := decide(r.Title, s.Rule)
			if ok {
				s.Checked[r.ID] = true
			}
		}
	}
}

type wordChoice uint8

const (
	wordUnrestricted wordChoice = iota
	wordWant
	wordAvoid
)

func (s *State) wordChoice(word string) wordChoice {
	for _, w := range s.Rule.None {
		if normalize(w) == normalize(word) {
			return wordAvoid
		}
	}
	for _, w := range s.Rule.All {
		if normalize(w) == normalize(word) {
			return wordWant
		}
	}
	return wordUnrestricted
}

func ruleWithWord(rule Rule, word string, choice wordChoice) Rule {
	rule = cloneRule(rule)
	word = strings.TrimSpace(word)
	if word == "" {
		return rule
	}
	remove := func(w string) bool { return normalize(w) == normalize(word) }
	rule.All = slices.DeleteFunc(rule.All, remove)
	rule.None = slices.DeleteFunc(rule.None, remove)
	switch choice {
	case wordWant:
		rule.All = append(rule.All, word)
	case wordAvoid:
		rule.None = append(rule.None, word)
	}
	return rule
}

func (s *State) countRuleMatches(rule Rule) int {
	n := 0
	for _, r := range s.resources() {
		if ok, _ := decide(r.Title, rule); ok {
			n++
		}
	}
	return n
}

func (s *State) applyRule(rule Rule, action string) {
	if reflect.DeepEqual(s.Rule, rule) {
		return
	}
	before := s.countRuleMatches(s.Rule)
	s.UndoFilters = append(s.UndoFilters, FilterEdit{Rule: cloneRule(s.Rule), Action: action})
	s.RedoFilters = nil
	s.Rule = rule
	s.initSelection()
	s.FilterFeedback = fmt.Sprintf("%s：%d → %d 条", action, before, s.countRuleMatches(rule))
}

func (s *State) setWord(word string, choice wordChoice) {
	word = strings.TrimSpace(word)
	if word == "" {
		return
	}
	action := "不再限制「" + word + "」"
	if choice == wordWant {
		action = "只要「" + word + "」"
	}
	if choice == wordAvoid {
		action = "不要「" + word + "」"
	}
	s.applyRule(ruleWithWord(s.Rule, word, choice), action)
	s.WordOpen = false
}

func (s *State) undoFilter() {
	if len(s.UndoFilters) > 0 {
		edit := s.UndoFilters[len(s.UndoFilters)-1]
		s.UndoFilters = s.UndoFilters[:len(s.UndoFilters)-1]
		s.RedoFilters = append(s.RedoFilters, FilterEdit{Rule: cloneRule(s.Rule), Action: edit.Action})
		s.Rule = cloneRule(edit.Rule)
		s.initSelection()
		s.FilterFeedback = fmt.Sprintf("已撤销%s · 匹配 %d 条", edit.Action, s.countRuleMatches(s.Rule))
		s.WordOpen = false
	}
}

func (s *State) redoFilter() {
	if len(s.RedoFilters) == 0 {
		return
	}
	edit := s.RedoFilters[len(s.RedoFilters)-1]
	s.RedoFilters = s.RedoFilters[:len(s.RedoFilters)-1]
	s.UndoFilters = append(s.UndoFilters, FilterEdit{Rule: cloneRule(s.Rule), Action: edit.Action})
	s.Rule = cloneRule(edit.Rule)
	s.initSelection()
	s.FilterFeedback = fmt.Sprintf("已重做%s · 匹配 %d 条", edit.Action, s.countRuleMatches(s.Rule))
	s.WordOpen = false
}

func (s *State) addAliases() {
	var words []string
	for _, w := range strings.Split(s.AnyDraft, "/") {
		w = strings.TrimSpace(w)
		if w != "" && !slices.ContainsFunc(words, func(existing string) bool { return normalize(existing) == normalize(w) }) {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		return
	}
	rule := cloneRule(s.Rule)
	rule.Any = append(rule.Any, words)
	s.applyRule(rule, "添加任意词组")
	s.AnyDraft = ""
}

func (s *State) quickWords() []string {
	var words []string
	for _, r := range s.resources() {
		for _, w := range r.Tokens {
			if !slices.Contains(words, w) {
				words = append(words, w)
			}
		}
	}
	return words
}
func (s *State) selected() ([]Resource, int) {
	var out []Resource
	total := 0
	for _, r := range s.resources() {
		ok, _ := decide(r.Title, s.Rule)
		if ok && s.Checked[r.ID] {
			out = append(out, r)
			total += r.MB
		}
	}
	return out, total
}
func (s *State) matchSummary() (int, int, int) {
	n, total := 0, 0
	eps := map[string]bool{}
	for _, r := range s.resources() {
		ok, _ := decide(r.Title, s.Rule)
		if ok {
			n++
			total += r.MB
			if r.Episode != "" {
				eps[r.Episode] = true
			}
		}
	}
	return n, len(eps), total
}
func (s *State) duplicates(ep string) int {
	if ep == "" {
		return 0
	}
	n := 0
	for _, r := range s.resources() {
		ok, _ := decide(r.Title, s.Rule)
		if ok && r.Episode == ep {
			n++
		}
	}
	return n
}
func sizeText(mb int) string {
	if mb >= 1000 {
		return fmt.Sprintf("%.2f GB", float64(mb)/1000)
	}
	return fmt.Sprintf("%d MB", mb)
}
func safeName(name string) string {
	return strings.TrimRight(strings.NewReplacer("/", "／", `\`, "＼", ":", "：", "*", "＊", "?", "？", `"`, "＂", "<", "＜", ">", "＞", "|", "｜").Replace(name), ". ")
}

func (s *State) changeSeason(delta int) {
	idx := slices.Index(seasons, s.Season)
	var year int
	_, _ = fmt.Sscan(s.Year, &year)
	idx += delta
	if idx < 0 {
		year--
		idx = 3
	}
	if idx > 3 {
		year++
		idx = 0
	}
	s.Year = fmt.Sprint(year)
	s.Season = seasons[idx]
	s.CatalogScroll = ui.ScrollState{}
	s.refreshCatalog()
}
