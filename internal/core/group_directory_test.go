package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func groupTask(t *testing.T, kernel *Core, release Resource) DownloadTask {
	t.Helper()
	hash, err := ResourceHash(release)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := kernel.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range tasks {
		if record.Task.Hash == hash {
			return record.Task
		}
	}
	t.Fatalf("release %q has no submitted task", release.ID)
	return DownloadTask{}
}

func TestGroupDirectorySharesEpisodesAndKeepsHashIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	downloader := &fakeDownloader{}
	kernel := openTest(t, path, Options{Downloader: downloader})
	first := resource("first", 101, "Episode 01")
	second := resource("second", 102, "Episode 02")
	selectedGroup := Group{ID: "group-a", Name: "制作组 A"}
	sub := create(t, kernel, anime, selectedGroup, Rule{}, []Resource{first}, []string{first.ID})
	if _, err := kernel.DownloadExisting(context.Background(), sub.ID, []Resource{second}, []string{second.ID}); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(sub.Directory, selectedGroup.Name)
	for _, release := range []Resource{first, second} {
		task := groupTask(t, kernel, release)
		if task.Directory != want || !task.SharedDirectory {
			t.Fatalf("episode must use the readable shared group directory: %+v, want %q", task, want)
		}
	}
	alias := first
	alias.ID = "another-catalog-id"
	otherGroup := Group{ID: "group-b", Name: "制作组 B"}
	other := create(t, kernel, anime, otherGroup, Rule{}, []Resource{alias}, []string{alias.ID})
	if tasks, err := kernel.ListTasks(); err != nil || len(tasks) != 2 {
		t.Fatalf("same hash admitted another task: %+v, %v", tasks, err)
	}
	if downloader.count() != 2 || groupTask(t, kernel, alias).Directory != want {
		t.Fatal("same hash changed its destination or was submitted again")
	}
	third := resource("third", 103, "Episode 03")
	if _, err := kernel.DownloadExisting(context.Background(), other.ID, []Resource{third}, []string{third.ID}); err != nil {
		t.Fatal(err)
	}
	if got := groupTask(t, kernel, third).Directory; got != filepath.Join(sub.Directory, otherGroup.Name) {
		t.Fatalf("another group has destination %q", got)
	}
	if err := kernel.RemoveSubscription(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	if err := kernel.Close(); err != nil {
		t.Fatal(err)
	}
	restartedDownloader := &fakeDownloader{}
	kernel = openTest(t, path, Options{Downloader: restartedDownloader})
	if err := kernel.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return restartedDownloader.count() == 3 })
	settings, err := kernel.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.DownloadRoot = filepath.Join(filepath.Dir(path), "new-download-root")
	if err := kernel.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	fourth := resource("fourth", 104, "Episode 04")
	create(t, kernel, Anime{ID: anime.ID, Title: "Renamed anime"}, Group{ID: selectedGroup.ID, Name: "Renamed group"}, Rule{}, []Resource{fourth}, []string{fourth.ID})
	if got := groupTask(t, kernel, fourth).Directory; got != want {
		t.Fatalf("recreated group moved after restart, rename or root change: %q, want %q", got, want)
	}
	if got := groupTask(t, kernel, first).Directory; got != want {
		t.Fatalf("accepted episode changed destination on replay: %q", got)
	}
}

