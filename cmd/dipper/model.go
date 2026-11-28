package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
	"github.com/steampixel007/Dipper/internal/source"
	"github.com/steampixel007/Dipper/internal/trackers"
	"github.com/steampixel007/Dipper/internal/update"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type Anime struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Day            int    `json:"day"`
	Cover          string `json:"cover"`
	Source         string `json:"source"`
	Summary        string `json:"summary,omitempty"`
	BroadcastStart string `json:"broadcastStart,omitempty"`
	OfficialURL    string `json:"officialURL,omitempty"`
	BangumiURL     string `json:"bangumiURL,omitempty"`
}
type Rule struct {
	All     []string             `json:"all,omitempty"`
	None    []string             `json:"none,omitempty"`
	Any     [][]string           `json:"any,omitempty"`
	Version *core.ReleaseVersion `json:"version,omitempty"`
}
type FilterEdit struct {
	Rule   Rule
	Action string
}
type Resource struct {
	ID, Title, Episode, Published string
	Bytes                         int64
	MB                            int
	Tokens                        []string
	Version                       *core.ReleaseVersion
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
	LastError              string
	NewCount               int
	Recent                 []string
	NewTitles              map[string]bool
	Active                 bool
	Queued                 bool
	Progress               float64
	// Completed counts this anime's downloads that finished without failing.
	Completed int
}
type Download struct {
	Error                               string
	DownSpeed, UpSpeed                  float64
	ID, AnimeID, Title, Episode, Folder string
	// ResourceID is the source resource the task was created from; titles
	// are not unique across releases.
	ResourceID                                            string
	Bytes, CompletedBytes, DownloadedBytes, UploadedBytes int64
	MB                                                    int
	Progress                                              float64
	Paused, Failed, Stopped, Queued                       bool
	SeedSeconds                                           int
	UploadedMB                                            float64
}

// DownloadFilter narrows the downloads page to one transfer state; the value
// is per-run, so the page keeps the choice only for the session.
type DownloadFilter int

const (
	downloadsAll DownloadFilter = iota
	downloadsDownloading
	downloadsSeeding
	downloadsPaused
	downloadsFailed
	downloadsStopped
	downloadsQueued
)

type Saved struct {
	Root                      string
	DownloadLimit             int
	downloadLimitDraft        string
	downloadLimitNotice       string
	downloadLimitError        bool
	Upload                    string
	Seed                      string
	SeedRatio                 string
	TransferLimits            transferLimits
	ExtraTrackers             string
	TrackerLists              []core.TrackerList
	DisabledTrackers          []string
	AutomaticTrackersDisabled bool
	ProxyMode                 int
	ProxyURL                  string
	ProxyAuto                 string
	ProxyFeedback             string
	ProxyTesting              bool
	proxyActiveMode           int
	proxyActiveAddress        string
	proxyUsername             string
	proxyPassword             string
	SourceRoute               string
	Subscriptions             []Subscription
	Downloads                 []Download
	IntroHidden               bool
	ReduceEffects             bool
	Appearance                string
	CloseBehavior             string
	SoftwareAuto              bool
	SoftwareAutoDownload      bool
	SoftwarePending           *softwarePending
	SoftwareCheckedAt         time.Time
	SavedVersionLanguage      core.SubtitleLanguage
	SavedVersionPriority      versionPriority
}
type downloadEngine interface {
	Snapshot() []downloader.Snapshot
	Pause(string) error
	Resume(string) error
	Stop(string) error
	PlayableFiles(string) ([]string, error)
	DownloadFiles(string) ([]downloader.DownloadedFile, error)
	Remove(string) error
	SetUploadLimit(int64) error
	SetDownloadLimit(int)
	SetSeedTarget(time.Duration)
	SetRatioTarget(float64)
	SetTrackerSettings(extra, disabled []string)
	SetRemoteTrackers([]string)
}

var _ downloadEngine = (*downloader.Engine)(nil)

