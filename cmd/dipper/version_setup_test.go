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
			io.WriteString(w, `<div class="subgroup-text" id="2"><a href="/Home/PublishGroup/2">较少资源组</a></div>`+versionTestTable(history[:2]))
			io.WriteString(w, `<div class="subgroup-text" id="3"><a href="/Home/PublishGroup/3">最多资源组</a></div>`+versionTestTable(history[:1]))
			io.WriteString(w, `<button class="js-expand-episode" data-bangumiid="1" data-subtitlegroupid="3"></button></div>`)
		case "/Home/ExpandEpisodeTable":
			if req.URL.Query().Get("subtitleGroupId") == "3" {
				io.WriteString(w, versionTestTable(history))
			} else {
				io.WriteString(w, versionTestTable(history[:2]))
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
	t.Run("manual-group-while-counting", func(t *testing.T) {
		releaseCount := make(chan struct{})
		slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/Home/ExpandEpisodeTable" && req.URL.Query().Get("subtitleGroupId") == "3" {
				select {
				case <-releaseCount:
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
		awaitVersionState(t, updates, func() bool { return s.GroupCountLoading })
		tt := ui.NewTester(s.view, 1320, 1000)
		uiClick(t, tt, "选择字幕组 较少资源组")
		close(releaseCount)
		awaitVersionState(t, updates, func() bool { return !s.GroupCountLoading && !s.HistoryLoading })
		if !s.detailReady() || s.GroupIDs[s.Group] != "2" || len(s.History) != 2 || s.GroupCounts[1] != 5 {
			t.Fatalf("late counts changed the chosen group or history: group=%d rows=%d counts=%v", s.Group, len(s.History), s.GroupCounts)
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
			awaitVersionState(t, updates, func() bool { return !s.DetailLoading && !s.GroupCountLoading && !s.HistoryLoading })
			if !s.detailReady() || s.GroupIDs[s.Group] != "3" || !slices.Equal(s.GroupCounts, []int{2, 5}) {
				t.Fatalf("did not choose largest group from complete history: group=%d counts=%v detail=%q history=%q", s.Group, s.GroupCounts, s.DetailError, s.HistoryError)
			}
			if s.Rule.Version == nil || s.Rule.Version.Height != 2160 {
				t.Fatalf("quality recommendation = %+v", s.Rule.Version)
			}
			width := 1320
			if mode == "original-title" {
				width = 900
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
				uiClick(t, tt, "720p · 简体中文 · HEVC · MKV · 1 条")
				if s.Rule.Version == nil || s.Rule.Version.Height != 720 {
					t.Fatalf("manual version choice did not override recommendation: %+v", s.Rule.Version)
				}
				uiClick(t, tt, "兼容性")
				first, _ := versionTestTorrent(1)
				second, _ := versionTestTorrent(2)
				uiClick(t, tt, "选择 "+first)
				uiClick(t, tt, "下载并追番")
				expected = append(expected, second)
			} else {
				uiClick(t, tt, "按原标题追番")
				if s.canSaveDetailRule() {
					t.Fatal("empty title rules can save a new subscription")
				}
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