func TestGroupDirectoryNamesStayReadableAndDistinct(t *testing.T) {
	kernel := openTest(t, filepath.Join(t.TempDir(), "state.db"), Options{Downloader: &fakeDownloader{}})
	cases := []struct {
		id   string
		name string
		want string
	}{
		{"upper", "Team", "Team"},
		{"lower", "team", "team [2]"},
		{"slash", "A/B", "A_B"},
		{"question", "A?B", "A_B [2]"},
		{"empty", "", "字幕组"},
		{"dots", "...", "字幕组 [2]"},
		{"device", "CON", "_CON"},
		{"port", "COM0", "_COM0"},
	}
	for index, testCase := range cases {
		t.Run(testCase.id, func(t *testing.T) {
			release := resource(testCase.id, 201+index, "Episode 01")
			sub := create(t, kernel, anime, Group{ID: testCase.id, Name: testCase.name}, Rule{}, []Resource{release}, []string{release.ID})
			task := groupTask(t, kernel, release)
			want := filepath.Join(sub.Directory, testCase.want)
			if task.Directory != want || !task.SharedDirectory {
				t.Fatalf("readable group destination = %q (shared %v), want %q", task.Directory, task.SharedDirectory, want)
			}
		})
	}
	longGroup := strings.Repeat("组", 52) + "....end"
	release := resource("long", 250, "Episode 01")
	sub := create(t, kernel, anime, Group{ID: "long", Name: longGroup}, Rule{}, []Resource{release}, []string{release.ID})
	task := groupTask(t, kernel, release)
	if filepath.Dir(task.Directory) != sub.Directory || strings.TrimRight(filepath.Base(task.Directory), " .") != filepath.Base(task.Directory) {
		t.Fatalf("truncated group name is not a safe directory: %q", task.Directory)
	}
	anotherRelease := resource("another-anime", 251, "Episode 01")
	another := create(t, kernel, Anime{ID: "another-anime", Title: anime.Title}, Group{ID: "upper", Name: "Team"}, Rule{}, []Resource{anotherRelease}, []string{anotherRelease.ID})
	if another.Directory == sub.Directory || groupTask(t, kernel, anotherRelease).Directory == filepath.Join(sub.Directory, "Team") {
		t.Fatal("different anime with the same title share a group directory")
	}
}

func TestLegacyTaskDirectorySurvivesReplayAndIndependentReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	kernel := openTest(t, path, Options{Downloader: &fakeDownloader{}})
	release := resource("legacy", 301, "Episode 01")
	sub := create(t, kernel, anime, group, Rule{}, []Resource{release}, []string{release.ID})
	hash, err := ResourceHash(release)
	if err != nil {
		t.Fatal(err)
	}
	legacyDirectory := filepath.Join(sub.Directory, hash)
	if err := kernel.transaction(context.Background(), func(transaction *sql.Tx, state *diskState) error {
		record, err := taskByHash(context.Background(), transaction, hash)
		if err != nil {
			return err
		}
		record.Task.Directory = legacyDirectory
		record.Task.SharedDirectory = false
		animeRecord := state.Anime[anime.ID]
		animeRecord.GroupDirectories = nil
		state.Anime[anime.ID] = animeRecord
		return saveTask(context.Background(), transaction, record)
	}); err != nil {
		t.Fatal(err)
	}
	if err := kernel.Close(); err != nil {
		t.Fatal(err)
	}
	downloader := &fakeDownloader{}
	kernel = openTest(t, path, Options{Downloader: downloader})
	if err := kernel.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return downloader.count() == 1 })
	assertLegacy := func() {
		t.Helper()
		task := groupTask(t, kernel, release)
		if task.Directory != legacyDirectory || task.SharedDirectory {
			t.Fatalf("saved legacy destination was migrated: %+v", task)
		}
	}
	assertLegacy()
	collision := resource("hash-named-group", 302, "Episode 02")
	create(t, kernel, anime, Group{ID: "hash-named-group", Name: hash}, Rule{}, []Resource{collision}, []string{collision.ID})
	if got := groupTask(t, kernel, collision).Directory; got != legacyDirectory+" [2]" {
		t.Fatalf("group folder collided with saved legacy task: %q", got)
	}
	next := resource("new-episode", 303, "Episode 03")
	if _, err := kernel.DownloadExisting(context.Background(), sub.ID, []Resource{next}, []string{next.ID}); err != nil {
		t.Fatal(err)
	}
	if got := groupTask(t, kernel, next); got.Directory != filepath.Join(sub.Directory, group.Name) || !got.SharedDirectory {
		t.Fatalf("new episode inherited legacy hash layout: %+v", got)
	}
	if err := kernel.RemoveSubscription(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	if err := kernel.ResetTask(context.Background(), hash); err != nil {
		t.Fatal(err)
	}
	assertLegacy()
	if err := kernel.RetryPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertLegacy()
	downloader.mu.Lock()
	defer downloader.mu.Unlock()
	accepted, ok := downloader.accepted[hash]
	if !ok || accepted.Directory != legacyDirectory || accepted.SharedDirectory {
		t.Fatalf("independent reset did not resubmit to saved legacy folder: %+v", accepted)
	}
}