type State struct {
	Core               *core.Core
	Engine             downloadEngine
	Source             *source.Client
	Ctx                context.Context
	Update             func(func())
	Groups, GroupIDs   []string
	GroupCounts        []int
	GroupPreviews      [][]source.Resource
	GroupCountLoading  bool
	History            []source.Resource
	HistoryPreviewOnly bool
	ResourceList       []Resource
	// resourceHashes bridges a viewed resource to the info hash the core
	// resolved for it, so resource rows match downloads by content identity.
	resourceHashes                                     map[string]string
	KnownAnime                                         map[string]Anime
	SourceLoading, DetailLoading, HistoryLoading, Busy bool
	SourceError, DetailError, HistoryError             string
	EditID                                             string
	LibraryID                                          string
	LibraryReturn                                      string
	LibraryMenuOpen                                    bool
	ProposedFolder                                     string
	Backfill                                           bool
	ViewResources                                      bool
	DetailReturn                                       string
	detailCache                                        map[string]cachedDetail
	catalogRevision, detailRevision, historyRevision   uint64
	catalogCancel, detailCancel, historyCancel         context.CancelFunc
	catalogFromDisk                                    bool
	catalogShown                                       catalogKey
	catalogDisk                                        catalogDisk

	Saved
	Catalog                         []Anime
	Page                            string
	Year                            string
	Season                          string
	SeasonDirection                 int
	Day                             int
	Search                          string
	FollowSearch                    string
	AnimeID                         string
	Group                           int
	ChoosingGroup                   bool
	GroupSearch                     string
	Rule                            Rule
	VersionSetup, TitleSetup        bool
	TitleRuleDraft                  Rule
	Versions                        []versionOption
	VersionLanguage                 core.SubtitleLanguage
	VersionPriority                 versionPriority
	ManualVersion                   bool
	UnclassifiedCount               int
	CollectionCount                 int
	ShowAllVersions                 bool
	ShowEpisodeDetails              bool
	ShowOriginalTitles              bool
	DownloadExistingChoice          bool
	TitleTemplateID                 string
	TitleManualInput                bool
	TitleShowAllSamples             bool
	ActiveTitleWord, ActiveTitleRow string
	Checked                         map[string]bool
	Editing                         bool
	Keyword, AnyDraft               string
	WordOpen, ShowAliases           bool
	WordAnchor                      string
	UndoFilters, RedoFilters        []FilterEdit
	UndoSaved                       *Subscription
	FilterFeedback                  string
	Help, Suggestions               bool
	CancelPrompt                    bool
	CancelID                        string
	RemovePrompt                    bool
	RemoveID                        string
	DownloadFilter                  DownloadFilter
	SuggestedResource               Resource
	OnlyMatches                     bool
	OnlyDownloaded                  bool
	OnlyNew                         bool
	EpisodeFilter                   string
	EpisodeFilterScroll             float32
	Notice, SaveError               string
	SourceTesting                   bool
	SourceRouteError                string
	SourceRoutes                    []sourceRoute
	SourceFailed                    bool
	trackerManagerOpen              bool
	trackerManager                  *trackerManagerDraft
	trackerAutomatic                trackers.List
	automaticTrackerRefreshRunning  bool
	trackerRefreshRunning           bool
	trackerFetcher                  trackers.Fetcher
	CatalogScroll                   ui.ScrollState
	browseDetail                    *browseDetail
	ResourceScroll                  ui.ScrollState
	FollowScroll                    ui.ScrollState
	DownloadScroll                  ui.ScrollState
	DownloadHistory                 bool
	DownloadHistoryScroll           ui.ScrollState
	DownloadFilterLast              DownloadFilter
	DownloadSearch                  string
	// CollapsedDownloads hides one anime's download rows on the downloads
	// page; the choice is per-session.
	CollapsedDownloads         map[string]bool
	Dir                        string
	ConfigDir                  string
	CoverDir                   string
	coverLoader                coverLoader
	ApplyAppearance            func(string)
	PickFolder                 func()
	OpenFolder                 func(string)
	OpenURL                    func(string)
	PlayFile                   func(string) error
	CopyText                   func(string)
	Quit                       func()
	Platform                   string
	ClosePrompt, RememberClose bool
	TrayAvailable              bool
	CheckTray                  func() bool
	HideWindow                 func()
	ShowWindow                 func()
	// CustomWindowControls is set when the view owns the window buttons because
	// the platform's title bar is hidden without its native controls.
	CustomWindowControls           bool
	WindowMinimize, WindowMaximize func()
	WindowClose                    func()
	WindowIsMaximized              func() bool
	FilesOpen                      bool
	FilesID                        string
	Files                          []downloader.DownloadedFile
	FilesError                     string
	FilesLoading                   bool
	filesRevision                  int
	SoftwareUpdate                 softwareUpdate
	SoftwareClient                 softwareClient
	SoftwareExecutable             func() (string, error)
	SoftwareInstallationOffer      func(context.Context, update.Offer, string, string) (update.Offer, error)
	OpenSoftwareFile               func(string) error
	PrepareSoftware                func(context.Context, string, update.Download, []string) (softwareInstaller, error)
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

func (s *State) subscription(id string) (Subscription, bool) {
	for _, sub := range s.Subscriptions {
		if sub.ID == id {
			return sub, true
		}
	}
	return Subscription{}, false
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
func (resource Resource) releaseVersion() core.ReleaseVersion {
	if resource.Version != nil {
		return *resource.Version
	}
	return core.ParseReleaseVersion(resource.Title)
}

func matchResource(resource Resource, rule Rule) (bool, string) {
	var version core.ReleaseVersion
	if rule.Version != nil {
		version = resource.releaseVersion()
	}
	result := core.MatchRelease(resource.Title, version, coreRule(rule))
	return result.Matches, result.Reason
}

func (s *State) initSelection() {
	if s.Backfill || (s.VersionSetup && s.TitleSetup && !hasTitleConditions(s.Rule)) {
		return
	}
	if s.focusedVersionSetup() && !s.DownloadExistingChoice {
		return
	}
	choices, _ := s.episodeChoices(s.matchingResources())
	for _, r := range choices {
		if _, seen := s.Checked[r.ID]; !seen {
			s.Checked[r.ID] = true
		}
	}
}

func (s *State) matchingResources() []Resource {
	var matches []Resource
	for _, resource := range s.resources() {
		if ok, _ := matchResource(resource, s.Rule); ok {
			matches = append(matches, resource)
		}
	}
	return matches
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
		if ok, _ := matchResource(r, rule); ok {
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
		s.FilterFeedback = fmt.Sprintf("已撤销%s，匹配 %d 条", edit.Action, s.countRuleMatches(s.Rule))
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
	s.FilterFeedback = fmt.Sprintf("已重做%s，匹配 %d 条", edit.Action, s.countRuleMatches(s.Rule))
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
	counts := map[string]int{}
	for _, r := range s.resources() {
		for _, w := range r.Tokens {
			counts[w]++
		}
	}
	words := make([]string, 0, len(counts))
	for w := range counts {
		words = append(words, w)
	}
	slices.SortStableFunc(words, func(a, b string) int {
		if counts[a] != counts[b] {
			return counts[b] - counts[a]
		}
		return strings.Compare(a, b)
	})
	return words
}
func (s *State) selected() ([]Resource, int) {
	plan := s.planSubmission()
	total := 0
	for _, resource := range plan.Selected {
		total += resource.MB
	}
	return plan.Selected, total
}
func (s *State) matchSummary() (int, int, int) {
	n, total := 0, 0
	eps := map[string]bool{}
	for _, r := range s.resources() {
		ok, _ := matchResource(r, s.Rule)
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

// toggleEpisodeFilter narrows the resource preview to one episode; clicking
// the active episode's badge clears the filter.
func (s *State) toggleEpisodeFilter(ep string) {
	if s.EpisodeFilter == ep {
		s.EpisodeFilter = ""
		s.ResourceScroll.Y = s.EpisodeFilterScroll
	} else {
		s.EpisodeFilter = ep
		s.EpisodeFilterScroll = s.ResourceScroll.Y
	}
}

// isNewRelease reports whether the title is one of the subscription's pending
// RSS releases, so the followed resource view can mark it as new.
func (s *State) isNewRelease(title string) bool {
	if s.EditID == "" {
		return false
	}
	sub, ok := s.subscription(s.EditID)
	if !ok {
		return false
	}
	return sub.NewTitles[normalize(title)]
}

// groupFolder returns the anime's download directory. A transfer's folder is
// its saved task directory under the anime directory, so the group header opens
// the parent and reaches every release even while the group is collapsed.
func (s *State) groupFolder(animeID string) string {
	for _, d := range s.Downloads {
		if d.AnimeID == animeID && d.Folder != "" {
			return filepath.Dir(d.Folder)
		}
	}
	return ""
}

func (s *State) duplicates(ep string) int {
	if ep == "" {
		return 0
	}
	n := 0
	for _, r := range s.resources() {
		ok, _ := matchResource(r, s.Rule)
		if ok && r.Episode == ep {
			n++
		}
	}
	return n
}

// speedText formats a transfer speed measured in decimal MB/s with binary
// byte units, matching the value the engine actually applies. A stopped or
// invalid transfer shows a dash instead of "0 KiB/s".
// downloadETA estimates the remaining download time from the current speed.
// An empty string means no meaningful estimate: the transfer is not actively
// downloading, or its size or speed is unknown.
func downloadETA(d Download) string {
	bytes := sizeBytes(d.Bytes, d.MB)
	if bytes <= 0 || math.IsNaN(d.Progress) || d.Progress >= 1 || d.DownSpeed <= 0 || d.Paused || d.Stopped || d.Failed || d.Queued {
		return ""
	}
	return etaText(float64(bytes-downloadDoneBytes(d)) / 1e6 / d.DownSpeed)
}

// etaText renders a remaining duration in seconds as a coarse human estimate.
func etaText(seconds float64) string {
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return ""
	}
	minutes := int(seconds / 60)
	switch {
	case minutes < 1:
		return "剩余不足 1 分钟"
	case minutes < 60:
		return fmt.Sprintf("剩余约 %d 分钟", minutes)
	case minutes%60 == 0:
		return fmt.Sprintf("剩余约 %d 小时", minutes/60)
	default:
		return fmt.Sprintf("剩余约 %d 小时 %d 分钟", minutes/60, minutes%60)
	}
}

// downloadGroupETA estimates a group's remaining download time from the
// combined measured speed of its active transfers; empty means nothing is
// actively downloading or the sizes and speeds are unknown.
func (s *State) downloadGroupETA(indices []int) string {
	remaining, speed := 0.0, 0.0
	for _, i := range indices {
		d := s.Downloads[i]
		if math.IsNaN(d.Progress) {
			return ""
		}
		if d.Paused || d.Stopped || d.Failed || d.Queued || d.Progress >= 1 {
			continue
		}
		bytes := sizeBytes(d.Bytes, d.MB)
		if bytes <= 0 {
			return ""
		}
		remaining += float64(bytes - downloadDoneBytes(d))
		speed += d.DownSpeed
	}
	if remaining <= 0 || speed <= 0 {
		return ""
	}
	return etaText(remaining / 1e6 / speed)
}

func sizeBytes(bytes int64, mb int) int64 {
	if bytes < 0 {
		return 0
	}
	if bytes > 0 {
		return bytes
	}
	if mb <= 0 || uint64(mb) > uint64(math.MaxInt64/1_000_000) {
		return 0
	}
	return int64(mb) * 1e6
}

func legacySizeMB(bytes int64) int {
	if bytes <= 0 {
		return 0
	}
	mb := bytes / 1e6
	if bytes%1e6 != 0 {
		mb++
	}
	if uint64(mb) > uint64(^uint(0)>>1) {
		return 0
	}
	return int(mb)
}

func byteSizeText(bytes int64) string {
	if bytes <= 0 {
		return "大小未知"
	}
	return exactByteText(bytes)
}

// exactByteText formats a known byte count. Zero is a real file size here, not
// an unknown value, so empty files must not be labelled "大小未知".
func exactByteText(bytes int64) string {
	if bytes < 0 {
		return "大小未知"
	}
	if bytes == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	value, unit := float64(bytes), 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	for unit < len(units)-1 && math.Round(value*100) >= 100_000 {
		value /= 1000
		unit++
	}
	text := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
	return text + " " + units[unit]
}

func downloadDoneBytes(d Download) int64 {
	bytes := sizeBytes(d.Bytes, d.MB)
	if bytes <= 0 {
		return 0
	}
	if d.CompletedBytes > 0 {
		return min(bytes, d.CompletedBytes)
	}
	if math.IsNaN(d.Progress) || d.Progress <= 0 {
		return 0
	}
	if d.Progress >= 1 || float64(bytes)*d.Progress >= float64(bytes) {
		return bytes
	}
	return int64(float64(bytes) * d.Progress)
}

func (s *State) downloadGroupSizes(indices []int) (total, done int64, unknown int, overflow bool) {
	for _, index := range indices {
		d := s.Downloads[index]
		bytes := sizeBytes(d.Bytes, d.MB)
		if bytes == 0 {
			unknown++
			continue
		}
		if total > math.MaxInt64-bytes {
			return 0, 0, unknown, true
		}
		total += bytes
		done += downloadDoneBytes(d)
	}
	return
}

func speedText(mbPerSecond float64, upload bool) string {
	arrow := "↓"
	if upload {
		arrow = "↑"
	}
	bytesPerSecond := mbPerSecond * 1e6
	switch {
	case bytesPerSecond <= 0 || math.IsNaN(bytesPerSecond) || math.IsInf(bytesPerSecond, 0):
		return arrow + " —"
	case bytesPerSecond < 1024:
		return arrow + " 不到 1 KiB/s"
	case bytesPerSecond < 1024*1024:
		return fmt.Sprintf("%s %.0f KiB/s", arrow, bytesPerSecond/1024)
	case bytesPerSecond < 1024*1024*1024:
		return fmt.Sprintf("%s %.1f MiB/s", arrow, bytesPerSecond/(1024*1024))
	}
	return fmt.Sprintf("%s %.1f GiB/s", arrow, bytesPerSecond/(1024*1024*1024))
}

func seedTimeText(seconds int) string {
	if seconds <= 0 {
		return "分享时间 0 分钟"
	}
	if seconds < 60 {
		return "分享时间不足 1 分钟"
	}
	return fmt.Sprintf("分享时间 %d 分钟", seconds/60)
}

func isPlayableMedia(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv", ".mp4", ".avi", ".ts", ".webm", ".mov":
		return true
	}
	return false
}
func safeName(name string) string {
	return strings.TrimRight(strings.NewReplacer("/", "／", `\`, "＼", ":", "：", "*", "＊", "?", "？", `"`, "＂", "<", "＜", ">", "＞", "|", "｜").Replace(name), ". ")
}

const earliestCatalogYear = 2013

func firstCatalogSeasonIndex(year int) int {
	if year == earliestCatalogYear {
		return len(seasons) - 1
	}
	return 0
}

func (s *State) changeCatalogYear() {
	year, _ := strconv.Atoi(s.Year)
	if first := firstCatalogSeasonIndex(year); slices.Index(seasons, s.Season) < first {
		s.Season = seasons[first]
	}
	s.refreshCatalog()
}

func (s *State) adjacentCatalogSeason(delta int) (year, seasonIndex int, available bool) {
	year, err := strconv.Atoi(s.Year)
	seasonIndex = slices.Index(seasons, s.Season)
	if err != nil || seasonIndex < 0 {
		return 0, 0, false
	}
	seasonIndex += delta
	if seasonIndex < 0 {
		year--
		seasonIndex = len(seasons) - 1
	}
	if seasonIndex >= len(seasons) {
		year++
		seasonIndex = 0
	}
	available = year >= earliestCatalogYear && year <= time.Now().Year() && seasonIndex >= firstCatalogSeasonIndex(year)
	return year, seasonIndex, available
}

func (s *State) canChangeSeason(delta int) bool {
	_, _, available := s.adjacentCatalogSeason(delta)
	return available
}

func (s *State) changeSeason(delta int) {
	year, seasonIndex, available := s.adjacentCatalogSeason(delta)
	if !available {
		return
	}
	s.SeasonDirection = delta
	s.Year = strconv.Itoa(year)
	s.Season = seasons[seasonIndex]
	s.refreshCatalog()
}
