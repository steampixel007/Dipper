package main

import (
	"fmt"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/source"
)

func TestVersionSetupOneReleasePerEpisode(t *testing.T) {
	state, engine, updates, _ := submissionFixture(t, false)
	state.VersionSetup = true
	state.Checked = map[string]bool{}
	const original = "[桜都字幕组] 学生会也有洞！ / 生徒会にも穴はある！ [01][1080P][简繁内封]"
	const revision = "[桜都字幕组] 学生会也有洞！ / Seitokai ni mo Ana wa Aru! [01v2][1080P][简繁内封]"
	rows := []source.Resource{
		{ID: "original", Title: original, Published: "2026/10/07 23:28"},
		{ID: "revision", Title: revision, Published: "2026/10/10 04:49"},
		{ID: "second-older", Title: "[桜都字幕组] 学生会也有洞！ [02][1080P][简繁内封]", Published: "2026/10/13 04:49"},
		{ID: "second-newer", Title: "[桜都字幕组] 学生会也有洞！ [02][1080P][简繁内封]", Published: "2026/10/15 04:49"},
		{ID: "unknown-one", Title: "[桜都字幕组] 学生会也有洞！ [预告][1080P][简繁内封]"},
		{ID: "unknown-two", Title: "[桜都字幕组] 学生会也有洞！ [访谈][1080P][简繁内封]"},
	}
	for index := range rows {
		rows[index].Size = 170 * 1024 * 1024
		rows[index].Magnet = fmt.Sprintf("magnet:?xt=urn:btih:%040x", index+1)
	}
	state.setHistory(rows)
	defaults := []string{"revision", "second-newer", "unknown-one", "unknown-two"}
	requireSelection := func(want ...string) {
		t.Helper()
		if got := state.planSubmission().IDs; !slices.Equal(got, want) {
			t.Fatalf("downloads selected = %v, want %v", got, want)
		}
	}
	requireSelection(defaults...)

	tester := ui.NewTester(state.view, 1320, 1000)
	if tester.HasText(original) {
		t.Fatal("unselected alternative is visible before opening the episode")
	}
	uiRequireText(t, tester, revision)
	uiClick(t, tester, "全选符合条件的集数")
	requireSelection()
	uiClick(t, tester, "全选符合条件的集数")
	requireSelection(defaults...)
	state.OnlyMatches = false
	tester.Frame()
	if tester.HasText(original) {
		t.Fatal("an alternative appeared after showing non-matching rows")
	}
	uiRequireText(t, tester, revision)
	state.OnlyMatches = true
	tester.Frame()
	uiClick(t, tester, "本集匹配 2 个版本 · 换一个")
	uiRequireText(t, tester, original, revision)
	uiClick(t, tester, "选择 original")
	chosen := []string{"original", "second-newer", "unknown-one", "unknown-two"}
	requireSelection(chosen...)
	if tester.HasText(revision) {
		t.Fatal("choosing an alternative did not return to the compact episode list")
	}
	uiRequireText(t, tester, original, "本次下载 4 个资源")
	uiClick(t, tester, "下载并追番")
	finishSubmission(t, state, updates)
	if got := engine.submittedIDs(); !slices.Equal(got, chosen) {
		t.Fatalf("submitted downloads = %v, want %v", got, chosen)
	}
}

func TestEpisodeRevisionTakesPrecedenceOverPublishOrder(t *testing.T) {
	original := Resource{Episode: "1", Title: "[字幕组] 番剧 [01][1080P]", Published: "2026/10/15 16:00"}
	revision := Resource{Episode: "1", Title: "[字幕组] 番剧 [01V2][1080P]", Published: "2026/10/10 16:00"}
	if !preferredEpisodeRelease(revision, original) || preferredEpisodeRelease(original, revision) {
		t.Fatal("an explicit revision should win even if the unversioned row has a later timestamp")
	}

	original = Resource{Episode: "7", Title: "[字幕组] 番剧 S01E07 [1080P]", Published: "2026/10/15 16:00"}
	revision = Resource{Episode: "7", Title: "[字幕组] 番剧 S01E07v2 [1080P]", Published: "2026/10/10 16:00"}
	if !preferredEpisodeRelease(revision, original) || preferredEpisodeRelease(original, revision) {
		t.Fatal("a season episode revision should win over publish time")
	}
}
