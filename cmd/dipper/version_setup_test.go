package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/source"
)

type versionTestRelease struct {
	title string
	size  int
}

func versionTestTorrent(index int) (string, string) {
	name := fmt.Sprintf("episode-%d.mkv", index)
	info := fmt.Sprintf("d6:lengthi1e4:name%d:%se", len(name), name)
	return fmt.Sprintf("%x", sha1.Sum([]byte(info))), "d4:info" + info + "e"
}

func versionTestTable(releases []versionTestRelease) string {
	var table strings.Builder
	table.WriteString(`<div class="episode-table"><table><tbody>`)
	for index, release := range releases {
		id, _ := versionTestTorrent(index)
		fmt.Fprintf(&table, `<tr><td><a class="magnet-link-wrap" href="/Home/Episode/%[1]s">%[2]s</a><button data-magnet="magnet:?xt=urn:btih:%[1]s"></button></td><td>%[3]d MB</td><td>2026-10-23 08:00:00</td><td><a href="/Download/20261023/%[1]s.torrent">下载</a></td></tr>`, id, html.EscapeString(release.title), release.size)
	}
	table.WriteString(`</tbody></table></div>`)
	return table.String()
}

func awaitVersionState(t *testing.T, updates chan func(), ready func() bool) {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for !ready() {
		select {
		case update := <-updates:
			update()
		case <-timeout.C:
			t.Fatal("version setup did not reach the requested state")
		}
	}
}

