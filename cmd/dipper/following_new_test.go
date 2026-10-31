package main

import (
	"context"
	"testing"
	"time"

	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

func TestUIFollowedResourcesMarkNewReleases(t *testing.T) {
	state := uiFixture(t, "following")
	server, cleanup := followingSourceServer(t)
	defer cleanup()
	state.Ctx, state.Source = context.Background(), source.New(server.URL)
	sub := state.Subscriptions[0]
	sub.AnimeID, sub.GroupID, sub.GroupName = "227", "161", "字幕组161"
	state.Subscriptions[0] = sub
	updates := make(chan func(), 4)
	state.Update = func(update func()) { updates <- update }
	tester := ui.NewTester(state.view, 1320, 860)
	uiClick(t, tester, "查看资源 字幕组161")
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for !state.detailReady() {
		select {
		case update := <-updates:
			update()
		case <-deadline.C:
			t.Fatalf("detail did not become ready; detail=%q history=%q", state.DetailError, state.HistoryError)
		}
		tester.Frame()
	}
	first, second := state.previewResources()[0], state.previewResources()[1]
	sub.Recent = nil
	sub.NewTitles = map[string]bool{normalize(first.Title): true, normalize(second.Title): true}
	state.Subscriptions[0] = sub
	tester.Frame()

	uiRequireText(t, tester, "新")
	count := 0
	for _, text := range tester.Texts() {
		if text == "新" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("both pending rows should be marked new, got %d", count)
	}

	sub.NewTitles = nil
	state.Subscriptions[0] = sub
	tester.Frame()
	for _, text := range tester.Texts() {
		if text == "新" {
			t.Fatal("new marks survived an empty pending set")
		}
	}
}