func TestVersionSetupEndToEnd(t *testing.T) {
	history := []versionTestRelease{
		{"[示例组] 示例番剧 [01][2160p][BDRip][CHT][HEVC][MKV]", 1200},
		{"[示例组] 示例番剧 [01][1080p][简体][AVC][AAC][MP4]", 400},
		{"[示例组] 示例番剧 [02][1080p][简体][AVC][AAC][MP4]", 420},
		{"[示例组] 示例番剧 [01][720p][简体][HEVC][MKV]", 170},
		{"[示例组] 示例番剧 [03][1080p][CHS][HEVC][10bit][MKV]", 600},
	}
	future := []versionTestRelease{
		{"[示例组] 示例番剧 [04][1920x1080][CHS][H.264][AAC][MP4]", 410},
		{"[示例组] 示例番剧 [04][1080p][繁体][AVC][AAC][MP4]", 410},
		{"[示例组] 示例番剧 [07][1080p][GB][HEVC][10bit][MKV]", 410},
		{"[示例组] 示例番剧 [07][1080p][CHS][HEVC][10bit][MKV][样片]", 410},
		{"[示例组] 示例番剧 [07][1080p][CHS][HEVC][10bit][MKV]", 410},
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/Home/Bangumi/1":
			io.WriteString(w, `<div class="bangumi-title">示例番剧<a class="mikan-rss" href="/RSS/Bangumi?bangumiId=1"></a></div><div class="bangumi-poster" style="background-image:url('/cover.png')"></div><div class="leftbar-nav"></div><div class="central-container">`)
			io.WriteString(w, `<div class="subgroup-text" id="2"><a href="/Home/PublishGroup/2">较少资源组</a></div>`+versionTestTable(history[3:4]))
			io.WriteString(w, `<div class="subgroup-text" id="3"><a href="/Home/PublishGroup/3">最多资源组</a></div>`+versionTestTable(history[:1]))
			io.WriteString(w, `<button class="js-expand-episode" data-bangumiid="1" data-subtitlegroupid="3"></button></div>`)
		case "/Home/ExpandEpisodeTable":
			if req.URL.Query().Get("subtitleGroupId") == "3" {
				io.WriteString(w, versionTestTable(history))
			} else {
				io.WriteString(w, versionTestTable(history[3:4]))
			}
		case "/RSS/Bangumi":
			if req.URL.Query().Get("subgroupid") != "3" {
				http.NotFound(w, req)
				return
			}
			io.WriteString(w, `<rss version="2.0"><channel><title>示例番剧</title><link>/RSS/Bangumi?bangumiId=1&amp;subgroupid=3</link>`)
			for index, release := range future {
				id, _ := versionTestTorrent(len(history) + index)
				fmt.Fprintf(w, `<item><title>%[2]s</title><link>/Home/Episode/%[1]s</link><pubDate>2026-10-24 08:00:00</pubDate><enclosure url="/Download/20261024/%[1]s.torrent" length="1" type="application/x-bittorrent" /></item>`, id, html.EscapeString(release.title))
			}
			io.WriteString(w, `</channel></rss>`)
		default:
			for index := range len(history) + len(future) {
				id, torrent := versionTestTorrent(index)
				if strings.HasSuffix(req.URL.Path, "/"+id+".torrent") {
					io.WriteString(w, torrent)
					return
				}
			}
			http.NotFound(w, req)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("truncated-preview-allows-switching-group-and-blocks-submit", func(t *testing.T) {
		releaseHistory := make(chan struct{})
		slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/Home/ExpandEpisodeTable" && req.URL.Query().Get("subtitleGroupId") == "3" {
				select {
				case <-releaseHistory:
				case <-req.Context().Done():
					return
				}
			}
			handler.ServeHTTP(w, req)
		}))
		defer slow.Close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s := uiFixture(t, "detail")
		s.Source, s.Ctx = source.New(slow.URL), ctx
		updates := make(chan func(), 16)
		s.Update = func(update func()) { updates <- update }
		s.openAnime("1", "browse")
		awaitVersionState(t, updates, func() bool { return !s.DetailLoading })
		if !s.HistoryPreviewOnly || !s.HistoryLoading || s.canSaveDetailRule() {
			t.Fatalf("truncated group should show preview while blocking submit: preview=%v loading=%v canSave=%v", s.HistoryPreviewOnly, s.HistoryLoading, s.canSaveDetailRule())
		}
		tt := ui.NewTester(s.view, 1320, 1000)
		uiRequireText(t, tt, "2160p · 繁体中文", "正在加载完整剧集…")
		uiClick(t, tt, "更换")
		uiClick(t, tt, "选择字幕组 较少资源组")
		close(releaseHistory)
		if !s.detailReady() || s.GroupIDs[s.Group] != "2" || len(s.History) != 1 {
			t.Fatalf("switching to complete non-truncated group should be instant: ready=%v group=%d rows=%d", s.detailReady(), s.Group, len(s.History))
		}
	})

	for _, mode := range []string{"recommended", "original-title"} {
		t.Run(mode, func(t *testing.T) {
			s := uiFixture(t, "detail")
			engine := &submissionTestEngine{}
			client := source.New(server.URL)
			root := t.TempDir()
			database := filepath.Join(root, "state.db")
			opts := core.Options{DownloadRoot: root, Downloader: engine, DisableScheduler: true,
				Feed: func(ctx context.Context, animeID, groupID string) ([]core.Resource, error) {
					rows, err := client.Feed(ctx, animeID, groupID)
					return coreResources(rows), err
				},
			}
			open := func() *core.Core {
				t.Helper()
				kernel, err := core.Open(database, opts)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { kernel.Close() })
				return kernel
			}
			s.Core, s.Engine, s.Source, s.Ctx = open(), engine, client, context.Background()
			updates := make(chan func(), 16)
			s.Update = func(update func()) { updates <- update }
			s.openAnime("1", "browse")
			awaitVersionState(t, updates, func() bool { return !s.DetailLoading && !s.HistoryLoading })
			if !s.detailReady() || s.GroupIDs[s.Group] != "3" || !slices.Equal(s.GroupCounts, []int{1, 5}) {
				t.Fatalf("did not select group 3 and load full history: group=%d counts=%v detail=%q history=%q", s.Group, s.GroupCounts, s.DetailError, s.HistoryError)
			}
			if s.Rule.Version == nil || s.Rule.Version.Height != 2160 {
				t.Fatalf("quality recommendation = %+v", s.Rule.Version)
			}
			width := 1320
			if mode == "original-title" {
				width = 960
			}
			tt := ui.NewTester(s.view, width, 1000)
			var expected []string
			if mode == "recommended" {
				uiClick(t, tt, "简体")
				if s.Rule.Version == nil || s.Rule.Version.Height != 1080 || s.Rule.Version.Language != core.SubtitleSimplified {
					t.Fatalf("language did not constrain quality: %+v", s.Rule.Version)
				}
				uiClick(t, tt, "省空间")
				if s.Rule.Version == nil || s.Rule.Version.Height != 720 {
					t.Fatalf("size did not use known resource bytes: %+v", s.Rule.Version)
				}
				uiClick(t, tt, "兼容性")
				if s.Rule.Version == nil || s.Rule.Version.Container != "MP4" {
					t.Fatalf("compatibility did not prefer MP4/AVC: %+v", s.Rule.Version)
				}
				uiClick(t, tt, "720p · 简体中文 · HEVC · MKV · 共 1 集 · 约 178.26 MB/集")
				if s.Rule.Version == nil || s.Rule.Version.Height != 720 || !s.ManualVersion {
					t.Fatalf("manual version choice did not override recommendation: %+v manual=%v", s.Rule.Version, s.ManualVersion)
				}
				uiClick(t, tt, "兼容性")
				if s.ManualVersion {
					t.Fatal("clicking a priority did not clear manual override")
				}
				first, _ := versionTestTorrent(1)
				second, _ := versionTestTorrent(2)
				uiClick(t, tt, "自选集数 / 查看明细 (2) ▾")
				uiClick(t, tt, "选择 "+first)
				uiClick(t, tt, "下载并追番")
				expected = append(expected, second)
			} else {
				uiClick(t, tt, "都不是想要的？按原标题自选")
				if s.canSaveDetailRule() {
					t.Fatal("empty title rules can save a new subscription")
				}
				uiClick(t, tt, "没有想要的？手动输入关键词 ›")
				uiScrollTo(t, tt, "查看标题词「CHS」")
				uiClick(t, tt, "查看标题词「CHS」")
				uiRequireText(t, tt, "CHS · 简体中文字幕")
				uiScrollTo(t, tt, "包含「CHS」 · 剩 1 条")
				uiClick(t, tt, "包含「CHS」 · 剩 1 条")
				uiScrollTo(t, tt, "标题关键词")
				uiClick(t, tt, "标题关键词")
				tt.Type("样片")
				uiClick(t, tt, "不要这个词")
				uiClick(t, tt, "标题关键词")
				tt.Type("07")
				uiClick(t, tt, "只要这个词")
				if s.countRuleMatches(s.Rule) != 0 || !s.canSaveDetailRule() {
					t.Fatal("zero historical matches should permit a future-only subscription")
				}
				uiClick(t, tt, "开启自动追番")
			}
			finishSubmission(t, s, updates)
			snapshot := submissionSnapshot(t, s)
			if len(snapshot.Subscriptions) != 1 || len(snapshot.Subscriptions[0].Items) != len(history) || s.SaveError != "" {
				t.Fatalf("subscription did not save history: %+v error=%q", snapshot.Subscriptions, s.SaveError)
			}
			if got := engine.submittedIDs(); !slices.Equal(got, expected) {
				t.Fatalf("history delivery = %v, want %v", got, expected)
			}
			if err := s.Core.Close(); err != nil {
				t.Fatal(err)
			}
			s.Core = open()
			sub := submissionSnapshot(t, s).Subscriptions[0]
			s.project(submissionSnapshot(t, s), nil)
			s.editSubscription(sub.ID)
			awaitVersionState(t, updates, func() bool { return !s.DetailLoading && !s.HistoryLoading })
			if !s.detailReady() || s.GroupIDs[s.Group] != sub.Group.ID || s.TitleSetup != (mode == "original-title") {
				t.Fatalf("reopened subscription has group=%d title setup=%v", s.Group, s.TitleSetup)
			}
			if mode == "recommended" {
				id, _ := versionTestTorrent(len(history))
				expected = append(expected, id)
			} else {
				id, _ := versionTestTorrent(len(history) + 4)
				expected = append(expected, id)
			}
			for range 2 {
				if err := s.Core.Check(s.Ctx, sub.ID); err != nil {
					t.Fatal(err)
				}
			}
			if got := engine.submittedIDs(); !slices.Equal(got, expected) {
				t.Fatalf("history plus RSS delivery after reopening = %v, want %v", got, expected)
			}
			tt = ui.NewTester(s.view, width, 1000)
			uiClick(t, tt, "下载已有资源")
			uiRequireText(t, tt, "下载选中资源", "全选匹配资源")
			if s.VersionSetup || s.TitleSetup || !s.Backfill {
				t.Fatal("backfill kept the subscription setup controls")
			}
		})
	}
}

func TestBrowseDetailCacheAvoidsDuplicateDetailFetch(t *testing.T) {
	releases := []versionTestRelease{
		{"[风车] 示例番剧 [01][1080p][简体][MP4]", 350},
		{"[风车] 示例番剧 [02][1080p][简体][MP4]", 360},
	}
	detailCalls, historyCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/Home/Bangumi/99":
			detailCalls++
			io.WriteString(w, `<div class="bangumi-title">示例番剧<a class="mikan-rss" href="/RSS/Bangumi?bangumiId=99"></a></div><div class="bangumi-poster" style="background-image:url('/cover.png')"></div><div class="leftbar-nav"></div><div class="central-container"><div class="header2-desc">一段简短介绍。</div><div class="subgroup-text" id="10"><a href="/Home/PublishGroup/10">风车</a></div>`+versionTestTable(releases)+`</div>`)
		case "/Home/ExpandEpisodeTable":
			historyCalls++
			http.NotFound(w, req)
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	s := uiFixture(t, "browse")
	s.Subscriptions = nil
	s.Source, s.Ctx = source.New(server.URL), context.Background()
	updates := make(chan func(), 8)
	s.Update = func(update func()) { updates <- update }

	anime := Anime{ID: "99", Title: "示例番剧"}
	s.openBrowseDetail(anime)
	awaitVersionState(t, updates, func() bool { return s.browseDetail != nil && !s.browseDetail.loading })
	if detailCalls != 1 {
		t.Fatalf("detail calls after synopsis = %d, want 1", detailCalls)
	}

	s.openFollowedResources("99", browseDetailReturn)
	if detailCalls != 1 || historyCalls != 0 {
		t.Fatalf("entering watch version reused cache poorly: detailCalls=%d historyCalls=%d", detailCalls, historyCalls)
	}
	if !s.detailReady() || s.Group != 0 || len(s.History) != 2 || s.Rule.Version == nil {
		t.Fatalf("cached detail did not immediately populate watch version: ready=%v group=%d history=%d version=%+v", s.detailReady(), s.Group, len(s.History), s.Rule.Version)
	}
}

func TestVersionScoringMainstreamCoverageAndGradedCompatibility(t *testing.T) {
	t.Run("mainstream-coverage-beats-single-4k-special", func(t *testing.T) {
		s := uiFixture(t, "detail")
		s.VersionSetup = true
		var rows []source.Resource
		for ep := 1; ep <= 8; ep++ {
			rows = append(rows, source.Resource{
				ID:    fmt.Sprintf("hd-%d", ep),
				Title: fmt.Sprintf("[风车] 柯南 [%02d][1080P][简体][MP4]", ep),
				Size:  400 * 1024 * 1024,
			})
		}
		rows = append(rows, source.Resource{
			ID:    "uhd-special",
			Title: "[风车] 柯南 [01][2160P][简体][MP4]",
			Size:  1200 * 1024 * 1024,
		})
		s.setHistory(rows)
		if s.Rule.Version == nil || s.Rule.Version.Height != 1080 {
			t.Fatalf("quality recommendation = %+v, want mainstream 1080p over 1-episode 2160p special", s.Rule.Version)
		}
	})

	t.Run("compatibility-scores-untagged-mp4-and-never-blanks-hevc-mkv", func(t *testing.T) {
		s := uiFixture(t, "detail")
		s.VersionSetup = true
		s.VersionPriority = preferCompatibility
		// Neither row explicitly writes "AVC", yet MP4 is the compatible choice.
		s.setHistory([]source.Resource{
			{ID: "mkv-1", Title: "[字幕组] 番剧 [01][1080P][简体][HEVC][MKV]", Size: 300 * 1024 * 1024},
			{ID: "mp4-1", Title: "[字幕组] 番剧 [01][1080P][简体][MP4]", Size: 420 * 1024 * 1024},
		})
		if s.Rule.Version == nil || s.Rule.Version.Container != "MP4" {
			t.Fatalf("compatibility failed to recommend MP4 without explicit AVC: %+v", s.Rule.Version)
		}

		// A group with only HEVC+MKV must still recommend a version and explain honestly.
		s.setHistory([]source.Resource{
			{ID: "hevc-1", Title: "[字幕组] 番剧 [01][1080P][简体][HEVC][MKV]", Size: 300 * 1024 * 1024},
		})
		if s.Rule.Version == nil || s.Rule.Version.Video != "HEVC" {
			t.Fatalf("compatibility cleared selection on HEVC/MKV-only group: %+v", s.Rule.Version)
		}
		if !strings.Contains(s.versionRecommendationNote(s.Versions[0]), "未提供更通用的 MP4") {
			t.Fatalf("unexpected compatibility note: %q", s.versionRecommendationNote(s.Versions[0]))
		}
	})

	t.Run("size-priority-falls-back-when-sizes-unknown", func(t *testing.T) {
		s := uiFixture(t, "detail")
		s.VersionSetup = true
		s.VersionPriority = preferSize
		s.setHistory([]source.Resource{
			{ID: "a", Title: "[字幕组] 番剧 [01][1080P][简体][AVC][MP4]"},
			{ID: "b", Title: "[字幕组] 番剧 [01][720P][简体][HEVC][MKV]"},
		})
		if s.Rule.Version == nil || s.Rule.Version.Height != 720 {
			t.Fatalf("size fallback with unknown bytes = %+v, want 720p", s.Rule.Version)
		}
	})
}

func TestVersionPreferencesPersistAcrossOpens(t *testing.T) {
	dir := t.TempDir()
	s := newState(dir)
	s.Source = source.New(source.DefaultOrigin)
	s.setVersionLanguage(core.SubtitleSimplified)
	s.setVersionPriority(preferCompatibility)

	reopened := newState(dir)
	reopened.Source = source.New(source.DefaultOrigin)
	if err := reopened.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	reopened.openAnime("42", "browse")
	if reopened.VersionLanguage != core.SubtitleSimplified || reopened.VersionPriority != preferCompatibility {
		t.Fatalf("restored preferences = (%v, %v), want (simplified, compatibility)", reopened.VersionLanguage, reopened.VersionPriority)
	}
}

func TestPickByExampleTitleSetupExtractsDistinguishingKeywordAndDownloads(t *testing.T) {
	state, engine, updates, _ := submissionFixture(t, false)
	state.VersionSetup = true
	rows := []source.Resource{
		{ID: "cn-1", Title: "[双语组] 星海旅记 - 01 [1080p][国语][简日内嵌][MP4]", Published: "2026/10/01 20:00", Size: 400e6},
		{ID: "jp-1", Title: "[双语组] 星海旅记 - 01 [1080p][日语][简日内嵌][MP4]", Published: "2026/10/01 20:00", Size: 400e6},
		{ID: "cn-2", Title: "[双语组] 星海旅记 - 02 [1080p][国语][简日内嵌][MP4]", Published: "2026/10/08 20:00", Size: 410e6},
		{ID: "jp-2", Title: "[双语组] 星海旅记 - 02 [1080p][日语][简日内嵌][MP4]", Published: "2026/10/08 20:00", Size: 410e6},
	}
	for index := range rows {
		rows[index].Magnet = fmt.Sprintf("magnet:?xt=urn:btih:%040x", index+1)
	}
	state.setHistory(rows)

	tester := ui.NewTester(state.view, 1320, 1000)
	// Same-signature collision between 国语 and 日语 triggers proactive guidance.
	uiRequireText(t, tester, "这个版本下有 2 集存在多条不同发布，可能包含未识别的版本差异。")
	uiClick(t, tester, "按原标题精确区分")
	uiRequireText(t, tester, "第 1 步：点一条你想要的资源")

	// Pick the Mandarin sample; system extracts "国语" automatically and matches both cn episodes.
	uiClick(t, tester, "选择模板 "+rows[2].Title)
	if !slices.Contains(state.Rule.All, "国语") {
		t.Fatalf("extracted keywords = %v, want to include 国语", state.Rule.All)
	}
	if got := state.planSubmission().IDs; !slices.Equal(got, []string{"cn-1", "cn-2"}) {
		t.Fatalf("planned downloads after picking example = %v, want [cn-1 cn-2]", got)
	}
	uiClick(t, tester, "下载并追番")
	finishSubmission(t, state, updates)
	if got := engine.submittedIDs(); !slices.Equal(got, []string{"cn-1", "cn-2"}) {
		t.Fatalf("submitted downloads from pick-by-example = %v, want [cn-1 cn-2]", got)
	}
}
